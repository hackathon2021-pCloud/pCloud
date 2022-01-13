package main

import (
	"database/sql"
	"server/model"
	"time"
)

// User represents API request
type User struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	StorageID string `json:"storage_id"`
}

// ToModel transforms the request to DB model
func (u *User) ToModel() *model.User {
	t := time.Now()
	valid := true
	if len(u.Token) == 0 {
		valid = false
	}
	return &model.User{
		ID:        u.ID,
		Name:      u.Name,
		StorageID: u.StorageID,
		Token:     sql.NullString{String: u.Token, Valid: valid},
		CreatedAt: t,
		UpdatedAt: t,
	}
}
