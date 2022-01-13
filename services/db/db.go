package db

import (
	"database/sql"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	sqlHost  = "127.0.0.1:4000"
	metaName = "pcloud_meta"
)

var (
	// DB represents a global single orm instance
	DB *gorm.DB
)

func init() {
	db, err := sql.Open("mysql", fmt.Sprintf("root:@tcp(%s)/", sqlHost))
	if err != nil {
		panic(err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE DATABASE IF NOT EXISTS " + metaName)
	if err != nil {
		panic(err)
	}

	DB, err = createDB()
	if err != nil {
		panic(err)
	}
}

func createDB() (*gorm.DB, error) {
	dsn := fmt.Sprintf("root:@tcp(%s)%s/?charset=utf8mb4&parseTime=True&loc=Local", sqlHost, metaName)
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}

func GetTxnSession() *gorm.DB {
	return DB.Session(&gorm.Session{SkipDefaultTransaction: true})
}
