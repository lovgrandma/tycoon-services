package video_queue

import (
	"strconv"

	"tycoon.systems/tycoon-services/s3credentials"
	// "go.mongodb.org/mongo-driver/bson"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hibiken/asynq"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"tycoon.systems/tycoon-services/structs"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	// "strings"
	"log"
	// "github.com/go-redis/redis/v8"
	"time"

	"os"

	"io"
	"io/ioutil"
	"net/http"
	"regexp"

	"sort"

	"os/exec"
	"strings"

	"path/filepath"

	"google.golang.org/grpc"
	"tycoon.systems/tycoon-services/security"
	vpb "tycoon.systems/tycoon-services/video"
	"tycoon.systems/tycoon-services/video/video_queue/transcode"
)

var (
	uri        = s3credentials.GetS3Data("mongo", "addressAuth", "")
	credential = options.Credential{
		AuthSource: "admin",
		Username:   s3credentials.GetS3Data("mongo", "u", ""),
		Password:   s3credentials.GetS3Data("mongo", "p", ""),
	}
	clientOpts = options.Client().ApplyURI(uri).
			SetAuth(credential)
	client, err              = mongo.Connect(context.TODO(), clientOpts)
	jobQueueAddr             = s3credentials.GetS3Data("redis", "redishost", "") + ":" + s3credentials.GetS3Data("redis", "tycoon_systems_video_queue_port", "")
	jobClient                = asynq.NewClient(asynq.RedisClientOpt{Addr: jobQueueAddr})
	returnJobResultPort      = s3credentials.GetS3Data("app", "services", "videoServer")
	returnJobResultAddr      = s3credentials.GetS3Data("app", "prodhost", "")
	routingServicesProd      = s3credentials.GetS3Data("app", "routingServer", "")
	routingServicesLocalPort = s3credentials.GetS3Data("app", "routingLocalPort", "")
	tempVideoPath            = s3credentials.GetS3Data("app", "tempVideoPath", "")
)

const (
	TypeVideoProcess               = "video:process"
	TypeLivestreamThumbnailProcess = "livestream:process"
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
		info, err := jobClient.Enqueue(task, asynq.Timeout(5*time.Hour))
		if err != nil {
			log.Printf("Could not create Video Process task at Enqueue: %v", err)
		}
		return info.ID
	}
	return "failed"
}

func ProvisionLivestreamThumbnailJob(vid *vpb.Video) string {
	if reflect.TypeOf(vid.ID).Kind() == reflect.String &&
		reflect.TypeOf(vid.Status).Kind() == reflect.String &&
		reflect.TypeOf(vid.Socket).Kind() == reflect.String &&
		reflect.TypeOf(vid.Destination).Kind() == reflect.String &&
		reflect.TypeOf(vid.Filename).Kind() == reflect.String &&
		reflect.TypeOf(vid.Path).Kind() == reflect.String {
		task, err := NewLivestreamThumbnailProcessTask(vid)
		if err != nil {
			log.Printf("Could not create Livestream Thumbnail Process task at Task Creation: %v", err)
		}
		info, err := jobClient.Enqueue(task, asynq.Timeout(5*time.Hour))
		if err != nil {
			log.Printf("Could not create Livestream Thumbnail Process task at Enqueue: %v", err)
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

func NewLivestreamThumbnailProcessTask(vid *vpb.Video) (*asynq.Task, error) {
	payload, err := json.Marshal(vid)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeLivestreamThumbnailProcess, payload), nil
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

// Unmarshal queued delivery task to determine if in correct format
func HandleLivestreamThumbnailProcessTask(ctx context.Context, t *asynq.Task) error {
	var p *vpb.Video
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	log.Printf("Beginning Livestream Thumbnail Process Transcode Job for User Socket: %v, Video ID: %v", p.GetSocket(), p.GetID())
	err := PerformLivestreamThumbnailProcess(p)
	if err != nil {
		return fmt.Errorf("Perform Livestream Thumbnail Process failed: %v: %w", err, asynq.SkipRetry)
	}
	return nil
}

func downloadFile(url, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	return nil
}

func DownloadTsFiles(vid *vpb.Video, matches []string, count int, path string) ([]string, error) {
	// Download the highest files
	var highestFiles []string
	for i := 0; i < count && i < len(matches); i++ {
		fmt.Printf("DL TS %v %v", path+matches[i], tempVideoPath+vid.GetID())
		fileURL := path + matches[i]
		filename := tempVideoPath + vid.GetID()

		err := downloadFile(fileURL, filename)
		if err != nil {
			fmt.Println("Error downloading file", fileURL, ":", err)
		} else {
			highestFiles = append(highestFiles, filename)
		}
	}

	return highestFiles, nil
}

func PerformLivestreamThumbnailProcess(vid *vpb.Video) error {
	fmt.Printf("Running Livestream Task %v %v", vid, tempVideoPath)
	// Status: "processing", ID: name, Socket: "", Destination: cdn, Filename: name, Path: path, Domain: domain
	url := vid.GetDestination() + "/" + vid.GetPath() + "/" + vid.GetID() + "/" + "index.m3u8"
	resp, err := http.Get(url) // read directory
	if err != nil {
		fmt.Println("Error:", err)
		return nil
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error:", err)
		return nil
	}

	pattern := `[^/\n]+\.ts`

	fmt.Printf("Body %v %v", string(body), vid.GetDestination()+"/"+vid.GetPath()+"/"+vid.GetID()+"/"+"index.m3u8")

	// Find all matches
	matches := regexp.MustCompile(pattern).FindAllString(string(body), -1)

	// Sort the matches in ascending order get highest value .ts files in directory
	sort.Slice(matches, func(i, j int) bool {
		num1, _ := strconv.Atoi(matches[i][:len(matches[i])-3])
		num2, _ := strconv.Atoi(matches[j][:len(matches[j])-3])
		return num1 < num2
	})

	highestFiles, err := DownloadTsFiles(vid, matches, 1, vid.GetDestination()+"/"+vid.GetPath()+"/"+vid.GetID()+"/")
	fmt.Printf("TS Files %v", highestFiles)
	mustDelete := highestFiles
	if err != nil {
		// attempt delete files
		for _, file := range mustDelete {
			transcode.DeleteFile(file, "")
		}
		return nil
	}
	// Extract three images from each of the highest files
	for _, file := range highestFiles {
		err := extractImagesFromTS(vid, file, 1, tempVideoPath)
		if err != nil {
			// attempt delete files
			fmt.Printf("Error Extracting %v", err)
			for _, file := range mustDelete {
				transcode.DeleteFile(file, "")
			}
			return nil
		}
	}

	// Convert extracted images to a GIF
	err = convertImagesToGIF(vid, tempVideoPath, vid.GetID(), tempVideoPath)
	if err != nil {
		fmt.Println("Error converting images to GIF:", err)
		// attempt delete files
		for _, file := range mustDelete {
			transcode.DeleteFile(file, "")
		}
		return nil
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(s3credentials.GetS3Data("awsConfig", "mediaBucketLocation1", "")),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: aws.Credentials{
				AccessKeyID:     s3credentials.GetS3Data("awsConfig", "accessKeyId", ""),
				SecretAccessKey: s3credentials.GetS3Data("awsConfig", "secretAccessKey", ""),
			},
		}),
	)
	if err != nil {
		return err
	}
	client := s3.NewFromConfig(cfg)
	uploader := manager.NewUploader(client)
	f, _ := os.Open(tempVideoPath + vid.GetID() + "_preview.gif")
	r := bufio.NewReader(f)
	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{ // upload to s3
		Bucket: aws.String(vid.GetDomain() + "-live"),
		Key:    aws.String("gif/" + vid.GetID() + "_preview.gif"),
		Body:   r,
	})
	if err != nil {
		return err
	}
	f.Close()                                                          // Close stream to prevent permissions issue
	transcode.DeleteFile(tempVideoPath+vid.GetID()+"_preview.gif", "") // delete gif
	// store on livestream record
	query := `
		mutation FindOneAndUpdateLive($schemaname: String!, $field: String!, $value: String!, $fieldActionMatch: String!, $newValue: String!) {
			findOneAndUpdateLive(schemaname: $schemaname, field: $field, value: $value, fieldActionMatch: $fieldActionMatch, newValue: $newValue) {
				id
				author
				status
				publish
				creation
				media
				thumbnail
				thumbtrack
				title
				description
				tags
				production
				cast
				directors
				writers
				timeline
				duration
				staticvideoref
			}
		}
	`
	// Prepare the GraphQL request payload
	payload := map[string]interface{}{
		"query": query,
		"variables": map[string]string{
			"schemaname":       vid.GetDomain(),
			"field":            "id",
			"value":            vid.GetID(),
			"fieldActionMatch": "gif",
			"newValue":         "gif/" + vid.GetID() + "_preview.gif",
		},
	}
	security.RunGraphqlQuery(payload, "POST", s3credentials.GetS3Data("graphql", "endpoint", ""), "", "findOneLive", vid.GetDomain())
	return nil
}

// Extract images from a .ts file using FFmpeg
func extractImagesFromTS(vid *vpb.Video, tsFile string, count int, outputDir string) error {
	filename := strings.TrimSuffix(tsFile, ".ts")
	outputPattern := fmt.Sprintf("%s_p_%%d.jpg", filename)

	fmt.Printf("tsFile %v Output Pattern %v", tsFile, outputPattern)
	cmd := exec.Command("ffmpeg", "-i", tsFile, "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),showinfo", count), "-vsync", "vfr", "-f", "image2", outputPattern)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to extract images from %s: %v", tsFile, err)
	}
	return nil
}

// Convert extracted images to a GIF using FFmpeg
func convertImagesToGIF(vid *vpb.Video, inputDir string, outputFilename string, outputDir string) error {
	inputPattern := fmt.Sprintf("%s%v_p_%%d.jpg", inputDir, vid.GetID())
	cmd := exec.Command("ffmpeg", "-i", inputPattern, "-vf", "palettegen", "-y", outputDir+outputFilename+"_palette.png")
	err := cmd.Run()
	if err != nil {
		cleanup(outputDir, outputFilename)
		return fmt.Errorf("failed to generate palette for GIF: %v", err)
	}
	cmd = exec.Command("ffmpeg", "-i", inputPattern, "-i", outputDir+outputFilename+"_palette.png", "-lavfi", "paletteuse", "-y", outputDir+outputFilename+"_preview.gif")
	err = cmd.Run()
	if err != nil {
		cleanup(outputDir, outputFilename)
		return fmt.Errorf("failed to convert images to GIF: %v", err)
	}
	cleanup(outputDir, outputFilename)
	return nil
}

func cleanup(outputDir string, outputFilename string) error {
	transcode.DeleteFile(outputDir+outputFilename+"_palette.png", "")
	transcode.DeleteFile(outputDir+outputFilename, "")
	files, _ := filepath.Glob(outputDir + "*" + outputFilename + "_p_*")

	for _, file := range files {
		os.Remove(file)
	}
	return nil
}

func PerformVideoProcess(vid *vpb.Video) error {
	var alreadyRunning bool = transcode.CheckAndUpdateRecord(vid, "processing") // Build initial record for tracking during processing
	if alreadyRunning {
		return nil
	}
	defaultTitle, defaultDescription, defaultDuration, err := transcode.ProbeDefaultMetadata(vid)
	fmt.Printf("Defaults %v %v %v %v\n", defaultTitle, defaultDescription, defaultDuration, err)
	var vidDefaults structs.Video
	if err == nil {
		vidDefaults = structs.Video{
			ID:          vid.GetID(),
			Title:       defaultTitle,
			Description: defaultDescription,
			Duration:    defaultDuration,
			Domain:      vid.GetDomain(),
		}
		transcode.FindOneAndUpdateVideoField(vidDefaults, "duration", vidDefaults.Duration)
		transcode.FindOneAndUpdateVideoField(vidDefaults, "title", vidDefaults.Title)
		transcode.FindOneAndUpdateVideoField(vidDefaults, "description", vidDefaults.Description)
	}
	var transcodedMedia []structs.MediaItem = transcode.TranscodeAudioProcess(vid, []structs.MediaItem{}) // Transcode Main Audio included in Video File -> Transcode Video Files -> Transcode Subtitles
	fmt.Printf("Transcoded Media so far: %v\n", transcodedMedia)
	var thumbtrack []structs.Thumbnail
	var thumbDir string
	thumbtrack, thumbDir = transcode.GenerateThumbnailTrack(vid, thumbtrack)
	var liveMediaItems []structs.MediaItem
	liveMediaItems, _ = transcode.PackageManifest(vid, transcodedMedia, true) // Package manifest files
	liveMediaItems = transcode.FindDefaultThumbnail(thumbtrack, liveMediaItems)
	doc, err := transcode.UpdateMongoRecord(vid, liveMediaItems, "upload", thumbtrack, false) // Update record
	if err != nil {
		fmt.Printf("Error Transcoding %v", err)
	}
	err = transcode.UploadToServers(liveMediaItems, vid.GetDestination(), "video/", thumbDir) // Send to Streaming servers
	if err != nil {
		fmt.Printf("issue with uploading to S3 %v\n", err)
	}
	err = transcode.UploadThumbtrackToServers(thumbtrack, thumbDir, "thumbtrack/") // Send thumbtrack files to streaming servers
	if err != nil {
		fmt.Printf("issue with uploading to S3 %v\n", err)
	}
	liveMediaItems, err = transcode.CleanUpStrayData(liveMediaItems)
	if err != nil {
		fmt.Printf("Issue with clean up %v\n", err)
	}
	time.Sleep(2 * time.Second)
	transcode.DeleteMediaItemFiles(liveMediaItems, vid.GetDestination()) // Delete media files
	transcode.DeleteThumbnails(thumbtrack, thumbDir)
	transcode.DeleteFolder(vid.GetDestination() + vid.GetID() + "-thumbs")
	var finalRecord structs.Video
	finalRecord, err = transcode.ScheduleProfanityCheck(vid, liveMediaItems)
	if err != nil {
		fmt.Printf("Error scheduling profanity check %v\n", err)
		return nil
	}
	returnFinishedJobReport(finalRecord, vid.GetDomain())
	fmt.Printf("Transcoded Media %v\nLiveItems %v\nDoc %v\nThumbtrack %v\nJob Finished\n", transcodedMedia, liveMediaItems, doc, thumbtrack)
	return nil
}

func returnFinishedJobReport(vid structs.Video, domain string) {
	useReturnJobResultAddr := returnJobResultAddr
	useReturnJobResultPort := returnJobResultPort
	var connAddr string
	if os.Getenv("dev") == "true" {
		useReturnJobResultAddr = "localhost"
		if domain != "public" {
			useReturnJobResultPort = routingServicesLocalPort // set to local routing services instance server // MUST CHANGE LATER, INVALID FOR STATIC VIDEO MUST CREATE SERVICE ON ROUTING LOCAL
		}
		connAddr = useReturnJobResultAddr + ":" + useReturnJobResultPort
	} else {
		connAddr = useReturnJobResultAddr + ":" + useReturnJobResultPort
		if domain != "public" {
			connAddr = routingServicesProd // Set to routing services server
		}
	}
	conn, err := grpc.Dial(connAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		fmt.Printf("Err: %v", err)
	}
	if err == nil {
		defer conn.Close()
		c := vpb.NewVideoManagementClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		c.ReturnVideoJobResult(ctx, &vpb.Video{
			ID:          vid.ID,
			Status:      vid.Status,
			Socket:      vid.Author,
			Destination: "",
			Filename:    "",
			Path:        "",
		})
	}
}

func resolveBadJob(id string, filename string) {

}
