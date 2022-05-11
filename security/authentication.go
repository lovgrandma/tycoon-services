package security

import (
	"tycoon.systems/tycoon-services/sms/sms_queue"
	"go.mongodb.org/mongo-driver/bson"
	"fmt"
	"context"
	"tycoon.systems/tycoon-services/structs"
)

func CheckAuthenticRequest(username string, from string, identifier string, hash string) bool {
	client := sms_queue.GetConnection()
	fmt.Printf("Connection %v", client)
	coll := client.Database("mpst").Collection("users")
	record := &structs.User{}
	err := coll.FindOne(context.TODO(), bson.D{{ "username", username}}).Decode(&record)
	if err != nil {
		fmt.Printf("Err %v", err)
	} else {
		fmt.Printf("Username %v, Email %v, Numbers %v, ID %v, GID %v", record.Username, record.Email, record.Numbers, record.ID, record.GID)
	}
	return true
}