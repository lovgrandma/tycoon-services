package video_queue

import (
	"tycoon.systems/tycoon-services/s3credentials"
	// "go.mongodb.org/mongo-driver/bson"
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
	// "github.com/go-redis/redis/v8"
	"time"
	
	vpb "tycoon.systems/tycoon-services/video"
	"tycoon.systems/tycoon-services/video/video_queue/transcode"
	// "google.golang.org/grpc"
	// "os"
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
	jobQueueAddr = s3credentials.GetS3Data("redis", "redishost", "") + ":" + s3credentials.GetS3Data("redis", "tycoon_systems_video_queue_port", "")
	jobClient = asynq.NewClient(asynq.RedisClientOpt{Addr: jobQueueAddr})
	returnJobResultPort = "6002"
	returnJobResultAddr = s3credentials.GetS3Data("app", "prodhost", "")
)

const (
	TypeVideoProcess = "video:process"
)

func main() {
	
}

func ProvisionVideoJob(vid *vpb.Video) string {
	if reflect.TypeOf(vid.ID).Kind() == reflect.String &&
	reflect.TypeOf(vid.Status).Kind() == reflect.String &&
	reflect.TypeOf(vid.Socket).Kind() == reflect.String &&
	reflect.TypeOf(vid.Destination).Kind() == reflect.String &&
	reflect.TypeOf(vid.Filename).Kind() == reflect.String &&
	reflect.TypeOf(vid.Path).Kind() == reflect.String {
		task, err := NewVideoProcessTask(vid)
		if err != nil {
			log.Printf("Could not create Video Process task at Task Creation: %v", err)
		}
		info, err := jobClient.Enqueue(task)
		if err != nil {
			log.Printf("Could not create Video Process task at Enqueue: %v", err)
		}
		return info.ID
	}
	return "failed"
}

func GetConnection() *mongo.Client {
	return client
}

// Build new delivery to be consumed by queue
func NewVideoProcessTask(vid *vpb.Video) (*asynq.Task, error) {
	payload, err := json.Marshal(vid)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeVideoProcess, payload), nil
}

// Unmarshal queued delivery task to determine if in correct format
func HandleVideoProcessTask(ctx context.Context, t *asynq.Task) error {
	var p *vpb.Video
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	log.Printf("Beginning Video Process Transcode Job for User Socket: %v, Video ID: %v", p.GetSocket(), p.GetID())
	err := PerformVideoProcess(p)
	if err != nil {
		return fmt.Errorf("Perform Video Process failed: %v: %w", err, asynq.SkipRetry)
	}
	return nil
}

func PerformVideoProcess(vid *vpb.Video) error {
	configResolutions := make([]int, 0)
	configResolutions = append(configResolutions, 2048, 1440, 720, 540, 360, 240) // Default resolutions to transcode
	var transcodedMedia []structs.MediaItem = transcode.TranscodeAudioProcess(vid, []structs.MediaItem{}) // Transcode main audio included in video file
	transcodedMedia = transcode.TranscodeVideoProcess(vid, transcodedMedia, configResolutions, 0) // Transcode video files
	transcodedMedia = transcode.FindClosedCaptions(vid, transcodedMedia) // Retrieve closed captions
	fmt.Printf("Transcoded Media so far: %v", transcodedMedia)
	var thumbtrack []structs.Thumbnail
	var thumbDir string
	thumbtrack, thumbDir = transcode.GenerateThumbnailTrack(vid, thumbtrack)
	var liveMediaItems []structs.MediaItem
	liveMediaItems, _ = transcode.PackageManifest(vid, transcodedMedia, true) // Package manifest files
	liveMediaItems = transcode.FindDefaultThumbnail(thumbtrack, liveMediaItems)
	doc, _ := transcode.UpdateMongoRecord(vid, liveMediaItems, "check", thumbtrack) // Update record
	err := transcode.UploadToServers(liveMediaItems, vid.GetDestination(), "video/", thumbDir) // Send to Streaming servers
	if err != nil {
		fmt.Printf("issue with uploading to S3 %v", err)
	}
	err = transcode.UploadThumbtrackToServers(thumbtrack, thumbDir, "thumbtrack/") // Send thumbtrack files to streaming servers
	if err != nil {
		fmt.Printf("issue with uploading to S3 %v", err)
	}
	liveMediaItems, err = transcode.CleanUpStrayData(liveMediaItems)
	if err != nil {
		fmt.Printf("Issue with clean up %v", err)
	}
	time.Sleep(2 * time.Second)
	transcode.DeleteMediaItemFiles(liveMediaItems, vid.GetDestination()) // Delete media files
	transcode.DeleteThumbnails(thumbtrack, thumbDir)
	transcode.DeleteFolder(vid.GetDestination() + vid.GetID() + "-thumbs")
	transcode.ScheduleProfanityCheck(vid)
	fmt.Printf("Transcoded Media %v\nLiveItems %v\nDoc %v\nThumbtrack %v\nJob Finished\n", transcodedMedia, liveMediaItems, doc, thumbtrack)
	return nil
}

func resolveBadJob(id string, filename string) {

}