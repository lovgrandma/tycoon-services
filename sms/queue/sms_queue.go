package sms_queue

import (
	"log"
	"github.com/go-bongo/bongo"
	"tycoon.systems/tycoon-services/s3credentials"
)

var (
	config = &bongo.Config{
		ConnectionString: s3credentials.GetS3Data("mongo", "addressAuth", ""),
		Database:         s3credentials.GetS3Data("mongo", "u", ""),
	}
	connection, err = bongo.Connect(config)
)

func main() {

}

func provisionSmsJob() {
	if err != nil {
		log.Fatal(err)
	}
}