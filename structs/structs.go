package structs

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func main() {

}

type User struct {
	ID            string `bson:"_id" json:"id,omitempty"`
	GID           string
	Email         string
	Username      string
	Numbers       []interface{}
	Icon          string
	Payment       string
	Subscriptions []interface{}
}

/* Message */

type Msg struct {
	From     string
	Content  string
	JobId    string
	Domain   string
	Function string
	User     string
}

type Number struct {
	ID       string `bson:"_id" json:"id,omitempty"`
	number   string
	UserId   string
	SmsUrl   string
	VoiceUrl string
	Status   string
	Sid      string
	Subs     []interface{}
	Chats    []interface{}
}

type FromObj struct {
	From   string
	Filter []interface{}
}

type ChatLog struct {
	ID    string
	Users []interface{}
	Log   []interface{}
	Host  string
}

/* Video */

type Video struct {
	ID          string `bson:"_id" json:"id,omitempty"`
	Author      string
	Status      string
	Publish     string
	Creation    string
	Mpd         string
	Hls         string
	Media       []MediaItem
	Thumbnail   string
	Thumbtrack  []Thumbnail
	Title       string
	Description string
	Tags        []interface{}
	Production  string
	Cast        []interface{}
	Directors   []interface{}
	Writers     []interface{}
	Timeline    []interface{}
	Duration    string
	Domain      string
}

type MediaItem struct {
	Type string
	Url  string
}

type Thumbnail struct {
	Time string
	Url  string
}

type TimelineNode struct {
	Type   string // Type "do-not-play-ads", "do-not-play-midrolls", "ad-marker", "inline-ad-marker", "product"
	Amount int    // e.g amount of midrolls 1-4
	Time   int    // Time of node activation
	Data   TimelineData
}

type TimelineData struct {
	UpTime int // Time in seconds that timeline node stays up
}

/* Ad */

type VastTag struct {
	ID              string `bson:"_id" json:"id,omitempty"`
	Status          string
	Socket          string
	Url             string
	DocumentId      string
	TrackingUrl     string
	AdTitle         string
	ClickthroughUrl string
	CallToAction    string
	StartTime       string
	EndTime         string
	PlayTime        string
	Domain          string
}

type AdUnit struct {
	ID                string `bson:"_id" json:"id,omitempty"`
	AdType            string
	DocumentId        string
	UserId            string
	Status            string
	Publish           bool
	Creation          int
	CallToAction      string
	AdTitle           string
	AdDescription     string
	ClickthroughUrl   string
	MaxCampaignBudget int
	DailyBudget       int
	CurrentBudgetUse  float32 `bson:"currentbudgetuse,truncate"`
	AdvertEndTime     int
	Vast              string
	Vpaid             string
	Media             []string
	Duration          Duration
	History           []AdHistoryItem
	Domain            string
}

type Duration struct {
	StartTime string
	EndTime   string
	PlayTime  string
}

type AdHistoryItem struct {
	CallToAction      string
	AdTitle           string
	AdDescription     string
	ClickthroughUrl   string
	MaxCampaignBudget string
	DailyBudget       string
	AdvertEndTime     string
}

/* Misc */

type Origin struct {
	Name string
}

/* Invoice */

type InvoiceItem struct {
	Name    string
	Details string
	Data    string
	Units   float64
	Cost    float64
	Note    string
	AdTitle string
	AdId    string
}

type Invoice struct {
	ID              primitive.ObjectID `bson:"_id"`
	StripeId        string
	Date            string
	Note            string
	Customer        string
	CustomerAddress string
	CustomerNetwork string
	Payee           string
	PayeeAddress    string
	PayeeNetwork    string
	Owed            float64
	Currency        string
	Data            string
	FootDetails     string
	Thankyou        string
	Paid            float64
	History         []string
}

type EmptyObject struct {
}
