package structs

import (
	//"go.mongodb.org/mongo-driver/bson/primitive"
)

func main() {

}

type User struct {
	ID string `bson:"_id" json:"id,omitempty"`
	GID string 
	Email string
	Username string
	Numbers []interface{}
	Icon string
	Payment string
	Subscriptions []interface{}
}