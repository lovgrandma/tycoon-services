package security

import (
	"tycoon.systems/tycoon-services/sms/sms_queue"
	"go.mongodb.org/mongo-driver/bson"
	"fmt"
	"context"
	"tycoon.systems/tycoon-services/structs"
	"golang.org/x/crypto/bcrypt"
)

func CheckAuthenticRequest(username string, from string, identifier string, hash string) bool {
	client := sms_queue.GetConnection()
	if client != nil {
		coll := client.Database("mpst").Collection("users")
		record := &structs.User{}
		err := coll.FindOne(context.TODO(), bson.D{{ "username", username}}).Decode(&record)
		if err != nil {
			fmt.Printf("Err %v", err)
		} else {
			err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(identifier))
			if err == nil {
				return true // Success, authenticated request
			}
		}
	}
	return false
}