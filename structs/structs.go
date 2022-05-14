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

type Msg struct {
    From string
    Content string
    JobId string
}

type Number struct {
	ID string `bson:"_id" json:"id,omitempty"`
	number string
	UserId string
	SmsUrl string
	VoiceUrl string
	Status string
	Sid string
	Subs []interface{}
	Chats []interface{}
}