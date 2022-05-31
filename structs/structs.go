package structs

import (
	//"go.mongodb.org/mongo-driver/bson/primitive"
)

func main() {

}

type User struct {
	ID 				string `bson:"_id" json:"id,omitempty"`
	GID 			string 
	Email 			string
	Username 		string
	Numbers 		[]interface{}
	Icon 			string
	Payment 		string
	Subscriptions 	[]interface{}
}

/* Message */

type Msg struct {
    From 			string
    Content 		string
    JobId			string
}

type Number struct {
	ID 				string `bson:"_id" json:"id,omitempty"`
	number 			string
	UserId 			string
	SmsUrl 			string
	VoiceUrl 		string
	Status 			string
	Sid 			string
	Subs 			[]FromObj
	Chats 			[]interface{}
}

type FromObj struct {
	From			string
	Filter			[]interface{}
}

type ChatLog struct {
	ID 				string
	Users 			[]interface{}
	Log 			[]interface{}
	Host 			string
}

/* Video */

type Video struct {
	ID				string `bson:"_id" json:"id,omitempty"`
	Author			string
	Status			string
	Publish			int
	Creation		int
	Mpd				string
	Hls				string
	Media			[]MediaItem
	Title			string
	Description		string
	Tags			[]interface{}
	Production		string
	Cast			[]interface{}
	Directors		[]interface{}
	Writers			[]interface{}
}

type MediaItem struct {
	Type			string
	Url				string
}
