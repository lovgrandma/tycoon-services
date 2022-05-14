package sms_queue

import (
	"tycoon.systems/tycoon-services/s3credentials"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"context"
	"tycoon.systems/tycoon-services/structs"
	"github.com/hibiken/asynq"
	"reflect"
	"encoding/json"
	"fmt"
	"strings"
	"log"
	"github.com/twilio/twilio-go"
	// openapi "github.com/twilio/twilio-go/rest/api/v2010"
	notifyapi "github.com/twilio/twilio-go/rest/notify/v1"
	messagingapi "github.com/twilio/twilio-go/rest/messaging/v1"
)

var (
	uri = s3credentials.GetS3Data("mongo", "addressAuth", "")
	credential = options.Credential{
		AuthSource: "admin",
		Username: s3credentials.GetS3Data("mongo", "u", ""),
		Password: s3credentials.GetS3Data("mongo", "p", ""),
	}
	clientOpts = options.Client().ApplyURI(uri).
   		SetAuth(credential)
	client, err = mongo.Connect(context.TODO(), clientOpts)
	jobQueueAddr = s3credentials.GetS3Data("redis", "redishost", "") + ":" + s3credentials.GetS3Data("redis", "tycoon_systems_queue_port", "")
	jobClient = asynq.NewClient(asynq.RedisClientOpt{Addr: jobQueueAddr})
)

const (
	TypeSmsDelivery		= "sms:deliver"
	_phoneAlreadyListedCheck = "Phone Number or Short Code is already in the Messaging Service"
)

func main() {
	
}

func ProvisionSmsJob(msg structs.Msg) string {
	if reflect.TypeOf(msg.Content).Kind() == reflect.String &&
	reflect.TypeOf(msg.From).Kind() == reflect.String &&
	reflect.TypeOf(msg.JobId).Kind() == reflect.String {
		task, err := NewSmsDeliveryTask(msg)
		if err != nil {
			log.Printf("Could not create Sms Delivery task at Task Creation: %v", err)
		}
		info, err := jobClient.Enqueue(task)
		if err != nil {
			log.Printf("Could not create Sms Delivery task at Enqueue: %v", err)
		}
		log.Printf("Enqueued Sms Delivery task: %v %v %v %v %v %v %v", info.ID, info.Queue, info.State, info.Timeout, info.LastErr, info.Type, info)
		return info.ID
	}
	return "failed"
}

func GetConnection() *mongo.Client {
	return client
}

// Build new delivery to be consumed by queue
func NewSmsDeliveryTask(msg structs.Msg) (*asynq.Task, error) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSmsDelivery, payload), nil
}

// Unmarshal queued delivery task to determine if in correct format
func HandleSmsDeliveryTask(ctx context.Context, t *asynq.Task) error {
	var p structs.Msg
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	log.Printf("Beginning Sms Blast Delivery for From: %v, Content: %v", p.From, p.Content)
	err := PerformSmsDelivery(p)
	if err != nil {
		return fmt.Errorf("Perform Sms Delivery failed: %v: %w", err, asynq.SkipRetry)
	}
	return nil
}

func PerformSmsDelivery(msg structs.Msg) error {
	if client == nil {
		return fmt.Errorf("Database client connection unavailable %v", err)
	}
	coll := client.Database("mpst").Collection("numbers")
	record := &structs.Number{}
	err := coll.FindOne(context.TODO(), bson.D{{"number", msg.From}}).Decode(&record)
	if err != nil {
		return fmt.Errorf("failure %v", err)
	}
	twilioClient := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: s3credentials.GetS3Data("twilioMpst", "sid", ""),
		Password: s3credentials.GetS3Data("twilioMpst", "authToken", ""),
	})
	var pg int = 1000
	var lmt int = 1000000
	services, err := twilioClient.MessagingV1.ListService(&messagingapi.ListServiceParams{
		PageSize: &pg,
		Limit: &lmt,
	})
	if err != nil {
		return fmt.Errorf("failure %v", err)	
	} else if services != nil {
		if reflect.TypeOf(*services[0].FriendlyName).Kind() == reflect.String {
			strVal := *services[0].FriendlyName
			if strVal == "tycoon-services-messaging" {
				var numSid *string
				numSid = &record.Sid
				viableNumber, err := twilioClient.MessagingV1.CreatePhoneNumber(*services[0].Sid, &messagingapi.CreatePhoneNumberParams{
					PhoneNumberSid: numSid,
				})
				if (err != nil && strings.Contains(err.Error(), _phoneAlreadyListedCheck) == true) || viableNumber != nil {
					fmt.Printf("GOood")
					var bodyStr *string
					bodyStr = &msg.Content
					notification, err := twilioClient.NotifyV1.CreateNotification(*services[0].Sid, &notifyapi.CreateNotificationParams{
						Body: bodyStr,
					})
					if err != nil {
						return fmt.Errorf("failure %v", err)
					} else {
						fmt.Printf("Notif %v", notification)
					}
				} else {
					return fmt.Errorf("failure %v", err)
				}
			}
		}
	}
	// twilio.notify.services(s3credentials.GetS3Data("twilioMpst", "notifyServicesSid", ""))
	switch reflect.TypeOf(record.Subs).Kind() {
		case reflect.Slice:
			s := reflect.ValueOf(record.Subs)
			for i := 0; i < s.Len(); i++ {
				fmt.Println(s.Index(i))
			}
    }
	return nil
}