package transcode

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/rekognition"
	rekognitionTypes "github.com/aws/aws-sdk-go-v2/service/rekognition/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/middleware"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"tycoon.systems/tycoon-services/s3credentials"
	"tycoon.systems/tycoon-services/structs"
	vpb "tycoon.systems/tycoon-services/video"
)

var (
	uri        = s3credentials.GetS3Data("mongo", "address", "")
	credential = options.Credential{
		AuthSource: "admin",
		Username:   s3credentials.GetS3Data("mongo", "u", ""),
		Password:   s3credentials.GetS3Data("mongo", "p", ""),
	}
	clientOpts = options.Client().ApplyURI(uri).
			SetAuth(credential)
	client, err     = mongo.Connect(context.TODO(), clientOpts)
	s3VideoEndpoint = s3credentials.GetS3Data("awsConfig", "buckets", "tycoon-systems-video")
	devEnv          = s3credentials.GetS3Data("app", "dev", "")
)

func main() {

}

func TranscodeVideoProcess(vid *vpb.Video, media []structs.MediaItem, configResolutions []int, step int) []structs.MediaItem {
	_, maxHeight := GetVideoSize(vid.GetPath())
	if step < len(configResolutions) {
		if configResolutions[step] <= maxHeight {
			var mediaItem structs.MediaItem
			newMedia := TranscodeSingleVideo(vid, mediaItem, configResolutions[step])
			media = append(media, newMedia)
		}
		return TranscodeVideoProcess(vid, media, configResolutions, step+1)
	}
	return FindClosedCaptions(vid, media) // Retrieve closed captions
}

func TranscodeAudioProcess(vid *vpb.Video, media []structs.MediaItem) []structs.MediaItem {
	var mediaItem structs.MediaItem
	newMedia := TranscodeSingleAudio(vid, mediaItem, "audio-main")
	media = append(media, newMedia)
	configResolutions := make([]int, 0)
	configResolutions = append(configResolutions, 2048, 1440, 720, 540, 360, 240) // Default resolutions to transcode
	return TranscodeVideoProcess(vid, media, configResolutions, 0)                // Transcode video files
}

func TranscodeSingleAudio(vid *vpb.Video, media structs.MediaItem, version string) structs.MediaItem {
	data, err := ffmpeg.Probe(vid.GetPath())
	if err != nil {
		fmt.Printf("Issue with probing video for audio data")
	}
	unstructuredData := make(map[string]interface{})
	channels := "2"
	json.Unmarshal([]byte(data), &unstructuredData)
	if _, ok := unstructuredData["streams"]; ok {
		if _, ok2 := unstructuredData["streams"].([]interface{}); ok2 {
			if _, ok3 := unstructuredData["streams"].([]interface{})[0].(map[string]interface{})["channels"]; ok3 {
				if _, ok4 := unstructuredData["streams"].([]interface{})[0].(map[string]interface{})["channels"].(string); ok4 {
					channels = unstructuredData["streams"].([]interface{})[0].(map[string]interface{})["channels"].(string)
				}
			}
		}
	}
	err = ffmpeg.Input(vid.GetPath()).
		Output(vid.GetDestination()+vid.GetID()+"-"+version+"-raw.mp4", ffmpeg.KwArgs{
			"vn":  "",    // Video none
			"c:a": "aac", // Convert all audio to aac
			"b:a": "256k",
			"ac":  channels,
		}).
		ErrorToStdOut().
		Run()
	if err != nil {
		fmt.Printf("Issue with transcoding single audio %v\nPath: %v\nOutput: %v\n", err, vid.GetPath(), vid.GetDestination()+vid.GetID()+"-"+version+"-raw.mp4")
		return media
	}
	return structs.MediaItem{
		Type: "audio",
		Url:  vid.GetID() + "-" + version + "-raw.mp4",
	}
}

func TranscodeSingleVideo(vid *vpb.Video, media structs.MediaItem, resolution int) structs.MediaItem {
	curRes := strconv.Itoa(resolution)
	err := ffmpeg.Input(vid.GetPath()).
		Output(vid.GetDestination()+vid.GetID()+"-"+curRes+"-raw.mp4", ffmpeg.KwArgs{
			"vf":          "scale=" + "-2:" + curRes,             // Sets scaled resolution with same ratios
			"c:v":         "libx264",                             // Set video codec
			"crf":         "24",                                  // Level of quality
			"tune":        "film",                                // Codec tune setting
			"x264-params": "keyint=24:min-keyint=24:no-scenecut", // Group of pictures setting
			"profile:v":   "baseline",
			"level":       "3.0",
			"pix_fmt":     "yuv420p",
			"preset":      "veryfast",   // Transcode speed
			"movflags":    "+faststart", // Move moov data to beginning of video with second pass for faster web start
			"x264opts":    "opencl",     // Enable opencl usage to improve speed of trancoding using GPU
		}).
		ErrorToStdOut().
		Run()
	if err != nil {
		fmt.Printf("Issue with transcoding Video %v\nPath: %v\nOutput: %v\n", err, vid.GetPath(), vid.GetDestination()+vid.GetID()+"-"+curRes+"-raw.mp4")
		return media
	}
	return structs.MediaItem{
		Type: "video",
		Url:  vid.GetID() + "-" + curRes + "-raw.mp4",
	}
}

func PackageManifest(vid *vpb.Video, media []structs.MediaItem, del bool) ([]structs.MediaItem, []structs.MediaItem) {
	app := "packager"
	argsSlice := make([]string, 0)
	re := regexp.MustCompile(`([a-zA-Z0-9].*)-raw(\.[a-zA-Z0-9].*)`) // 15cf28e1a0a048f6a2deecee161e4f8a-raw.mp4
	// reRaw := regexp.MustCompile(`([a-zA-Z0-9].*)-([a-zA-Z0-9].*)-raw(\.[a-zA-Z0-9].*)`) // 15cf28e1a0a048f6a2deecee161e4f8a-720-raw.mp4
	var liveMediaItems []structs.MediaItem
	for i := 0; i < len(media); i++ {
		if media[i].Type == "text" && media[i].Url != "bad" {
			// Do not add subtitle references to manifest for now using shaka packager. Functionality can be done manually using a script to run after completion of this process. For now client side will
			// manually grab vtt files from media property stored on record signified by operation bellow
			liveMediaItems = append(liveMediaItems, structs.MediaItem{
				Type: "text",
				Url:  media[i].Url,
			})
			media[i].Url = "" // Ignore file deletion here since we are not creating new files from "raw" files for vtt.
		} else if media[i].Url != "bad" {
			args := ""
			matchPath := re.FindAllStringSubmatch(media[i].Url, -1)
			// matchPath2 := reRaw.FindAllStringSubmatch(media[i].Url, -1)
			p := matchPath[0][1] + matchPath[0][2]
			args = args + "in=" + media[i].Url + ",stream=" + media[i].Type + ",output=" + p
			liveMediaItems = append(liveMediaItems, structs.MediaItem{
				Type: media[i].Type,
				Url:  p,
			})
			if media[i].Type == "audio" {
				p2 := matchPath[0][1] + ".m3u8"
				args = args + ",playlist_name=" + p2
				liveMediaItems = append(liveMediaItems, structs.MediaItem{
					Type: "hls-playlist",
					Url:  p2,
				})
				args = args + ",hls_group_id=audio,hls_name=ENGLISH"
			} else {
				p2 := matchPath[0][1] + "-" + media[i].Type + ".m3u8"
				args = args + ",playlist_name=" + p2
				liveMediaItems = append(liveMediaItems, structs.MediaItem{
					Type: "hls-playlist",
					Url:  p2,
				})
				p3 := matchPath[0][1] + "-" + media[i].Type + "_iframe.m3u8"
				args = args + ",iframe_playlist_name=" + p3
				liveMediaItems = append(liveMediaItems, structs.MediaItem{
					Type: "iframe-hls-playlist",
					Url:  p3,
				})
			}
			argsSlice = append(argsSlice, args)
		}
	}
	var expectedMpdPath string = vid.GetID() + "-mpd.mpd"
	liveMediaItems = append(liveMediaItems, structs.MediaItem{
		Type: "mpd",
		Url:  expectedMpdPath,
	})
	var expectedHlsPath string = vid.GetID() + "-hls.m3u8"
	liveMediaItems = append(liveMediaItems, structs.MediaItem{
		Type: "hls",
		Url:  expectedHlsPath,
	})
	argsSlice = append(argsSlice, "--mpd_output")
	argsSlice = append(argsSlice, expectedMpdPath)
	argsSlice = append(argsSlice, "--hls_master_playlist_output")
	argsSlice = append(argsSlice, expectedHlsPath)
	fmt.Printf("Shaka Command: %v %v\n", app, argsSlice)
	cmd := exec.Command(app, argsSlice...)
	cmd.Dir = vid.GetDestination()
	strderr, _ := cmd.StderrPipe()
	err := cmd.Start()
	slurp, _ := io.ReadAll(strderr)
	fmt.Printf("%s\n", slurp)
	if del {
		DeleteFile(vid.GetPath(), vid.GetDestination())
		DeleteMediaItemFiles(media, vid.GetDestination())
	}
	if err != nil {
		fmt.Printf("Error with packager: %v\n", err)
	}
	return liveMediaItems, media // liveData, old (deleted if del true)
}

func CheckAndUpdateRecord(vid *vpb.Video, status string) bool {
	if client == nil {
		return false
	}
	videos := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("videos")
	record := &structs.Video{}
	err := videos.FindOne(context.TODO(), bson.D{{"_id", vid.GetID()}}).Decode(&record) // Get from video data
	if err == mongo.ErrNoDocuments {
		defaultTitle, defaultDescription := ProbeDefaultMetadata(vid)
		document := structs.Video{
			ID:          vid.GetID(),
			Author:      vid.GetSocket(),
			Status:      status,
			Publish:     -1,
			Creation:    int(time.Now().UnixNano() / 1000000),
			Mpd:         "",
			Hls:         "",
			Media:       []structs.MediaItem{},
			Thumbnail:   "",
			Thumbtrack:  []structs.Thumbnail{},
			Title:       defaultTitle,
			Description: defaultDescription,
			Tags:        make([]interface{}, 0),
			Production:  "",
			Cast:        make([]interface{}, 0),
			Directors:   make([]interface{}, 0),
			Writers:     make([]interface{}, 0),
			Timeline:    make([]interface{}, 0),
			Duration:    FindDuration(vid),
		}
		videos.InsertOne(context.TODO(), document)
		return false
	} else {
		if record.Status != "waiting" {
			fmt.Printf("Job has already started processing. Preventing from running same task again. Current Status: %v. Exiting\n", record.Status)
			return true
		}
		opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
		var v structs.Video
		videos.FindOneAndUpdate(
			context.TODO(),
			bson.D{{"_id", vid.GetID()}},
			bson.M{"$set": bson.M{"status": status}},
			opts).
			Decode(&v)
	}
	fmt.Printf("Not running. Current Status: %v\n", record.Status)
	return false
}

func UpdateMongoRecord(vid *vpb.Video, media []structs.MediaItem, status string, thumbtrack []structs.Thumbnail, once bool) (any, error) {
	if client == nil {
		return nil, fmt.Errorf("Database client connection unavailable %v\n", err)
	}
	videos := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("videos")
	record := &structs.Video{}
	err := videos.FindOne(context.TODO(), bson.D{{"_id", vid.GetID()}}).Decode(&record) // Get from phone number data
	if err != nil {
		fmt.Printf("Mongo Update Err %v\n", err)
	}
	if err == mongo.ErrNoDocuments {
		defaultTitle, defaultDescription := ProbeDefaultMetadata(vid)
		document := structs.Video{
			ID:          vid.GetID(),
			Author:      vid.GetSocket(),
			Status:      status,
			Publish:     -1,
			Creation:    int(time.Now().UnixNano() / 1000000),
			Mpd:         "",
			Hls:         "",
			Media:       media,
			Thumbnail:   "",
			Thumbtrack:  []structs.Thumbnail{},
			Title:       defaultTitle,
			Description: defaultDescription,
			Tags:        make([]interface{}, 0),
			Production:  "",
			Cast:        make([]interface{}, 0),
			Directors:   make([]interface{}, 0),
			Writers:     make([]interface{}, 0),
			Timeline:    make([]interface{}, 0),
			Duration:    FindDuration(vid),
		}
		insertedDocument, err2 := videos.InsertOne(context.TODO(), document)
		if err2 != nil {
			return nil, err2
		}
		return insertedDocument, nil
	} else {
		if record.Status != "processing" && status == "waiting" && once == true {
			return nil, nil
		}
		trimmedMediaData := media
		m, err := CleanUpStrayData(media)
		if err == nil {
			trimmedMediaData = m
		}
		document := structs.Video{ // Upsert document
			ID:          vid.GetID(),
			Author:      vid.GetSocket(),
			Status:      status,
			Publish:     -1,
			Creation:    record.Creation,
			Mpd:         FindMediaOfType(media, "mpd"),
			Hls:         FindMediaOfType(media, "hls"),
			Media:       trimmedMediaData,
			Thumbnail:   FindMediaOfType(media, "thumbnail"),
			Thumbtrack:  thumbtrack,
			Title:       record.Title,
			Description: record.Description,
			Tags:        record.Tags,
			Production:  record.Production,
			Cast:        record.Cast,
			Directors:   record.Directors,
			Writers:     record.Writers,
			Timeline:    record.Timeline,
			Duration:    record.Duration,
		}
		opts := options.FindOneAndUpdate().SetUpsert(true)
		opts = options.FindOneAndUpdate().SetReturnDocument(options.After)
		newDoc, _ := toDoc(document)
		var v structs.Video
		videos.FindOneAndUpdate(
			context.TODO(),
			bson.D{{"_id", vid.GetID()}},
			bson.M{"$set": newDoc},
			opts).
			Decode(&v)
		return v, nil
	}
	return nil, nil
}

func ProbeDefaultMetadata(vid *vpb.Video) (string, string) {
	var title string = ""
	var description string = ""
	data, err := ffmpeg.Probe(vid.GetPath())
	if err != nil {
		return title, description
	}
	unstructuredData := make(map[string]interface{})
	json.Unmarshal([]byte(data), &unstructuredData)
	if _, ok := unstructuredData["format"]; ok {
		if _, ok2 := unstructuredData["format"].(map[string]interface{})["tags"]; ok2 {
			if _, ok3 := unstructuredData["format"].(map[string]interface{})["tags"].(map[string]interface{})["description"]; ok3 {
				if _, ok4 := unstructuredData["format"].(map[string]interface{})["tags"].(map[string]interface{})["description"].(string); ok4 {
					description = unstructuredData["format"].(map[string]interface{})["tags"].(map[string]interface{})["description"].(string)
				}
			}
			if _, ok5 := unstructuredData["format"].(map[string]interface{})["tags"].(map[string]interface{})["title"]; ok5 {
				if _, ok6 := unstructuredData["format"].(map[string]interface{})["tags"].(map[string]interface{})["title"].(string); ok6 {
					title = unstructuredData["format"].(map[string]interface{})["tags"].(map[string]interface{})["title"].(string)
				}
			}
		}
	}
	return title, description
}

func FindDefaultThumbnail(thumbtrack []structs.Thumbnail, media []structs.MediaItem) []structs.MediaItem {
	if len(thumbtrack) != 0 {
		for i := 5; i > 0; i-- {
			if len(thumbtrack) > i {
				media = append(media, structs.MediaItem{
					Type: "thumbnail",
					Url:  thumbtrack[i].Url,
				})
				break
			}
		}
	}
	return media
}

func toDoc(v any) (doc *bson.D, err error) {
	data, err := bson.Marshal(v)
	if err != nil {
		return
	}

	err = bson.Unmarshal(data, &doc)
	return
}

func UploadToServers(liveMediaItems []structs.MediaItem, destination string, uploadFolder string, thumbDir string) error {
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
	if devEnv == "true" {
		s3VideoEndpoint = s3credentials.GetS3Data("awsConfig", "devBuckets", "tycoon-systems-video-development")
	}
	fmt.Printf("s3VideoEndpoint %v\n", s3VideoEndpoint)
	for i := 0; i < len(liveMediaItems); i++ {
		upFrom := destination
		upTo := uploadFolder
		if liveMediaItems[i].Type == "thumbnail" {
			upFrom = thumbDir
			upTo = "thumbnail/"
		} else if liveMediaItems[i].Type == "text" {
			upTo = "text/"
		}
		f, _ := os.Open(upFrom + liveMediaItems[i].Url)
		r := bufio.NewReader(f)
		fmt.Printf("Uploading: %v %v\n", upFrom+liveMediaItems[i].Url, upTo+liveMediaItems[i].Url)
		_, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(s3VideoEndpoint),
			Key:    aws.String(upTo + liveMediaItems[i].Url),
			Body:   r,
		})
		if err != nil {
			return err
		}
		f.Close() // Close stream to prevent permissions issue
	}
	return nil
}

func UploadThumbtrackToServers(thumbtrack []structs.Thumbnail, destination string, uploadFolder string) error {
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
	if devEnv == "true" {
		s3VideoEndpoint = s3credentials.GetS3Data("awsConfig", "devBuckets", "tycoon-systems-video-development")
	}
	fmt.Printf("s3VideoEndpoint %v\n", s3VideoEndpoint)
	for i := 0; i < len(thumbtrack); i++ {
		f, _ := os.Open(destination + thumbtrack[i].Url)
		r := bufio.NewReader(f)
		fmt.Printf("Uploading: %v\n", uploadFolder+thumbtrack[i].Url)
		_, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(s3VideoEndpoint),
			Key:    aws.String(uploadFolder + thumbtrack[i].Url),
			Body:   r,
		})
		if err != nil {
			return err
		}
		f.Close() // Close stream to prevent permissions issue
	}
	return nil
}

func CleanUpStrayData(media []structs.MediaItem) ([]structs.MediaItem, error) {
	for i := len(media) - 1; i > 0; i-- {
		if media[i].Type == "thumbnail" {
			media = append(media[:i], media[i+1:]...)
		}
	}
	return media, nil
}

func GenerateThumbnailTrack(vid *vpb.Video, thumbtrack []structs.Thumbnail) ([]structs.Thumbnail, string) {
	var thumbDir string = vid.GetDestination() + vid.GetID() + "-thumbs"
	os.MkdirAll(thumbDir, os.ModePerm)
	err := ffmpeg.Input(vid.GetPath()).
		Output(thumbDir+"/"+vid.GetID()+"-thumb%03d.jpg", ffmpeg.KwArgs{
			"vf":  "select='not(mod(n,300))',setpts='N/(30*TB)',scale=-2:180",
			"f":   "image2",
			"q:v": "6", // Quality of image. 6 is reasonable, thumbtrack total comes to half a mb for a 30 minute episode of Atlanta "The Jacket" with mod(n,300)
		}).
		ErrorToStdOut().
		Run()
	if err != nil {
		fmt.Printf("Err Generating thumbnail track - GENERATE: %v\n", err)
		return []structs.Thumbnail{}, ""
	}
	files, err := ioutil.ReadDir(thumbDir)
	if err != nil {
		fmt.Printf("Err Generating thumbnail track - READ DIR: %v\n", err)
		OrganizeAndDeleteThumbnails(thumbtrack, files, thumbDir)
		return []structs.Thumbnail{}, ""
	}
	data, err := ffmpeg.Probe(vid.GetPath())
	if err != nil {
		fmt.Printf("Err Generating thumbnail track - PROBE: %v\n", err)
		OrganizeAndDeleteThumbnails(thumbtrack, files, thumbDir)
		return []structs.Thumbnail{}, ""
	}
	unstructuredData := make(map[string]interface{})
	json.Unmarshal([]byte(data), &unstructuredData)
	fmt.Printf("Streaming %v", unstructuredData)
	if len(files) > 0 {
		var ti float64 = 0
		for i, file := range files {
			if i >= len(files) { // Double check to ensure no panic
				break
			}
			t := structs.Thumbnail{
				Time: fmt.Sprintf("%.2f", ti),
				Url:  file.Name(),
			}
			thumbtrack = append(thumbtrack, t)
			ti = ti + 10
		}
		return thumbtrack, thumbDir + "/"
	}
	fmt.Printf("No thumbnail files generated")
	OrganizeAndDeleteThumbnails(thumbtrack, files, thumbDir)
	return []structs.Thumbnail{}, ""
}

func ScheduleProfanityCheck(vid *vpb.Video, media []structs.MediaItem) (structs.Video, error) {
	firstVideo := ""
	for i := 0; i < len(media); i++ {
		if media[i].Type == "video" {
			firstVideo = media[i].Url
			break
		}
	}
	if firstVideo == "" {
		return structs.Video{}, err
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
		return structs.Video{}, err
	}
	if devEnv == "true" {
		s3VideoEndpoint = s3credentials.GetS3Data("awsConfig", "devBuckets", "tycoon-systems-video-development")
	}
	rekognitionClient := rekognition.NewFromConfig(cfg)
	fmt.Printf("Dev Env %v, Video to Check on s3 %v\n", s3VideoEndpoint, "video/"+firstVideo)
	startContentModerationOutput, err := rekognitionClient.StartContentModeration(
		context.TODO(),
		&rekognition.StartContentModerationInput{
			Video: &rekognitionTypes.Video{
				S3Object: &rekognitionTypes.S3Object{
					Bucket: aws.String(s3VideoEndpoint),
					Name:   aws.String("video/" + firstVideo),
				},
			},
			ClientRequestToken: aws.String(vid.GetID()),
			JobTag:             aws.String("video"),
			NotificationChannel: &rekognitionTypes.NotificationChannel{
				RoleArn:     aws.String(s3credentials.GetS3Data("awsConfig", "rekognitionRoleArnId", "")),
				SNSTopicArn: aws.String(s3credentials.GetS3Data("awsConfig", "rekognitionSnsTopicArnId", "")),
			},
		},
	)
	if err != nil {
		fmt.Printf("Error starting content moderation: %v\n", err)
		return structs.Video{}, err
	}
	o := rekognition.StartContentModerationOutput{
		JobId:          startContentModerationOutput.JobId,
		ResultMetadata: startContentModerationOutput.ResultMetadata,
	}
	jobId := *o.JobId
	var m middleware.Metadata = o.ResultMetadata
	fmt.Printf("Rekognition Job Id %v\n", jobId)
	fmt.Printf("Metadata %v\n", m)
	co, err := rekognitionClient.GetContentModeration(
		context.TODO(),
		&rekognition.GetContentModerationInput{
			JobId: aws.String(jobId),
		},
	)
	if err != nil {
		fmt.Printf("Error retrieving job id data: %v\n", err)
		return structs.Video{}, err
	}
	fmt.Printf("Content moderation %v", co)
	if client == nil {
		return structs.Video{}, errors.New("No client for database connection")
	}
	videos := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("videos")
	record := &structs.Video{}
	err = videos.FindOne(context.TODO(), bson.D{{"_id", vid.GetID()}}).Decode(&record) // Get from phone number data
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var v structs.Video
	videos.FindOneAndUpdate(
		context.TODO(),
		bson.D{{"_id", vid.GetID()}},
		bson.M{"$set": bson.M{"status": "check:" + jobId}},
		opts).
		Decode(&v)
	return *record, nil
}

func DeleteFolder(dir string) error {
	err := os.Remove(dir)
	if err != nil {
		return err
	}

	return nil
}

func OrganizeAndDeleteThumbnails(thumbtrack []structs.Thumbnail, files []fs.FileInfo, path string) error {
	if files != nil {
		for i := 0; i < len(files); i++ {
			t := structs.Thumbnail{
				Time: "",
				Url:  files[i].Name(),
			}
			thumbtrack = append(thumbtrack, t)
		}
		DeleteThumbnails(thumbtrack, path)
	}
	return nil
}

func DeleteThumbnails(stale []structs.Thumbnail, dir string) error {
	for i := 0; i < len(stale); i++ {
		DeleteFile(stale[i].Url, dir)
	}
	return nil
}

func DeleteMediaItemFiles(stale []structs.MediaItem, dir string) error {
	for i := 0; i < len(stale); i++ {
		if len(stale[i].Url) > 0 {
			DeleteFile(stale[i].Url, dir)
		}
	}
	return nil
}

func DeleteFile(stale string, dir string) error {
	path := dir
	path = path + stale
	err := os.Remove(path)
	if err != nil {
		fmt.Printf("Error deleting Err: %v. File: %v\n", err, path)
	}
	return nil
}

func GetVideoSize(path string) (int, int) {
	data, err := ffmpeg.Probe(path)
	if err != nil {
		return -1, -1
	}
	type VideoInfo struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int
			Height    int
		} `json:"streams"`
	}
	videoInfo := &VideoInfo{}
	err = json.Unmarshal([]byte(data), videoInfo)
	if err != nil {
		return -1, -1
	}
	for _, s := range videoInfo.Streams {
		if s.CodecType == "video" {
			return s.Width, s.Height
		}
	}
	return -1, -1
}

func FindMediaOfType(media []structs.MediaItem, t string) string {
	for i := 0; i < len(media); i++ {
		if media[i].Type == t {
			return media[i].Url
		}
	}
	return ""
}

func FindClosedCaptions(vid *vpb.Video, media []structs.MediaItem) []structs.MediaItem {
	data, err := ffmpeg.Probe(vid.GetPath())
	if err != nil {
		return media
	}
	unstructuredData := make(map[string]interface{})
	json.Unmarshal([]byte(data), &unstructuredData)
	fmt.Printf("Data before Find Closed Captions %v\n", unstructuredData)
	for i := 0; i < 3; i++ {
		out := vid.GetDestination() + vid.GetID() + "-" + strconv.Itoa(i) + "-subtitle" + ".vtt"
		err := ffmpeg.Input(vid.GetPath()).
			Output(out, ffmpeg.KwArgs{
				"map": "0:s:" + strconv.Itoa(i),
			}).
			ErrorToStdOut().
			Run()
		if err == nil {
			media = append(media, structs.MediaItem{
				Type: "text",
				Url:  vid.GetID() + "-" + strconv.Itoa(i) + "-subtitle" + ".vtt",
			})
		}
	}
	return media
}

func FindDuration(vid *vpb.Video) int {
	fmt.Printf("Find and record duration")
	data, err := ffmpeg.Probe(vid.GetPath())
	if err != nil {
		fmt.Printf("Issue with probing video for audio data")
	}
	unstructuredData := make(map[string]interface{})
	json.Unmarshal([]byte(data), &unstructuredData)
	fmt.Printf("Data %v", unstructuredData)
	if _, ok := unstructuredData["format"]; ok {
		if _, ok2 := unstructuredData["format"].(map[string]interface{})["duration"]; ok2 {
			if _, ok3 := unstructuredData["format"].(map[string]interface{})["duration"].(string); ok3 {
				resolvedInt, err2 := strconv.Atoi(strings.Split(unstructuredData["format"].(map[string]interface{})["duration"].(string), ".")[0])
				if err2 == nil {
					return resolvedInt
				} else {
					fmt.Printf("Error resolving duration of Video %v", err2)
				}
			}
		}
	}
	return 0
}
