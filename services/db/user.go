package db

import (
	"gorm.io/gorm"
	"server/model"
)

func CreateUser(session *gorm.DB, user *model.User) error {
	result := session.Create(user)
	return result.Error
}

func GetUserByToken(token string) *model.User {
	user := &model.User{}
	result := DB.Session(&gorm.Session{}).Where("name = ?", token).First(user)
	if result.Error != nil {
		return nil
	}
	return user
}
