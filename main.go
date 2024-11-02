package main

import (
	"log"
	"net/http"
	"os"
	"rest-go-demo/auth"
	"rest-go-demo/controllers"
	"rest-go-demo/database"
	"rest-go-demo/referrals"
	"rest-go-demo/twitter"

	"github.com/joho/godotenv"
	"github.com/rs/cors"

	_ "rest-go-demo/docs" // docs is generated by Swag CLI, you have to import it

	httpSwagger "github.com/swaggo/http-swagger"

	"github.com/didip/tollbooth/v7"

	"github.com/go-co-op/gocron"
	"github.com/gorilla/mux"
	_ "github.com/jinzhu/gorm/dialects/mysql" //Required for MySQL dialect

	"time"
)

// @title   WalletChat API
// @version 2.0
// @description.markdown
// @wallet_chat                API Support via Twitter
// @contact.url                https://walletchat.fun
// @contact.email              contact@walletchat.fun
// @securityDefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
// @host                       app.walletchat.fun
// @BasePath
func main() {
	godotenv.Load(".env")

	initDB()
	log.Println("Starting the HTTP server on port 8080")

	jwtProvider := auth.NewJwtHmacProvider(
		os.Getenv("JWT_HMAC_SECRET"),
		"https://walletchat.fun",
		time.Minute*60*24*30,
	)

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/register", auth.RegisterHandler()).Methods("POST")
	router.HandleFunc("/users/{address}/nonce", auth.UserNonceHandler()).Methods("GET")
	router.HandleFunc("/get_unread_cnt/{address}", controllers.GetUnreadMsgCntTotalExternal).Methods("GET") //For Android app
	router.HandleFunc("/verify_email/{email}/{code}", controllers.VerifyEmail).Methods("GET")
	router.HandleFunc("/signin", auth.SigninHandler(jwtProvider)).Methods("POST")
	router.HandleFunc("/resolve_name/{name}", controllers.ResolveName).Methods("GET")
	router.HandleFunc("/ethereum_token_overlap/{contract_address}", controllers.Erc20TokenOverlap).Methods("GET") //for custom GPT - not WC directly
	router.HandleFunc("/solana_token_overlap/{contract_address}", controllers.SolTokenOverlap).Methods("GET")     //for custom GPT - not WC directly
	router.PathPrefix("/docs").Handler(httpSwagger.WrapHandler)
	router.HandleFunc("/track_ga4/{event}/{email}/{addr}/{placeholder_photo}", controllers.TrackEventGA4).Methods("GET")
	router.HandleFunc("/get_leaderboard_data", referrals.GetLeaderboardData).Methods("GET")
	router.HandleFunc("/get_leaderboard_data/{address}", referrals.GetLeaderboardDataSingle).Methods("GET")
	router.HandleFunc("/get_referral_code/{address}", referrals.GetReferralCodeAddr).Methods("GET")
	//debugging
	router.HandleFunc("/debug_print", controllers.DebugPrint).Methods("POST")

	//bookmarks
	router.HandleFunc("/oura_register", controllers.RegisterOuraUser).Methods("POST")

	wsRouter := router.PathPrefix("/v1").Subrouter()

	wsRouter.Use(auth.AuthMiddleware(jwtProvider))
	wsRouter.HandleFunc("/welcome", auth.WelcomeHandler()).Methods("GET")

	initaliseHandlers(wsRouter)

	//schedule daily notifications
	//s := gocron.NewScheduler(time.UTC)
	// set time
	//s.Every(1).Day().At("10:30").Do(func() { sendPeriodicNotifications() })
	//s.Every(1).Day().At("01:00").Do(func() { referrals.CreateDailyReferralCodes() })
	// starts the scheduler asynchronously
	//s.StartAsync()

	//schedule telegram polling for new verified users (should be webhook someday)
	t := gocron.NewScheduler(time.UTC)
	// set time
	t.Every(10).Seconds().Do(func() { updateTelegramVerifiedUsers() })
	// starts the scheduler asynchronously
	t.StartAsync()

	//schedule twitter username polling for new verified users
	u := gocron.NewScheduler(time.UTC)
	// set time
	u.Every(61).Seconds().Do(func() { twitter.SearchVerifyUsernames() })
	// starts the scheduler asynchronously
	u.StartAsync()

	//schedule twitter polling for users requesting referral codes
	v := gocron.NewScheduler(time.UTC)
	// set time
	v.Every(62).Seconds().Do(func() { twitter.SearchForUserReferralCodeTweets() })
	// starts the scheduler asynchronously
	v.StartAsync()

	// w := gocron.NewScheduler(time.UTC)
	// // set time
	// w.Every(100000).Seconds().Do(func() { referrals.GetLeaderboardDataCronJob() })
	// // starts the scheduler asynchronously
	// w.StartAsync()

	controllers.InitGlobals()
	controllers.InitRandom()
	referrals.InitRandom()
	twitter.InitSearchParams()

	//handler := cors.Default().Handler(router)
	handler := cors.AllowAll().Handler(router) //Live API overrides this anyway
	// c := cors.New(cors.Options{
	// 	AllowedOrigins: []string{"http://localhost:3002", "http://localhost:8080", "https://api.nftport.xyz", "https://app.walletchat.fun",
	// 		"https://api.v2.walletchat.fun", "https://beta.walletchat.fun", "https://lit.walletchat.fun",
	// 		"https://ledger.walletchat.fun"},
	// 	AllowCredentials: true,
	// 	// Enable Debugging for testing, consider disabling in production
	// 	//Debug: true,
	// })
	//handler := c.Handler(router)

	//rate limit POST/PUT requests (We still use GET for polling, so we can't rate limit this yet)
	lmt := tollbooth.NewLimiter(float64(5), nil)
	lmt.SetIPLookups([]string{"RemoteAddr", "X-Forwarded-For", "X-Real-IP"}).SetMethods([]string{"POST"})

	log.Fatal(http.ListenAndServe(":8080", tollbooth.LimitHandler(lmt, handler)))
}

func sendPeriodicNotifications() {
	controllers.SendNotificationEmails()
}

func updateTelegramVerifiedUsers() {
	controllers.UpdateTelegramNotifications()
}

// var count int32 = 0

// func trackApiRequests(fn http.HandlerFunc) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		atomic.AddInt32(&count, 1)
// 		log.Println(count)
// 		fn(w, r)
// 	}
// }

// these endpoints are protected by JWTs
func initaliseHandlers(router *mux.Router) {
	router.HandleFunc("/apicount", auth.GetCountsAPI()).Methods("GET")
	router.HandleFunc("/resolve_name/{name}", controllers.ResolveName).Methods("GET")

	//1-to-1 chats (both general and NFT related)
	router.HandleFunc("/get_unread_cnt/{address}", controllers.GetUnreadMsgCntTotal).Methods("GET")

	router.HandleFunc("/get_unread_cnt_by_type/{address}/{type}", controllers.GetUnreadMsgCntTotalByType).Methods("GET")
	router.HandleFunc("/get_unread_cnt/{fromaddr}/{toaddr}", controllers.GetUnreadMsgCnt).Methods("GET")
	router.HandleFunc("/get_unread_cnt/{address}/{nftaddr}/{nftid}", controllers.GetUnreadMsgCntNft).Methods("GET")
	router.HandleFunc("/get_unread_cnt_nft/{address}", controllers.GetUnreadMsgCntNftAllByAddr).Methods("GET")
	router.HandleFunc("/getall_chatitems/{address}", controllers.GetChatFromAddress).Methods("GET")
	router.HandleFunc("/getall_chatitems/{fromaddr}/{toaddr}", controllers.GetAllChatFromAddressToAddr).Methods("GET")
	router.HandleFunc("/get_n_chatitems/{fromaddr}/{toaddr}/{count}", controllers.GetNChatFromAddressToAddr).Methods("GET")
	router.HandleFunc("/getread_chatitems/{fromaddr}/{toaddr}", controllers.GetReadChatFromAddressToAddr).Methods("GET")
	router.HandleFunc("/getall_chatitems/{fromaddr}/{toaddr}/{time}", controllers.GetNewChatFromAddressToAddr).Methods("GET")
	router.HandleFunc("/getnft_chatitems/{fromaddr}/{toaddr}/{nftaddr}/{nftid}", controllers.GetChatNftAllItemsFromAddrAndNFT).Methods("GET")
	router.HandleFunc("/getnft_chatitems/{address}/{nftaddr}/{nftid}", controllers.GetChatNftAllItemsFromAddr).Methods("GET")
	router.HandleFunc("/getnft_chatitems/{nftaddr}/{nftid}", controllers.GetChatNftContext).Methods("GET")
	router.HandleFunc("/getnft_chatitems/{address}", controllers.GetNftChatFromAddress).Methods("GET")
	router.HandleFunc("/update_chatitem/{fromaddr}/{toaddr}", controllers.UpdateChatitemByOwner).Methods("PUT")
	router.HandleFunc("/deleteall_chatitems/{address}", controllers.DeleteAllChatitemsToAddressByOwner).Methods("GET")
	router.HandleFunc("/delete_chatitem/{id}", controllers.DeleteChatitem).Methods("DELETE")
	router.HandleFunc("/get_inbox/{address}", controllers.GetInboxByOwner).Methods("GET")
	router.HandleFunc("/get_last_unread/{address}", controllers.GetLastMsgToOwner).Methods("GET")
	router.HandleFunc("/create_chatitem", controllers.CreateChatitem).Methods("POST")
	//router.HandleFunc("/create_chatitem_tmp", controllers.CreateChatitemTmp).Methods("POST")
	//router.HandleFunc("/getall_chatitems", controllers.GetAllChatitems).Methods("GET")
	router.HandleFunc("/block_user/{address}", controllers.BlockUser).Methods("GET")
	router.HandleFunc("/is_moderator/{company}/{address}", controllers.IsModerator).Methods("GET")

	//unreadcnt per week4 requirements
	router.HandleFunc("/unreadcount/{address}", controllers.GetUnreadcnt).Methods("GET", "OPTIONS")
	//router.HandleFunc("/unreadcount/{address}", controllers.PutUnreadcnt).Methods("PUT")

	//group chat
	router.HandleFunc("/create_groupchatitem", controllers.CreateGroupChatitem).Methods("POST")
	//router.HandleFunc("/get_groupchatitems/{address}", controllers.GetGroupChatItems).Methods("GET")
	router.HandleFunc("/get_groupchatitems/{address}/{useraddress}", controllers.GetGroupChatItemsByAddr).Methods("GET")
	router.HandleFunc("/get_groupchatitems_unreadcnt/{address}/{useraddress}", controllers.GetGroupChatItemsByAddrLen).Methods("GET")

	//community chat
	router.HandleFunc("/community/{community}/{address}", controllers.GetCommunityChat).Methods("GET") //TODO: make common
	router.HandleFunc("/community/{community}/{time}/{count}", controllers.GetCommunityChatAfterTime).Methods("GET")
	router.HandleFunc("/community_pagenum/{community}/{pagenum}", controllers.GetCommunityChatPage).Methods("GET")
	router.HandleFunc("/community", controllers.CreateCommunityChatItem).Methods("POST")
	router.HandleFunc("/create_community", controllers.CreateCommunity).Methods("POST")
	router.HandleFunc("/update_community", controllers.UpdateCommunity).Methods("POST")
	router.HandleFunc("/community/conditions", controllers.ChangeCommunityConditions).Methods("POST")

	//bookmarks
	router.HandleFunc("/create_bookmark", controllers.CreateBookmarkItem).Methods("POST")
	router.HandleFunc("/delete_bookmark", controllers.DeleteBookmarkItem).Methods("POST")
	router.HandleFunc("/get_bookmarks/{address}", controllers.GetBookmarkItems).Methods("GET")
	router.HandleFunc("/get_bookmarks/{walletaddr}/{nftaddr}", controllers.IsBookmarkItem).Methods("GET")

	//naming addresses (users or NFT collections)
	router.HandleFunc("/name", controllers.CreateAddrNameItem).Methods("POST")
	//router.HandleFunc("/name", controllers.UpdateAddrNameItem).Methods("PUT")
	router.HandleFunc("/name/{address}", controllers.GetAddrNameItem).Methods("GET")

	//Logos / Images stored in base64
	router.HandleFunc("/image", controllers.CreateImageItem).Methods("POST")
	router.HandleFunc("/image", controllers.UpdateImageItem).Methods("PUT")
	router.HandleFunc("/image/{addr}", controllers.GetImageItem).Methods("GET")
	router.HandleFunc("/imageraw", controllers.CreateRawImageItem).Methods("POST")
	router.HandleFunc("/imagepublic", controllers.CreatePublicImageItem).Methods("POST")
	router.HandleFunc("/imageraw/{imageid}", controllers.GetRawImageItem).Methods("GET")

	//settings items - currently this is the public key added upon first login for encryption/signing without MM
	//router.HandleFunc("/create_settings", controllers.CreateSettings).Methods("POST")
	router.HandleFunc("/update_settings", controllers.UpdateSettings).Methods("POST")
	router.HandleFunc("/get_settings/{address}", controllers.GetSettings).Methods("GET")
	router.HandleFunc("/delete_settings/{address}", controllers.DeleteSettings).Methods("DELETE")
	router.HandleFunc("/verify_email/{email}/{code}", controllers.VerifyEmail).Methods("GET")

	//comments on a specific NFT
	router.HandleFunc("/create_comments", controllers.CreateComments).Methods("POST")
	//router.HandleFunc("/get_comments", controllers.GetAllComments).Methods("GET") //doubt we will need this
	router.HandleFunc("/get_comments/{nftaddr}/{nftid}", controllers.GetComments).Methods("GET")
	router.HandleFunc("/delete_comments/{fromaddr}/{nftaddr}/{nftid}", controllers.DeleteComments).Methods("DELETE")

	//Twitter Related APIs
	router.HandleFunc("/get_twitter/{contract}", controllers.GetTwitter).Methods("GET")
	router.HandleFunc("/get_twitter_cnt/{contract}", controllers.GetTwitterCount).Methods("GET")
	router.HandleFunc("/get_comments_cnt/{nftaddr}/{nftid}", controllers.GetCommentsCount).Methods("GET")
	//in twitter.go
	router.HandleFunc("/search_tweets/{query_str}", twitter.SearchTweets).Methods("GET")
	router.HandleFunc("/get_followers/{query_str}", twitter.GetNumTwitterFollowers).Methods("GET") //admin only
	router.HandleFunc("/get_follower_counts", twitter.GetAllTwitterFollowerCount).Methods("GET")   //admin only

	//holder functions
	//TODO: this would need a signature from holder to fully verify - ok for now
	router.HandleFunc("/is_owner/{contract}/{wallet}", controllers.IsOwner).Methods("GET")
	router.HandleFunc("/rejoin_all/{wallet}", controllers.AutoJoinCommunities).Methods("GET")
	router.HandleFunc("/backfill_all_bookmarks", controllers.FixUpBookmarks).Methods("GET") //just meant for internal use - not for external use

	//POAP related stuff (some could be called client side directly but this protects the API key)
	router.HandleFunc("/get_poaps/{wallet}", controllers.GetPoapsByAddr).Methods("GET")

	//OpenSea Pass-Thru to prevent CORS error and API key leakage
	router.HandleFunc("/opensea_asset_contract/{contract}", controllers.GetOpenseaAssetContract).Methods("GET")
	router.HandleFunc("/opensea_collection_stats/{contract}", controllers.GetOpenseaCollectionStats).Methods("GET")
	router.HandleFunc("/opensea_asset/{nftaddr}/{nftid}/{address}", controllers.GetOpenseaAsset).Methods("GET")
	router.HandleFunc("/opensea_asset_owner/{address}", controllers.GetOpenseaAssetOwner).Methods("GET")
	router.HandleFunc("/opensea_asset_owner_ens/{address}", controllers.GetOpenseaAssetOwnerENS).Methods("GET")

	//WalletGuard Calls to prevent CORS errors and API key leakage
	router.HandleFunc("/wallet_guard_check", controllers.WalletGuardCheck).Methods("POST")

	//Leaderboard calls
	router.HandleFunc("/get_referral_code", referrals.GetReferralCode).Methods("GET")
	router.HandleFunc("/create_referral_code/{address}", referrals.CreateReferralCode).Methods("GET") //mainly for testing
	router.HandleFunc("/redeem_referral_code/{code}", referrals.RedeemReferralCode).Methods("GET")
	router.HandleFunc("/get_leaderboard_data", referrals.GetLeaderboardData).Methods("GET")
	router.HandleFunc("/get_valid_referred_user", referrals.GetHasEnteredValidCode).Methods("GET")
}

func initDB() {
	config :=
		database.Config{
			User:       "doadmin",
			Password:   os.Getenv("DB_PASSWORD"),
			ServerName: os.Getenv("DB_URL"),
			DB:         "walletchat",
		}
		// database.Config{
		// 	User:       "root",
		// 	Password:   "",
		// 	ServerName: "localhost:3306",
		// 	DB:         "walletchat",
		// }

	connectionString := database.GetConnectionString(config)
	err := database.Connect(connectionString)
	if err != nil {
		panic(err.Error())
	}

	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	// database.Connector.DB().SetMaxIdleConns(10)

	// // SetMaxOpenConns sets the maximum number of open connections to the database.
	// database.Connector.DB().SetMaxOpenConns(100)

	// // SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
	// database.Connector.DB().SetConnMaxLifetime(time.Hour)
	//These are supposed to help create the proper DB based on the data struct if it doesn't already exist
	//had some issues with it and just created the tables directly in MySQL (still have to match data structs)
	// database.Migrate(&entity.Settings{})
	//database.MigrateComments(&entity.Comments{})
	// database.MigrateChatitem(&entity.Chatitem{})
}
