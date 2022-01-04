package db

import (
	"gorm.io/gorm"
	"server/model"
)

func CreateS3Storage(session *gorm.DB, storage *model.S3Storage) error {
	result := session.Create(storage)
	return result.Error
}
