package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"log"
	"net/http"
	"server/db"
	"server/s3"
)

func main() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/register", onRegister)

	log.Fatal(http.ListenAndServe(":8080", router))
}

func onRegister(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		http.Error(w, "Please send a request body", 400)
		return
	}
	user := &User{}
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	// check user exists
	if u := db.GetUserByToken(user.Token); u != nil {
		http.Error(w, "user already exists", 500)
		return
	}

	storage := s3.GetDefaultS3Storage()
	// binding storage to user
	user.StorageID = storage.ID
	userModel := user.ToModel()

	tx := func(tx *gorm.DB) error {
		err = db.CreateS3Storage(tx, storage)
		if err != nil {
			return err
		}
		err = db.CreateUser(tx, userModel)
		if err != nil {
			return err
		}
		return nil
	}

	session := db.GetTxnSession()
	err = session.Transaction(tx)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	return
}
