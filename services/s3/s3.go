package s3

import (
	"github.com/google/uuid"
	"server/model"
)

const (
	defaultRegion          = "us-west-2"
	defaultAccessKey       = ""
	defaultSecretAccessKey = ""
)

func getAvailableBucket() string {
	// TODO create and get available bucket for different user
	return "pcloud2021"
}

func GetDefaultS3Storage() *model.S3Storage {
	// TODO refine id
	uuid := uuid.New()

	return &model.S3Storage{
		ID:              uuid,
		Bucket:          getAvailableBucket(),
		Prefix:          uuid,
		Region:          defaultRegion,
		AccessKey:       defaultAccessKey,
		SecretAccessKey: defaultSecretAccessKey,
	}
}
