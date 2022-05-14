package controllers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"rest-go-demo/database"
	"rest-go-demo/entity"
	"time"

	"github.com/gorilla/mux"
)

//GetAllInbox get all inboxes data
// func GetAllInbox(w http.ResponseWriter, r *http.Request) {
// 	var inbox []entity.Inbox
// 	database.Connector.Find(&inbox)
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusOK)
// 	json.NewEncoder(w).Encode(inbox)
// }

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

//GetInboxByID returns the latest message for each unique conversation
//TODO: properly design the relational DB structs to optimize this search/retrieve
func GetInboxByOwner(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["address"]

	//fmt.Printf("GetInboxByOwner: %#v\n", key)

	//get all items that relate to passed in owner/address
	var chat []entity.Chatitem
	database.Connector.Where("fromaddr = ?", key).Or("toaddr = ?", key).Find(&chat)

	//get unique conversation addresses
	var uniqueChatMembers []string
	for _, chatitem := range chat {
		//fmt.Printf("search for unique addrs")
		if chatitem.Fromaddr != key {
			if !stringInSlice(chatitem.Fromaddr, uniqueChatMembers) {
				//fmt.Printf("Unique Addr Found: %#v\n", chatitem.Fromaddr)
				uniqueChatMembers = append(uniqueChatMembers, chatitem.Fromaddr)
			}
		}
		if chatitem.Toaddr != key {
			if !stringInSlice(chatitem.Toaddr, uniqueChatMembers) {
				//fmt.Printf("Unique Addr Found: %#v\n", chatitem.Toaddr)
				uniqueChatMembers = append(uniqueChatMembers, chatitem.Toaddr)
			}
		}
	}

	//fmt.Printf("find first message now")
	//for each unique chat member that is not the owner addr, get the latest message
	var userInbox []entity.Chatitem
	for _, chatmember := range uniqueChatMembers {
		var firstItem entity.Chatitem
		var secondItem entity.Chatitem
		var firstItems []entity.Chatitem
		var secondItems []entity.Chatitem
		fmt.Printf("Unique Chat Addr Check for : %#v\n", chatmember)
		// rowsto, err := database.Connector.DB().Query("SELECT * FROM chatitems WHERE fromaddr = ? AND toaddr = ? ORDER BY id DESC", chatmember, key)
		// if err != nil {
		// 	fmt.Printf("error 1")
		// }
		// for rowsto.Next() {
		// 	rowsto.Scan(&firstItem)
		// }
		// rowsfrom, err := database.Connector.DB().Query("SELECT * FROM chatitems WHERE fromaddr = ? AND toaddr = ? ORDER BY id DESC", key, chatmember)
		// if err != nil {
		// 	fmt.Printf("error 2")
		// }
		// for rowsfrom.Next() {
		// 	rowsfrom.Scan(&secondItem)
		// }

		database.Connector.Where("fromaddr = ?", chatmember).Where("toaddr = ?", key).Order("id desc").Find(&firstItems)
		if len(firstItems) > 0 {
			firstItem = firstItems[0]
		}
		fmt.Printf("FirstItem : %#v\n", firstItem)
		database.Connector.Where("fromaddr = ?", key).Where("toaddr = ?", chatmember).Order("id desc").Find(&secondItems)
		if len(secondItems) > 0 {
			secondItem = secondItems[0]
		}
		fmt.Printf("SecondItem : %#v\n", secondItem)

		//pick the most recent message
		if firstItem.Fromaddr != "" {
			if secondItem.Fromaddr == "" {
				userInbox = append(userInbox, firstItem)
			} else {
				layout := "2006-01-02T15:04:05.000Z"
				firstTime, error := time.Parse(layout, firstItem.Timestamp)
				if error != nil {
					//fmt.Println(error)
					return
				}
				secondTime, error := time.Parse(layout, secondItem.Timestamp)
				if error != nil {
					//fmt.Println(error)
					return
				}

				if firstTime.After(secondTime) {
					userInbox = append(userInbox, firstItem)
				} else {
					userInbox = append(userInbox, secondItem)
				}
			}
		} else if secondItem.Fromaddr != "" {
			userInbox = append(userInbox, secondItem)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userInbox)
}

//*********chat info*********************
//GetAllChatitems get all chat data
func GetAllChatitems(w http.ResponseWriter, r *http.Request) {
	//log.Println("get all chats")
	var chat []entity.Chatitem
	database.Connector.Find(&chat)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(chat)
}

//GetChatFromAddressToOwner returns all chat items from user to owner
func GetChatFromAddress(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["address"]

	var chat []entity.Chatitem
	database.Connector.Where("fromaddr = ?", key).Or("toaddr = ?", key).Find(&chat)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chat)
}

//CreateChatitem creates Chatitem
func CreateChatitem(w http.ResponseWriter, r *http.Request) {
	requestBody, _ := ioutil.ReadAll(r.Body)
	var chat entity.Chatitem
	json.Unmarshal(requestBody, &chat)

	database.Connector.Create(chat)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(chat)
}

//UpdateInboxByOwner updates person with respective owner address
func UpdateChatitemByOwner(w http.ResponseWriter, r *http.Request) {
	requestBody, _ := ioutil.ReadAll(r.Body)
	var chat entity.Chatitem

	json.Unmarshal(requestBody, &chat)

	//for now only support updating the message and read status
	database.Connector.Model(&entity.Chatitem{}).
		Where("fromaddr = ?", chat.Fromaddr).
		Where("toaddr = ?", chat.Toaddr).
		Where("timestamp = ?", chat.Timestamp).
		Update("message", chat.Message)
	database.Connector.Model(&entity.Chatitem{}).
		Where("fromaddr = ?", chat.Fromaddr).
		Where("toaddr = ?", chat.Toaddr).
		Where("timestamp = ?", chat.Timestamp).
		Update("unread", chat.Msgread)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(chat)
}

func DeleteAllChatitemsToAddressByOwner(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	to := vars["toAddr"]
	owner := vars["fromAddr"]

	var chat entity.Chatitem

	database.Connector.Where("toAddr = ?", to).Where("fromAddr = ?", owner).Delete(&chat)
	w.WriteHeader(http.StatusNoContent)
}

func CreateSettings(w http.ResponseWriter, r *http.Request) {
	requestBody, _ := ioutil.ReadAll(r.Body)
	var settings entity.Settings
	json.Unmarshal(requestBody, &settings)

	database.Connector.Create(settings)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(settings)
}

func UpdateSettings(w http.ResponseWriter, r *http.Request) {
	requestBody, _ := ioutil.ReadAll(r.Body)
	var settings entity.Settings

	json.Unmarshal(requestBody, &settings)
	database.Connector.Model(&entity.Settings{}).Where("walletaddr = ?", settings.Walletaddr).Update("publickey", settings.Publickey)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(settings)
}

func DeleteSettings(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["address"]

	var settings entity.Settings

	database.Connector.Where("walletaddr = ?", key).Delete(&settings)
	w.WriteHeader(http.StatusNoContent)
}

func GetSettings(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["address"]

	var settings []entity.Settings
	database.Connector.Where("walletaddr = ?", key).Find(&settings)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}
