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
	// "strings"
	"log"
	"github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
	// notifyapi "github.com/twilio/twilio-go/rest/notify/v1"
	messagingapi "github.com/twilio/twilio-go/rest/messaging/v1"
	"github.com/go-redis/redis/v8"
	"tycoon.systems/tycoon-services/sms/sms_utility"
	"time"
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
	rdb = redis.NewClient(&redis.Options{
        Addr:     s3credentials.GetS3Data("redis", "redishost", "") + ":" + s3credentials.GetS3Data("redis", "mpstnumbersport", ""),
        Password: "", // no password set
        DB:       0,  // use default DB
    })
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
		// log.Printf("Enqueued Sms Delivery task: %v %v %v %v %v %v %v", info.ID, info.Queue, info.State, info.Timeout, info.LastErr, info.Type, info)
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
	err := coll.FindOne(context.TODO(), bson.D{{"number", msg.From}}).Decode(&record) // Get from phone number data
	if err != nil {
		return fmt.Errorf("failure %v", err)
	}
	twilioClient := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: s3credentials.GetS3Data("twilioMpst", "sid", ""),
		Password: s3credentials.GetS3Data("twilioMpst", "authToken", ""),
	})
	service, err := twilioClient.MessagingV1.CreateService(&messagingapi.CreateServiceParams{
		FriendlyName: &msg.From,
	})
	if err != nil {
		return fmt.Errorf("failure %v", err)
	}
	var numSid *string
	numSid = &record.Sid
	createdNumber, err := twilioClient.MessagingV1.CreatePhoneNumber(*service.Sid, &messagingapi.CreatePhoneNumberParams{
		PhoneNumberSid: numSid,
	})
	if err != nil {
		DeleteService(*twilioClient, *service.Sid)
		return fmt.Errorf("failure %v", err)
	}
	switch reflect.TypeOf(record.Subs).Kind() {
		case reflect.Slice:
			s := reflect.ValueOf(record.Subs)
			for i := 0; i < s.Len(); i++ {
				if reflect.TypeOf(record.Subs[i]).Kind() == reflect.String {
					var to string = record.Subs[i].(string)
					params := &openapi.CreateMessageParams{
						To: &to,
						From: &msg.From,
						Body: &msg.Content,
					}
					_, err := twilioClient.ApiV2010.CreateMessage(params)
					if err != nil {
						fmt.Printf("Err sending message to %v. Err: %v", to, err)
					}
					UpdateRedisConverstaion(to, msg.From, msg.Content, i)
				}
			}
		default:
			fmt.Printf("Invalid subs record type, expecting string")
	}
	RemovePhoneNumberFromService(*twilioClient, *service.Sid, *createdNumber.Sid)
	DeleteService(*twilioClient, *service.Sid)
	return nil
}

func RemovePhoneNumberFromService(twilioClient twilio.RestClient, serviceSid string, phoneSid string) error {
	err := twilioClient.MessagingV1.DeletePhoneNumber(serviceSid, phoneSid)
	return err
}

func DeleteService(twilioClient twilio.RestClient, serviceSid string) error {
	err := twilioClient.MessagingV1.DeleteService(serviceSid)
	return err
}

func UpdateRedisConverstaion(to string, from string, content string, i int) error {
	resolvedKey := sms_utility.ResolveKey(to, from)
	var ctx = context.Background()
	result, err := rdb.Get(ctx, resolvedKey).Result()
	if err == redis.Nil {
		fmt.Printf("Conversation does not exist")
	} else if err != nil {
		return fmt.Errorf("Cannot update Redis converstaion for to: %v, from: %v, content: %v, Err: %v", to, from, content, err)
	} else {
		var chatLog structs.ChatLog
		json.Unmarshal([]byte(result), &chatLog)
		t := time.Now()
		newLog := make(map[string]interface{})
		newLog["author"] = from
		newLog["content"] = content
		newLog["timestamp"] = t.Format("2006/01/02 3:04:05 PM")
		chatLog.Log = append(chatLog.Log, newLog)
		safeChatLog := make(map[string]interface{}) 
		safeChatLog["id"] = chatLog.Id
		safeChatLog["users"] = chatLog.Users
		safeChatLog["log"] = chatLog.Log
		safeChatLog["host"] = chatLog.Host
		memoryReadyData, _ := json.Marshal(safeChatLog)
		rdb.Set(ctx, resolvedKey, memoryReadyData, 0)
	}
	return nil
}