package structs

//"go.mongodb.org/mongo-driver/bson/primitive"

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
	From    string
	Content string
	JobId   string
}

type Number struct {
	ID       string `bson:"_id" json:"id,omitempty"`
	number   string
	UserId   string
	SmsUrl   string
	VoiceUrl string
	Status   string
	Sid      string
	Subs     []FromObj
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
	Publish     int
	Creation    int
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
	Duration    int
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
	AdvertEndTime     int
	Vast              string
	Vpaid             string
	History           []AdHistoryItem
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
