package transcode

import (
	"tycoon.systems/tycoon-services/s3credentials"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	vpb "tycoon.systems/tycoon-services/video"
	"tycoon.systems/tycoon-services/structs"
	"fmt"
	"encoding/json"
	"strconv"
	"os/exec"
	"os"
	"io"
	"bufio"
	"regexp"
	"context"
	"time"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"io/ioutil"
	"io/fs"
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
		return TranscodeVideoProcess(vid, media, configResolutions, step + 1)
	}
	return media;
}

func TranscodeAudioProcess(vid *vpb.Video, media []structs.MediaItem) []structs.MediaItem {
	var mediaItem structs.MediaItem
	newMedia := TranscodeSingleAudio(vid, mediaItem, "audio-main")
	media = append(media, newMedia)
	return media;
}

func TranscodeSingleAudio(vid *vpb.Video, media structs.MediaItem, version string) structs.MediaItem {
	data, err := ffmpeg.Probe(vid.GetPath())
	if err != nil {
		fmt.Printf("Issue with probing video for audio data")
	}
	unstructuredData := make(map[string]interface{})
	channels := "2"
	json.Unmarshal([]byte(data), &unstructuredData)
	fmt.Printf("Unstructured Data %v", &unstructuredData)
	if _, ok := unstructuredData["streams"]; ok {
		if _, ok2 := unstructuredData["streams"].([]interface{}); ok2 {
			if _, ok3 := unstructuredData["streams"].([]interface{})[0].(map[string]interface{})["channels"]; ok3 {
				if _, ok4 := unstructuredData["streams"].([]interface{})[0].(map[string]interface{})["channels"].(string); ok4 {
					channels = unstructuredData["streams"].([]interface{})[0].(map[string]interface{})["channels"].(string)
				} else {
					channels = "6"
				}
			} else {
				channels = "6"
			}
		}
	}
	err = ffmpeg.Input(vid.GetPath()).
		Output(vid.GetDestination() + vid.GetID() + "-" + version + "-raw.mp4", ffmpeg.KwArgs{
			"vn": "", // Video none
			"c:a": "aac", // Convert all audio to aac
			"b:a": "256k",
			"ac": channels,
		}).
		ErrorToStdOut().
		Run()
	if err != nil {
		return structs.MediaItem{
			Type: "audio",
			Url: "bad",
		}
	}
	return structs.MediaItem{
		Type: "audio",
		Url: vid.GetID() + "-" + version + "-raw.mp4",
	};
}

func TranscodeSingleVideo(vid *vpb.Video, media structs.MediaItem, resolution int) structs.MediaItem {
	curRes := strconv.Itoa(resolution)
	err := ffmpeg.Input(vid.GetPath()).
		Output(vid.GetDestination() + vid.GetID() + "-" + curRes + "-raw.mp4", ffmpeg.KwArgs{
			"vf": "scale=" + "-2:" + curRes,
			"c:v": "libx264",
			"crf": "24",
			"tune": "film",
			"x264-params": "keyint=24:min-keyint=24:no-scenecut",
			"profile:v": "baseline",
			"level": "3.0",
			"pix_fmt": "yuv420p",
		}).
		ErrorToStdOut().
		Run()
	if err != nil {
		return structs.MediaItem{
			Type: "video",
			Url: "bad",
		}
	}
	return structs.MediaItem{
		Type: "video",
		Url: vid.GetID() + "-" + curRes + "-raw.mp4",
	};
}

func PackageManifest(vid *vpb.Video, media []structs.MediaItem, del bool) ([]structs.MediaItem, []structs.MediaItem) {
	app := "packager"
	argsSlice := make([]string, 0)
	re := regexp.MustCompile(`([a-zA-Z0-9].*)-raw(\.[a-zA-Z0-9].*)`) // 15cf28e1a0a048f6a2deecee161e4f8a-raw.mp4
	// reRaw := regexp.MustCompile(`([a-zA-Z0-9].*)-([a-zA-Z0-9].*)-raw(\.[a-zA-Z0-9].*)`) // 15cf28e1a0a048f6a2deecee161e4f8a-720-raw.mp4
	var liveMediaItems []structs.MediaItem
	for i := 0; i < len(media); i++ {
		args := ""
		if media[i].Url != "bad" {
			matchPath := re.FindAllStringSubmatch(media[i].Url, -1)
			// matchPath2 := reRaw.FindAllStringSubmatch(media[i].Url, -1)
			p := matchPath[0][1] + matchPath[0][2]
			args = args + "in=" + media[i].Url + ",stream=" + media[i].Type + ",output=" + p
			liveMediaItems = append(liveMediaItems, structs.MediaItem{
				Type: media[i].Type,
				Url: p,
			})
			if (media[i].Type == "audio") {
				p2 := matchPath[0][1] + ".m3u8" 
				args = args + ",playlist_name=" + p2
				liveMediaItems = append(liveMediaItems, structs.MediaItem{
					Type: "hls-playlist",
					Url: p2,
				})
				args = args + ",hls_group_id=audio,hls_name=ENGLISH"
			} else {
				p2 := matchPath[0][1] + "-" + media[i].Type + ".m3u8"
				args = args + ",playlist_name=" +  p2
				liveMediaItems = append(liveMediaItems, structs.MediaItem{
					Type: "hls-playlist",
					Url: p2,
				})
				p3 :=  matchPath[0][1] + "-" + media[i].Type + "_iframe.m3u8"
				args = args + ",iframe_playlist_name=" + p3
				liveMediaItems = append(liveMediaItems, structs.MediaItem{
					Type: "iframe-hls-playlist",
					Url: p3,
				})
			}
		}
		argsSlice = append(argsSlice, args)
	}
	var expectedMpdPath string = vid.GetID() + "-mpd.mpd"
	liveMediaItems = append(liveMediaItems, structs.MediaItem{
		Type: "mpd",
		Url: expectedMpdPath,
	})
	var expectedHlsPath string = vid.GetID() + "-hls.m3u8"
	liveMediaItems = append(liveMediaItems, structs.MediaItem{
		Type: "hls",
		Url: expectedHlsPath,
	})
	argsSlice = append(argsSlice, "--mpd_output")
	argsSlice = append(argsSlice, expectedMpdPath)
	argsSlice = append(argsSlice, "--hls_master_playlist_output")
	argsSlice = append(argsSlice, expectedHlsPath)
	// fmt.Printf("%v %v", app, argsSlice) // Packager command
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
		fmt.Printf("Error with packager: %v", err)
	}
	return liveMediaItems, media; // liveData, old (deleted if del true)
}

func UpdateMongoRecord(vid *vpb.Video, media []structs.MediaItem, status string, thumbtrack []structs.Thumbnail) (any, error) {
	if client == nil {
		return nil, fmt.Errorf("Database client connection unavailable %v", err)
	}
	videos := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("videos")
	record := &structs.Video{}
	err := videos.FindOne(context.TODO(), bson.D{{"_id", vid.GetID()}}).Decode(&record) // Get from phone number data
	if err == mongo.ErrNoDocuments {
		document := structs.Video{
			ID: 		vid.GetID(),
			Author:		vid.GetSocket(),
			Status:		status,
			Publish:	-1,
			Creation:	int(time.Now().UnixNano() / 1000000),
			Mpd:		"",
			Hls:		"",
			Media:		media,
			Thumbnail:	"",
			Thumbtrack:	[]structs.Thumbnail{},
			Title:		"",
			Description:"",
			Tags:		make([]interface{}, 0),
			Production: "",
			Cast:		make([]interface{}, 0),
			Directors:	make([]interface{}, 0),
			Writers:	make([]interface{}, 0),
		}
		insertedDocument, err2 := videos.InsertOne(context.TODO(), document)
		if err2 != nil {
			return nil, err2
		}
		return insertedDocument, nil
	} else {
		trimmedMediaData := media
		m, err := CleanUpStrayData(media)
		if err == nil {
			trimmedMediaData = m
		}
		document := structs.Video{ // Upsert document
			ID: 		vid.GetID(),
			Author:		vid.GetSocket(),
			Status:		status,
			Publish:	-1,
			Creation:	record.Creation,
			Mpd:		FindMediaOfType(media, "mpd"),
			Hls:		FindMediaOfType(media, "hls"),
			Media:		trimmedMediaData,
			Thumbnail:	FindMediaOfType(media, "thumbnail"),
			Thumbtrack:	thumbtrack,
			Title:		record.Title,
			Description:record.Description,
			Tags:		record.Tags,
			Production: record.Production,
			Cast:		record.Cast,
			Directors:	record.Directors,
			Writers:	record.Writers,
		}
		opts := options.FindOneAndUpdate().SetUpsert(true)
		opts = options.FindOneAndUpdate().SetReturnDocument(options.After)
		newDoc, _ := toDoc(document)
		var v structs.Video
		videos.FindOneAndUpdate(
			context.TODO(),
			bson.D{{ "_id", vid.GetID()}},
			bson.M{ "$set": newDoc },
			opts).Decode(&v)
		return v, nil
	}
	return nil, nil
}

func FindDefaultThumbnail(thumbtrack []structs.Thumbnail, media []structs.MediaItem) []structs.MediaItem {
	if len(thumbtrack) != 0 {
		for i := 5; i > 0; i-- {
			if len(thumbtrack) > i {
				media = append(media, structs.MediaItem{
					Type: "thumbnail",
					Url: thumbtrack[i].Url,
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
				AccessKeyID: s3credentials.GetS3Data("awsConfig", "accessKeyId", ""), 
				SecretAccessKey: s3credentials.GetS3Data("awsConfig", "secretAccessKey", ""),
			},
		}),
	)
	if err != nil {
		return err
	}
	client := s3.NewFromConfig(cfg)
	uploader := manager.NewUploader(client)
	for i := 0; i < len(liveMediaItems); i++ {
		upFrom := destination
		upTo := uploadFolder
		if liveMediaItems[i].Type == "thumbnail" {
			upFrom = thumbDir
			upTo = "thumbnail/"
		}
		f, _ := os.Open(upFrom + liveMediaItems[i].Url)
		r := bufio.NewReader(f)
		fmt.Printf("Uploading: %v\n", upFrom + liveMediaItems[i].Url)
		_, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(s3credentials.GetS3Data("awsConfig", "buckets", "tycoon-systems-video1")),
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
				AccessKeyID: s3credentials.GetS3Data("awsConfig", "accessKeyId", ""), 
				SecretAccessKey: s3credentials.GetS3Data("awsConfig", "secretAccessKey", ""),
			},
		}),
	)
	if err != nil {
		return err
	}
	client := s3.NewFromConfig(cfg)
	uploader := manager.NewUploader(client)
	for i := 0; i < len(thumbtrack); i++ {
		f, _ := os.Open(destination + thumbtrack[i].Url)
		r := bufio.NewReader(f)
		fmt.Printf("Uploading: %v\n", uploadFolder + thumbtrack[i].Url)
		_, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(s3credentials.GetS3Data("awsConfig", "buckets", "tycoon-systems-video1")),
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
	for i := len(media) -1; i > 0; i-- {
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
		Output(thumbDir + "/" + vid.GetID() + "-thumb%03d.jpg", ffmpeg.KwArgs{
			"vf": "select='not(mod(n,600))',setpts='N/(30*TB)',scale=-2:180",
			"f": "image2",
		}).
		ErrorToStdOut().
		Run()
	if err != nil {
		fmt.Printf("Err %v", err)
		return []structs.Thumbnail{}, ""
	}
	files, err := ioutil.ReadDir(thumbDir)
	if err != nil {
		fmt.Printf("Err %v", err)
		OrganizeAndDeleteThumbnails(thumbtrack, files, vid.GetPath())
		return []structs.Thumbnail{}, ""
	}
	data, err := ffmpeg.Probe(vid.GetPath())
	if err != nil {
		OrganizeAndDeleteThumbnails(thumbtrack, files, vid.GetPath())
		return []structs.Thumbnail{}, ""
	}
	unstructuredData := make(map[string]interface{})
	json.Unmarshal([]byte(data), &unstructuredData)
	fmt.Printf("Unstructured Data %v", unstructuredData)
	if _, ok := unstructuredData["streams"]; ok {
		if _, ok2 := unstructuredData["streams"].([]interface{}); ok2 {
			if _, ok3 := unstructuredData["streams"].([]interface{})[0].(map[string]interface{})["duration"]; ok3 {
				duration := unstructuredData["streams"].([]interface{})[0].(map[string]interface{})["duration"].(string)
				dur, _ := strconv.ParseFloat(duration, 64)
				var ti float64 = 0
				for i, file := range files {
					if ti >= dur || i >= len(files) { // Double check to ensure no panic
						break
					}
					t := structs.Thumbnail{
						Time: fmt.Sprintf("%.2f", ti),
						Url: file.Name(),
					}
					thumbtrack = append(thumbtrack, t)
					ti = ti + 20
				}
				return thumbtrack, thumbDir + "/"
			}
		}
	}
	OrganizeAndDeleteThumbnails(thumbtrack, files, vid.GetPath())
	return []structs.Thumbnail{}, ""
}

func ScheduleProfanityCheck(vid *vpb.Video) error {

	return nil
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
				Url: files[i].Name(),
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
		DeleteFile(stale[i].Url, dir)
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
			Width int
			Height int
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