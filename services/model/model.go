package model

import (
	"database/sql"
	"time"
)

type S3Storage struct {
	ID           string `json:"id"`
	Endpoint     string `json:"endpoint"`
	Region       string `json:"region"`
	Bucket       string `json:"bucket"`
	Prefix       string `json:"prefix"`
	StorageClass string `json:"storage_class"`
	// server side encryption
	Sse             string `json:"sse"`
	Acl             string `json:"acl"`
	AccessKey       string `json:"access_key"`
	SecretAccessKey string `json:"secret_access_key"`
	ForcePathStyle  bool   `json:"force_path_style"`
	SseKmsKeyId     string `json:"sse_kms_key_id"`
}

type User struct {
	ID        uint
	Name      string
	StorageID string
	Token     sql.NullString
	CreatedAt time.Time
	UpdatedAt time.Time
}
