package ad_queue

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"io"
	"os"

	"github.com/hibiken/asynq"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"tycoon.systems/tycoon-services/s3credentials"
	"tycoon.systems/tycoon-services/structs"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	// "strings"

	"encoding/xml"
	"log"
	"net/http"
	"net/url"

	"time"

	"google.golang.org/grpc"
	adpb "tycoon.systems/tycoon-services/ad"

	"github.com/go-redis/redis/v8"

	"github.com/stripe/stripe-go"
	stripe73 "github.com/stripe/stripe-go/v73"
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
	client, err                         = mongo.Connect(context.TODO(), clientOpts)
	jobQueueAddr                        = s3credentials.GetS3Data("redis", "redishost", "") + ":" + s3credentials.GetS3Data("redis", "tycoon_systems_ad_queue_port", "")
	adAnalyticsAddr                     = s3credentials.GetS3Data("redis", "redishost", "") + ":" + s3credentials.GetS3Data("redis", "tycoon_systems_ad_analytics_port", "")
	jobClient                           = asynq.NewClient(asynq.RedisClientOpt{Addr: jobQueueAddr})
	returnJobResultPort                 = s3credentials.GetS3Data("app", "services", "adServer")
	returnJobResultAddr                 = s3credentials.GetS3Data("app", "prodhost", "")
	vastUploadFolderPath                = "../tycoon-services-vast-ad-generation/"
	s3VideoEndpoint                     = s3credentials.GetS3Data("awsConfig", "buckets", "tycoon-systems-video")
	s3VideoAdEndpoint                   = s3credentials.GetS3Data("awsConfig", "buckets", "tycoon-systems-ads")
	devEnv                              = s3credentials.GetS3Data("app", "dev", "")
	adServerEndPoint                    = s3credentials.GetS3Data("app", "server", "")
	adServerPort                        = s3credentials.GetS3Data("app", "adServerPort", "")
	videoCdn                            = s3credentials.GetS3Data("prod", "tycoonSystemsVideo1", "")
	adVideoCdn                          = s3credentials.GetS3Data("prod", "tycoonSystemsAds1", "")
	tycoonSystemsAdAnalyticsRedisClient = redis.NewClient(&redis.Options{
		Addr:     adAnalyticsAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	stripeKey   = s3credentials.GetS3Data("stripe", "key", "")
	viewsPrice  = s3credentials.GetS3Data("stripe", "analyticsLive", "views_price")
	clicksPrice = s3credentials.GetS3Data("stripe", "analyticsLive", "clicks_price")
)

const (
	TypeVastGenerate           = "vast:generate"
	_vastAlreadyGeneratedCheck = "Vast Video has already been generated"
)

func main() {

}

func ProvisionCreateNewVastCompliantAdVideoJob(vast structs.VastTag) string {
	if reflect.TypeOf(vast.ID).Kind() == reflect.String {
		task, err := NewCreateVastCompliantAdVideoTask(vast)
		if err != nil {
			log.Printf("Could not create Vast Generate task at Task Creation: %v", err)
		}
		info, err := jobClient.Enqueue(task)
		if err != nil {
			log.Printf("Could not create Vast Generate task at Enqueue: %v", err)
		}
		log.Printf("Enqueued Ad Creation task")
		return info.ID
	}
	return "failed"
}

func GetConnection() *mongo.Client {
	return client
}

// Build new delivery to be consumed by queue
func NewCreateVastCompliantAdVideoTask(vast structs.VastTag) (*asynq.Task, error) {
	payload, err := json.Marshal(vast)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeVastGenerate, payload), nil
}

// Unmarshal queued delivery task to determine if in correct format
func HandleCreateVastCompliantAdVideoTask(ctx context.Context, t *asynq.Task) error {
	var p structs.VastTag
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	log.Printf("Beginning VastTag Generation for ID: %v, Socket: %v", p.ID, p.Socket)
	err := PerformVastCompliantAdVideoGeneration(p)
	if err != nil {
		return fmt.Errorf("Perform VastTag Gemeration failed: %v: %w", err, asynq.SkipRetry)
	}
	return nil
}

func PerformVastCompliantAdVideoGeneration(vast structs.VastTag) error {
	fmt.Printf("Video:  %v, %v, %v, %v, %v, %v, %v, %v, %v, %v, %v\n", vast.ID, vast.DocumentId, vast.Socket, vast.Status, vast.Url, vast.TrackingUrl, vast.AdTitle, vast.ClickthroughUrl, vast.StartTime, vast.EndTime, vast.PlayTime)

	// get location of highest resolution file 1080 -> 720 -> 540 -> 360. If nothing higher than 360 fail
	videos := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("videos")
	adUnits := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("adunits")
	record := &structs.Video{}
	err := videos.FindOne(context.TODO(), bson.D{{"_id", vast.DocumentId}}).Decode(&record) // Get from video data
	if err != nil {
		log.Printf("Get Video Failed %v", err)
		return fmt.Errorf("Get Video Failed")
	}
	fmt.Printf("Media %v", record.Media)
	configResolutions := make([]string, 0)
	configResolutions = append(configResolutions, "2048", "1440", "720", "540", "360", "240")
	var highestResVideo string
	var adAudio string
	for i := 0; i < len(configResolutions); i++ {
		if len(highestResVideo) == 0 {
			for j := 0; j < len(record.Media); j++ {
				f, _ := regexp.MatchString(configResolutions[i], record.Media[j].Url)
				if f {
					highestResVideo = record.Media[j].Url
					break
				}
			}
		}
	}
	for i := 0; i < len(record.Media); i++ {
		f, _ := regexp.MatchString("audio", record.Media[i].Url)
		if f {
			adAudio = record.Media[i].Url
			break
		}
	}
	log.Printf("Highest Res Video Found %v Audio Found %v", highestResVideo, adAudio)

	// download video and audio to local from s3
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
		log.Printf("err %v", err)
		return err
	}
	client := s3.NewFromConfig(cfg)
	downloader := manager.NewDownloader(client)
	if devEnv == "true" {
		s3VideoEndpoint = s3credentials.GetS3Data("awsConfig", "devBuckets", "tycoon-systems-video-development")
		s3VideoAdEndpoint = s3credentials.GetS3Data("awsConfig", "devBuckets", "tycoon-systems-ads-development")
	}
	v1Endpoint := "video/" + highestResVideo
	a1Endpoint := "video/" + adAudio
	v1Input := s3.HeadObjectInput{
		Bucket: aws.String(s3VideoEndpoint),
		Key:    aws.String(v1Endpoint),
	}
	a1Input := s3.HeadObjectInput{
		Bucket: aws.String(s3VideoEndpoint),
		Key:    aws.String(a1Endpoint),
	}
	v1InputResult, err := client.HeadObject(context.TODO(), &v1Input)
	if err != nil {
		return err
	}
	a1InputResult, err := client.HeadObject(context.TODO(), &a1Input)
	if err != nil {
		log.Printf("err %v %v %v %v %v", err, s3VideoAdEndpoint, s3VideoEndpoint, v1Input, v1Endpoint)
		return err
	}
	vBuf := make([]byte, int(v1InputResult.ContentLength))
	aBuf := make([]byte, int(a1InputResult.ContentLength))
	w := manager.NewWriteAtBuffer(vBuf)
	w2 := manager.NewWriteAtBuffer(aBuf)
	_, err = downloader.Download(context.TODO(), w, &s3.GetObjectInput{
		Bucket: aws.String(s3VideoEndpoint),
		Key:    aws.String(v1Endpoint),
	})
	_, err = downloader.Download(context.TODO(), w2, &s3.GetObjectInput{
		Bucket: aws.String(s3VideoEndpoint),
		Key:    aws.String(a1Endpoint),
	})
	err = ioutil.WriteFile(vastUploadFolderPath+vast.ID+"-720-raw.mp4", vBuf, 0644)
	if err != nil {
		log.Printf("err %v", err)
		return err
	}
	err = ioutil.WriteFile(vastUploadFolderPath+vast.ID+"-audio-raw.mp4", aBuf, 0644)
	if err != nil {
		log.Printf("err %v", err)
		return err
	}

	// generate video with "packed" audio based on starttime and endtime using ffmpeg
	err = ffmpeg.Input(vastUploadFolderPath+vast.ID+"-720-raw.mp4", ffmpeg.KwArgs{
		"i": vastUploadFolderPath + vast.ID + "-audio-raw.mp4",
	}).
		Output(vastUploadFolderPath+vast.ID+"-720-paudio.mp4", ffmpeg.KwArgs{
			"ss":          vast.StartTime,
			"t":           vast.PlayTime,
			"c":           "copy",
			"shortest":    "",
			"vf":          "scale=" + "-2:" + "720", // Sets scaled resolution with same ratios
			"c:v":         "libx264",                // Set video codec
			"c:a":         "aac",
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
		fmt.Printf("Issue with transcoding Video %v\nPath: %v\nAudio Path: %v\nOutput: %v\n", err, vastUploadFolderPath+vast.ID+"-720-raw.mp4", vastUploadFolderPath+vast.ID+"-720-audio.mp4", vastUploadFolderPath+vast.ID+"-720-audio.mp4")
		return err
	}
	fmt.Printf("Done FFmpeg Job")

	// Remove old media
	oldAdRecord := &structs.AdUnit{}
	err = adUnits.FindOne(context.TODO(), bson.D{{"_id", vast.ID}}).Decode(&oldAdRecord) // Get from video data
	if len(oldAdRecord.Media) > 0 {
		for i := 0; i < len(oldAdRecord.Media); i++ {
			client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
				Bucket: aws.String(s3VideoAdEndpoint),
				Key:    aws.String("video/" + oldAdRecord.Media[i]),
			})
		}
	}

	// upload to tycoon-systems-ads/video or tycoon-systems-ads-development/video
	uploader := manager.NewUploader(client)

	f, _ := os.Open(vastUploadFolderPath + vast.ID + "-720-paudio.mp4")
	r := bufio.NewReader(f)
	fmt.Printf("Uploading: %v\n", vastUploadFolderPath+vast.ID+"-720-paudio.mp4")
	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s3VideoAdEndpoint),
		Key:    aws.String("video/" + vast.ID + "-720-paudio.mp4"),
		Body:   r,
	})
	if err != nil {
		return err
	}
	f.Close()

	// store record on mongodb
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var adUnit structs.AdUnit
	duration := structs.Duration{
		StartTime: vast.StartTime,
		EndTime:   vast.EndTime,
		PlayTime:  vast.PlayTime,
	}
	mediaItems := []string{}
	mediaItems = append(mediaItems, vast.ID+"-720-paudio.mp4")
	err = adUnits.FindOneAndUpdate(
		context.TODO(),
		bson.D{{"_id", vast.ID}},
		bson.M{"$set": bson.M{"media": mediaItems, "duration": duration}},
		opts).
		Decode(&adUnit)
	if err != nil {
		log.Printf("Update Mongo Ad Unit Failed %v", err)
		return err
	}
	// delete local files
	DeleteFile(vast.ID+"-720-raw.mp4", vastUploadFolderPath)
	DeleteFile(vast.ID+"-audio-raw.mp4", vastUploadFolderPath)
	DeleteFile(vast.ID+"-720-paudio.mp4", vastUploadFolderPath)
	// advise node.js server job is done, ad is in review
	returnFinishedAdCreativeCreationJobReport(vast, "Success transcoding", "video/")
	return nil
}

type Vmap struct {
	XMLName  xml.Name  `xml:"vmap:VMAP"`
	VmapAttr string    `xml:"xmlns:vmap,attr"`
	Version  string    `xml:"version,attr"`
	AdBreaks []AdBreak `xmlns:"vmap,namespace"`
}

type AdBreak struct {
	XMLName    xml.Name `xml:"vmap:AdBreak"`
	TimeOffset string   `xml:"timeOffset,attr"`
	BreakType  string   `xml:"breakType,attr"`
	BreakId    string   `xml:"breakId,attr"`
	AdSource   AdSource `xmlns:"vmap,namespace"`
}

type AdSource struct {
	XMLName          xml.Name `xml:"vmap:AdSource"`
	Id               string   `xml:"id,attr"`
	AllowMultipleAds string   `xml:"allowMultipleAds,attr"`
	FollowRedirects  string   `xml:"followRedirects,attr"`
	AdTagUri         AdTagUri `xmlns:"vmap,namespace"`
}

type AdTagUri struct {
	XMLName      xml.Name `xml:"vmap:AdTagURI"`
	TemplateType string   `xml:"templateType,attr"`
	Url          string   `xml:",innerxml"`
}

func GetVideoDuration(document string) (string, error) {
	videos := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("videos")
	record := &structs.Video{}
	err := videos.FindOne(context.TODO(), bson.D{{"_id", document}}).Decode(&record) // Get from video data
	if err != nil {
		return "0", fmt.Errorf("Get Video Duration failed")
	}
	return record.Duration, nil
}

func GenerateAndServeVmap(r *http.Request) ([]byte, error) {
	var document string = r.URL.Query().Get("document") // Get document id value
	// var docType string = r.URL.Query().Get("type")      // Get document type
	duration, err := GetVideoDuration(document) // Get duration to determine length of video for ad breaks
	record := &structs.Video{}
	if client != nil {
		videos := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("videos")
		record = &structs.Video{}
		err = videos.FindOne(context.TODO(), bson.D{{"_id", document}}).Decode(&record) // Get from video data
	}
	adExceptions := []string{"do-not-play-ads", "do-not-play-midrolls"}
	foundRolls := 0
	foundAdsAbsolved := false
	for i := 0; i < len(record.Timeline); i++ {
		if _, ok := record.Timeline[i].(structs.TimelineNode); ok {
			field := record.Timeline[i].(structs.TimelineNode).Type
			for j := 0; j < len(adExceptions); j++ {
				if field == adExceptions[j] {
					foundAdsAbsolved = true
				}
			}
			if field == "ad-marker" {
				foundRolls += 1
			}
		}
	}
	// No valid ads on timeline and asking to not absolve from midrolls, create default timeline based on duration
	if foundRolls == 0 && foundAdsAbsolved == false {
		record.Timeline = GenerateMockTimeline(duration) // Generate timeline to iterate through ad points
	}
	validMidRolls := resolveValidAdsMidRolls(r)
	var adBreaks []AdBreak
	protocol := "https://"
	root := adServerEndPoint
	if devEnv == "true" {
		protocol = "http://"
		root = "localhost"
	}
	// Append Pre-roll
	singleAd, n := resolveSingleAd(r, validMidRolls)
	var vastQueryParams string
	if n != -1 {
		vastQueryParams = generateVastQueryParameters(r, singleAd)
		vastQueryParams = generateVmapQueryParameters(vastQueryParams, r, singleAd, true, "preroll")
		adBreaks = append(adBreaks,
			AdBreak{xml.Name{}, "start", "linear", "preroll",
				AdSource{xml.Name{}, "preroll-ad-1", "false", "true",
					AdTagUri{xml.Name{}, "vast3", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/vast" + vastQueryParams + " ]]>"},
				},
			},
		)
	}
	// Append Mid-rolls
	midRoll := 1
	for i := 0; i < len(record.Timeline); i++ {
		if _, ok := record.Timeline[i].(structs.TimelineNode); ok {
			field := record.Timeline[i].(structs.TimelineNode).Type
			time := record.Timeline[i].(structs.TimelineNode).Time
			amount := record.Timeline[i].(structs.TimelineNode).Amount
			validAdsCopy := make([]structs.AdUnit, len(validMidRolls))
			copy(validAdsCopy, validMidRolls)
			if field == "ad-marker" {
				for j := 0; j < amount; j++ {
					midRollNum := j + 1
					useAd, n2 := resolveSingleAd(r, validAdsCopy)
					if n2 != -1 {
						validAdsCopy = removeAdAtIndex(validAdsCopy, n)
						vastQueryParams = generateVastQueryParameters(r, useAd)
						vastQueryParams = generateVmapQueryParameters(vastQueryParams, r, singleAd, true, "midroll")
						adBreaks = append(adBreaks,
							AdBreak{xml.Name{}, resolveSecondsToReadableAdTimeFormat(time), "linear", "midroll-" + strconv.Itoa(midRoll),
								AdSource{xml.Name{}, "midroll-" + strconv.Itoa(midRoll) + "-ad-" + strconv.Itoa(midRollNum), "false", "true",
									AdTagUri{xml.Name{}, "vast3", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/vast" + vastQueryParams + " ]]>"},
								},
							},
						)
					}
				}
				midRoll = midRoll + 1
			}
		}

	}
	// Append Post-roll
	singlePostAd, n3 := resolveSingleAd(r, validMidRolls)
	if n3 != -1 {
		vastQueryParams = generateVastQueryParameters(r, singlePostAd)
		vastQueryParams = generateVmapQueryParameters(vastQueryParams, r, singleAd, true, "postroll")
		adBreaks = append(adBreaks,
			AdBreak{xml.Name{}, "end", "linear", "postroll",
				AdSource{xml.Name{}, "postroll-ad-1", "false", "true",
					AdTagUri{xml.Name{}, "vast3", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/vast" + vastQueryParams + " ]]>"},
				},
			},
		)
	}
	// iterate through adbreaks and ad each midroll (2 midroll ads each)
	response := Vmap{
		xml.Name{},
		"http://www.iab.net/videosuite/vmap",
		"1.0",
		adBreaks,
	}

	x, err := xml.MarshalIndent(response, "", " ")
	if err != nil {
		return nil, err
	}
	return []byte(x), nil
}

type Vast struct {
	XMLName xml.Name `xml:"Vast"`
	// VastAttr                  string   `xml:"xmlns:xsi,attr"`
	// NoNamespaceSchemaLocation string   `xml:"xsi:noNamespaceSchemaLocation,attr"`
	Version   string `xml:"version,attr"`
	AdElement AdElement
}

type AdElement struct {
	XMLName xml.Name `xml:"Ad"`
	IdAttr  string   `xml:"id,attr"`
	InLine  InLineElement
}

type InLineElement struct {
	XMLName     xml.Name `xml:"InLine"`
	AdSystem    string   `xml:"AdSystem"`
	AdTitle     string   `xml:"AdTitle"`
	Description string   `xml:"Description"`
	Error       string   `xml:"Error"`
	Impression  string   `xml:"Impression"`
	Creatives   CreativesElement
}

type CreativesElement struct {
	XMLName           xml.Name `xml:"Creatives"`
	Creative          []CreativeElement
	CreativeCompanion []CreativeCompanionElement
}

type CreativeElement struct {
	XMLName      xml.Name `xml:"Creative"`
	IdAttr       string   `xml:"id,attr"`
	AdIdAttr     string   `xml:"AdID,attr"`
	SequenceAttr string   `xml:"sequence,attr"`
	Linear       LinearElement
}

type CreativeCompanionElement struct {
	XMLName      xml.Name `xml:"Creative"`
	IdAttr       string   `xml:"id,attr"`
	SequenceAttr string   `xml:"sequence,attr"`
	CompanionAds CompanionAdsElement
}

type CompanionAdsElement struct {
	XMLName   xml.Name `xml:"CompanionAds"`
	Companion CompanionElement
}

type CompanionElement struct {
	XMLName        xml.Name `xml:"Companion"`
	IdAttr         string   `xml:"id,attr"`
	WidthAttr      string   `xml:"width,attr"`
	HeightAttr     string   `xml:"height,attr"`
	StaticResource StaticResourceElement
}

type StaticResourceElement struct {
	XMLName      xml.Name `xml:"StaticResource"`
	CreativeType string   `xml:"creativeType,attr"`
	Url          string   `xml:",innerxml"`
}

type LinearElement struct {
	XMLName        xml.Name `xml:"Linear"`
	Duration       string   `xml:"Duration"`
	TrackingEvents TrackingEventsElement
	VideoClicks    VideoClicksElement
	MediaFiles     MediaFilesElement
}

type TrackingEventsElement struct {
	XMLName       xml.Name `xml:"TrackingEvents"`
	TrackingEvent []TrackingEventElement
}

type TrackingEventElement struct {
	XMLName   xml.Name `xml:"Tracking"`
	EventAttr string   `xml:"event,attr"`
	Url       string   `xml:",innerxml"`
}

type VideoClicksElement struct {
	XMLName      xml.Name `xml:"VideoClicks"`
	ClickThrough ClickThroughElement
}

type ClickThroughElement struct {
	XMLName xml.Name `xml:"ClickThrough"`
	IdAttr  string   `xml:"id,attr"`
	Url     string   `xml:",innerxml"`
}

type MediaFilesElement struct {
	XMLName   xml.Name `xml:"MediaFiles"`
	MediaFile []MediaFileElement
}

type MediaFileElement struct {
	XMLName                 xml.Name `xml:"MediaFile"`
	IdAttr                  string   `xml:"id,attr"`
	DeliveryAttr            string   `xml:"delivery,attr"`
	WidthAttr               string   `xml:"width,attr"`
	HeightAttr              string   `xml:"height,attr"`
	TypeAttr                string   `xml:"type,attr"`
	MinBitrateAttr          string   `xml:"minBitrate,attr"`
	MaxBitrateAttr          string   `xml:"maxBitrate,attr"`
	ScalableAttr            string   `xml:"scalable,attr"`
	MaintainAspectRatioAttr string   `xml:"maintainAspectRatio,attr"`
	Url                     string   `xml:",innerxml"`
}

func GenerateAndServeVast(r *http.Request) ([]byte, error) {
	id := r.URL.Query().Get("id")
	protocol := "https://"
	root := adServerEndPoint
	if devEnv == "true" {
		protocol = "http://"
		root = "localhost"
		adVideoCdn = s3credentials.GetS3Data("dev", "tycoonSystemsAds1", "")
	}
	record := &structs.AdUnit{}
	if client != nil {
		adUnits := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("adunits")
		adUnits.FindOne(context.TODO(), bson.D{{"_id", id}}).Decode(&record)
	}
	var duration structs.Duration = record.Duration
	vastQueryParams := generateVastQueryParameters(r, *record)
	escapedQueryParams := strings.ReplaceAll(vastQueryParams, "&", "&amp;")
	response := Vast{
		xml.Name{},
		"3.0",
		AdElement{
			xml.Name{},
			id,
			InLineElement{
				xml.Name{},
				"Tycoon",
				record.AdTitle,
				"<![CDATA[ " + record.AdDescription + " ]]>",
				protocol + root + ":" + adServerPort + "/ads/error" + vastQueryParams + "&videoplayfailed=[ERRORCODE]&placebo=0",
				protocol + root + ":" + adServerPort + "/ads/view" + vastQueryParams,
				CreativesElement{
					xml.Name{},
					[]CreativeElement{
						{
							xml.Name{},
							record.DocumentId,
							record.ID,
							"1",
							LinearElement{
								xml.Name{},
								duration.PlayTime,
								TrackingEventsElement{
									xml.Name{},
									[]TrackingEventElement{
										{xml.Name{}, "start", protocol + root + ":" + adServerPort + "/ads/track" + escapedQueryParams + "&amp;event=start"},
										{xml.Name{}, "firstQuartile", protocol + root + ":" + adServerPort + "/ads/track" + escapedQueryParams + "&amp;event=firstquartile"},
										{xml.Name{}, "midpoint", protocol + root + ":" + adServerPort + "/ads/track" + escapedQueryParams + "&amp;event=midpoint"},
										{xml.Name{}, "thirdQuartile", protocol + root + ":" + adServerPort + "/ads/track" + escapedQueryParams + "&amp;event=thirdquartile"},
										{xml.Name{}, "complete", protocol + root + ":" + adServerPort + "/ads/track" + escapedQueryParams + "&amp;event=complete"},
										{xml.Name{}, "pause", protocol + root + ":" + adServerPort + "/ads/track" + escapedQueryParams + "&amp;event=pause"},
										{xml.Name{}, "mute", protocol + root + ":" + adServerPort + "/ads/track" + escapedQueryParams + "&amp;event=mute"},
										{xml.Name{}, "fullscreen", protocol + root + ":" + adServerPort + "/ads/track" + escapedQueryParams + "&amp;event=fullscreen"},
										{xml.Name{}, "acceptInvitationLinear", protocol + root + ":" + adServerPort + "/ads/track" + escapedQueryParams + "&amp;event=acceptinvitationlinear"},
										{xml.Name{}, "closeLinear", protocol + root + ":" + adServerPort + "/ads/track" + escapedQueryParams + "&amp;event=closelinear"},
										{xml.Name{}, "click", protocol + root + ":" + adServerPort + "/ads/track" + escapedQueryParams + "&amp;event=click"},
									},
								},
								VideoClicksElement{
									xml.Name{},
									ClickThroughElement{xml.Name{}, "Tycoon", "<![CDATA[ " + record.ClickthroughUrl + " ]]>"},
								},
								MediaFilesElement{
									xml.Name{},
									[]MediaFileElement{
										// {xml.Name{}, "Tycoon", "progressive", "426", "240", "video/mp4", "49", "258", "true", "true", "<![CDATA[ https://d2wj75ner94uq0.cloudfront.net/video/63fc01dc50af45988feac2208ff087ac-720.mp4 ]]>"},
										// {xml.Name{}, "Tycoon", "streaming", "426", "240", "application/x-mpegURL", "49", "258", "true", "true", "<![CDATA[ " + videoCdn + "/video/" + playRecord.Hls + " ]]>"},
										// {xml.Name{}, "Tycoon", "streaming", "426", "240", "application/dash+xml", "49", "258", "true", "true", "<![CDATA[ " + videoCdn + "/video/" + playRecord.Mpd + " ]]>"},
										{xml.Name{}, "Tycoon", "progressive", "960", "720", "video/mp4", "49", "258", "true", "true", "<![CDATA[ " + adVideoCdn + "/video/" + record.Media[0] + " ]]>"},
									},
								},
							},
						},
					},
					[]CreativeCompanionElement{
						{
							xml.Name{},
							record.DocumentId,
							"1",
							CompanionAdsElement{
								xml.Name{},
								CompanionElement{
									xml.Name{},
									record.DocumentId,
									"300",
									"250",
									StaticResourceElement{
										xml.Name{},
										"image/png",
										"<![CDATA[ https://www.tycoon.systems/ ]]>",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	x, err := xml.MarshalIndent(response, "", " ")
	if err != nil {
		return nil, err
	}
	return []byte(x), nil
}

func GenerateMockTimeline(duration string) []interface{} {
	minutes := 5
	durationInt, _ := strconv.Atoi(duration)
	requiredRolls := (durationInt / 60) / minutes
	curTime := 300 // 5 minutes start time, dont start at pre roll
	timeline := make([]interface{}, requiredRolls)
	for i := 0; i < requiredRolls; i++ {
		timeline = append(timeline, structs.TimelineNode{
			Type:   "ad-marker",
			Amount: 2,
			Time:   curTime,
		})
		curTime += 300
	}
	return timeline
}

func UpdateRecord(url string, id string) (bool, error) {
	if client == nil {
		return false, fmt.Errorf("No client to Update Record")
	}
	adUnits := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("adunits")
	record := &structs.VastTag{}
	err := adUnits.FindOne(context.TODO(), bson.D{{"_id", id}}).Decode(&record)
	if err == mongo.ErrNoDocuments {
		return false, fmt.Errorf("No record of existing adunit for vast tag")
	} else {
		opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
		var v structs.AdUnit
		adUnits.FindOneAndUpdate(
			context.TODO(),
			bson.D{{"_id", id}},
			bson.M{"$set": bson.M{"vast": url}},
			opts).
			Decode(&v)
		return true, nil
	}
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

func resolveSecondsToReadableAdTimeFormat(duration int) string {
	var hours int = duration / 3600
	var minutes int = (duration - (hours * 3600)) / 60
	var seconds int = (duration - ((hours * 3600) + (minutes * 60)))
	hoursPadded := fmt.Sprintf("%02s", strconv.Itoa(hours))
	minutesPadded := fmt.Sprintf("%02s", strconv.Itoa(minutes))
	secondsPadded := fmt.Sprintf("%02s", strconv.Itoa(seconds))
	return hoursPadded + ":" + minutesPadded + ":" + secondsPadded
}

/**
Find All valid ads from ad units table
*/
func resolveValidAdsMidRolls(r *http.Request) []structs.AdUnit {
	var records []structs.AdUnit
	if client != nil {
		adUnits := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("adunits")
		cursor, err := adUnits.Find(context.TODO(), bson.D{{"status", "live"}, {"publish", true}}) // Get from video data
		if err != nil {
			fmt.Printf("Err %v", err)
			return records
		} else {
			for cursor.Next(context.TODO()) {
				var result structs.AdUnit
				if err := cursor.Decode(&result); err != nil {
					fmt.Printf("Error %v", err)
					return records
				}
				currentTime := time.Now()
				totalRecordDoc := result.ID + "-" + "total"
				currentRecordDoc := result.ID + "-" + currentTime.Format("2006-01-02")
				existsViews := tycoonSystemsAdAnalyticsRedisClient.HGet(context.TODO(), totalRecordDoc, "view")
				existsClicks := tycoonSystemsAdAnalyticsRedisClient.HGet(context.TODO(), totalRecordDoc, "click")
				todaysViews := tycoonSystemsAdAnalyticsRedisClient.HGet(context.TODO(), currentRecordDoc, "view")
				todaysClicks := tycoonSystemsAdAnalyticsRedisClient.HGet(context.TODO(), currentRecordDoc, "click")
				if existsViews.Val() == "" || existsClicks.Val() == "" || todaysViews.Val() == "" || todaysClicks.Val() == "" {
					records = append(records, result)
				} else {
					totalViewsFloat, err := strconv.ParseFloat(existsViews.Val(), 64)
					totalClicksFloat, err2 := strconv.ParseFloat(existsClicks.Val(), 64)
					todaysViewsFloat, err3 := strconv.ParseFloat(todaysViews.Val(), 64)
					todaysClicksFloat, err4 := strconv.ParseFloat(todaysClicks.Val(), 64)
					if err != nil || err2 != nil || err3 != nil || err4 != nil {
						log.Printf("err %v, err2 %v, err3 %v, err4 %v", err, err2, err3, err4)
						records = append(records, result)
					} else {
						viewCost := 0.00719
						clickCost := 0.35
						viewsBudget := totalViewsFloat * viewCost
						clicksBudget := totalClicksFloat * clickCost
						todaysViewsBudget := todaysViewsFloat * viewCost
						todaysClicksBudget := todaysClicksFloat * clickCost
						maxCampaignBudgetFloat := float64(result.MaxCampaignBudget)
						dailyCampaignBudgetFloat := float64(result.DailyBudget)
						if viewsBudget+clicksBudget < maxCampaignBudgetFloat && todaysViewsBudget+todaysClicksBudget < dailyCampaignBudgetFloat {
							log.Printf("Within Budget. Total Views + Clicks %v Max Campaign Budget %v Todays Views + Clicks %v Daily Campaign Budget %v. Views %v, Clicks %v, todaysViews %v, todaysClicks %v", viewsBudget+clicksBudget, maxCampaignBudgetFloat, todaysViewsBudget+todaysClicksBudget, dailyCampaignBudgetFloat, viewsBudget, clicksBudget, todaysViewsBudget, todaysClicksBudget)
							records = append(records, result) // within budget, serve ad
						}
					}
				}
			}
		}
	}
	return records
}

func resolveSingleAd(r *http.Request, adUnits []structs.AdUnit) (structs.AdUnit, int) {
	if len(adUnits) > 0 {
		var random int = rand.Intn(len(adUnits))
		if reflect.TypeOf(adUnits[random]).Kind() == reflect.TypeOf(structs.AdUnit{}).Kind() {
			return adUnits[random], random
		}
	}
	return structs.AdUnit{}, -1
}

func generateVastQueryParameters(r *http.Request, adUnit structs.AdUnit) string {
	params := "?"
	params = params + "id=" + adUnit.ID
	params = params + "&type=" + adUnit.AdType
	params = params + "&document=" + adUnit.DocumentId
	return params
}

func generateVmapQueryParameters(params string, r *http.Request, adUnit structs.AdUnit, linear bool, vpos string) string {
	if linear == true {
		params = params + "&vad_type=linear"
	}
	if vpos != "" {
		params = params + "&vpos=" + vpos
	}
	return params
}

func removeAdAtIndex(a []structs.AdUnit, i int) []structs.AdUnit {
	t := make([]structs.AdUnit, 0)
	t = append(t, a[:i]...)
	return append(t, a[i+1:]...)
}

func returnFinishedAdCreativeCreationJobReport(vast structs.VastTag, status string, destination string) {
	useReturnJobResultAddr := returnJobResultAddr
	if os.Getenv("dev") == "true" {
		useReturnJobResultAddr = "localhost"
	}
	conn, err := grpc.Dial(useReturnJobResultAddr+":"+returnJobResultPort, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		fmt.Printf("Err: %v", err)
	}
	if err == nil {
		defer conn.Close()
		c := adpb.NewAdManagementClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		c.ReturnVastJobResult(ctx, &adpb.Vast{ID: vast.ID, DocumentId: vast.DocumentId, Status: status, Socket: vast.Socket, Destination: destination, Filename: "", Path: ""})
	}
}

func RecordView(r *http.Request) {
	id := r.URL.Query().Get("id")
	documentType := r.URL.Query().Get("type")
	document := r.URL.Query().Get("document")
	fmt.Printf("Record View: %v, %v, %v\n", id, documentType, document)
	currentTime := time.Now()
	currentRecordDoc := id + "-" + currentTime.Format("2006-01-02")
	totalRecordDoc := id + "-" + "total"
	exists := tycoonSystemsAdAnalyticsRedisClient.Exists(context.TODO(), currentRecordDoc, "view")
	totalExists := tycoonSystemsAdAnalyticsRedisClient.Exists(context.TODO(), totalRecordDoc, "view")
	fmt.Printf("Exists %v", exists.Val())
	if exists.Val() == 0 {
		tycoonSystemsAdAnalyticsRedisClient.HSet(context.TODO(), currentRecordDoc,
			"view", 1,
			"start", 0,
			"firstQuartile", 0,
			"midpoint", 0,
			"thirdQuartile", 0,
			"complete", 0,
			"pause", 0,
			"mute", 0,
			"fullscreen", 0,
			"acceptInvitationLinear", 0,
			"closeLinear", 0,
			"click", 0,
			"paid", 0,
		)
	} else {
		tycoonSystemsAdAnalyticsRedisClient.HIncrBy(context.TODO(), currentRecordDoc, "view", 1)
	}
	if totalExists.Val() == 0 {
		tycoonSystemsAdAnalyticsRedisClient.HSet(context.TODO(), totalRecordDoc,
			"view", 1,
			"start", 0,
			"firstQuartile", 0,
			"midpoint", 0,
			"thirdQuartile", 0,
			"complete", 0,
			"pause", 0,
			"mute", 0,
			"fullscreen", 0,
			"acceptInvitationLinear", 0,
			"closeLinear", 0,
			"click", 0,
		)
	} else {
		tycoonSystemsAdAnalyticsRedisClient.HIncrBy(context.TODO(), totalRecordDoc, "view", 1)
	}
}

func HandleTrackRequest(r *http.Request) {
	id := r.URL.Query().Get("id")
	documentType := r.URL.Query().Get("type")
	document := r.URL.Query().Get("document")
	event := r.URL.Query().Get("event")
	fmt.Printf("Record Event: %v, %v, %v, %v\n", event, id, documentType, document)
	currentTime := time.Now()
	currentRecordDoc := id + "-" + currentTime.Format("2006-01-02")
	totalRecordDoc := id + "-" + "total"
	exists := tycoonSystemsAdAnalyticsRedisClient.Exists(context.TODO(), currentRecordDoc, "view")
	totalExists := tycoonSystemsAdAnalyticsRedisClient.Exists(context.TODO(), totalRecordDoc, "view")
	fmt.Printf("Exists %v Record %v\n", exists.Val(), currentRecordDoc)
	if exists.Val() == 0 {
		tycoonSystemsAdAnalyticsRedisClient.HSet(context.TODO(), currentRecordDoc,
			"view", resolveInc("view", event, 1),
			"start", resolveInc("start", event, 1),
			"firstQuartile", resolveInc("firstQuartile", event, 1),
			"midpoint", resolveInc("midpoint", event, 1),
			"thirdQuartile", resolveInc("thirdQuartile", event, 1),
			"complete", resolveInc("complete", event, 1),
			"pause", resolveInc("pause", event, 1),
			"mute", resolveInc("mute", event, 1),
			"fullscreen", resolveInc("fullscreen", event, 1),
			"acceptInvitationLinear", resolveInc("acceptInvitationLinear", event, 1),
			"closeLinear", resolveInc("closeLinear", event, 1),
			"click", resolveInc("click", event, 1),
			"paid", 0,
		)
	} else {
		tycoonSystemsAdAnalyticsRedisClient.HIncrBy(context.TODO(), currentRecordDoc, event, 1)
	}
	if totalExists.Val() == 0 {
		tycoonSystemsAdAnalyticsRedisClient.HSet(context.TODO(), totalRecordDoc,
			"view", resolveInc("view", event, 1),
			"start", resolveInc("start", event, 1),
			"firstQuartile", resolveInc("firstQuartile", event, 1),
			"midpoint", resolveInc("midpoint", event, 1),
			"thirdQuartile", resolveInc("thirdQuartile", event, 1),
			"complete", resolveInc("complete", event, 1),
			"pause", resolveInc("pause", event, 1),
			"mute", resolveInc("mute", event, 1),
			"fullscreen", resolveInc("fullscreen", event, 1),
			"acceptInvitationLinear", resolveInc("acceptInvitationLinear", event, 1),
			"closeLinear", resolveInc("closeLinear", event, 1),
			"click", resolveInc("click", event, 1),
		)
	} else {
		tycoonSystemsAdAnalyticsRedisClient.HIncrBy(context.TODO(), totalRecordDoc, event, 1)
	}
}

func resolveInc(v string, event string, by int) int {
	if v == event {
		return by
	}
	return 0
}

func AggregateAdAnalytics(currentTime time.Time) {
	adUnits := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("adunits")
	cursor, err := adUnits.Find(context.TODO(), bson.D{}) // Get from video data
	if err != nil {
		fmt.Printf("Err %v", err)
		return
	} else {
		for cursor.Next(context.TODO()) {
			var result structs.AdUnit
			if err := cursor.Decode(&result); err != nil {
				fmt.Printf("Error %v", err)
				continue
			}
			fmt.Printf("Ad Unit CRON Invoicing %v\n", result.ID)
			currentRecordDoc := result.ID + "-" + currentTime.Format("2006-01-02")
			fmt.Printf("Record: %v\n", currentRecordDoc)
			todaysViews := tycoonSystemsAdAnalyticsRedisClient.HGet(context.TODO(), currentRecordDoc, "view")
			todaysClicks := tycoonSystemsAdAnalyticsRedisClient.HGet(context.TODO(), currentRecordDoc, "click")
			fmt.Printf("Todays Clicks %v", todaysClicks.Val())
			var todaysViewsFloat float64 = 0.0
			var todaysClicksFloat float64 = 0.0
			var err2 error
			var err3 error
			if todaysViews.Val() != "" {
				todaysViewsFloat, err2 = strconv.ParseFloat(todaysViews.Val(), 32)
			}
			if todaysClicks.Val() != "" {
				todaysClicksFloat, err3 = strconv.ParseFloat(todaysClicks.Val(), 32)
			}
			if err2 != nil || err3 != nil {
				fmt.Printf("Error getting Views and Clicks for Record %v\n", currentRecordDoc)
				continue
			}
			var viewCost float64 = 0.00719
			var clickCost float64 = 0.35
			todaysViewsFloat64 := float64(todaysViewsFloat)
			todaysClicksFloat64 := float64(todaysClicksFloat)
			todaysViewsBudget := todaysViewsFloat64 * viewCost
			todaysClicksBudget := todaysClicksFloat64 * clickCost
			currentBudgetUse := float64(result.CurrentBudgetUse)
			newCurrentBudgetUse := todaysViewsBudget + todaysClicksBudget + currentBudgetUse
			var newCurrentBudgetUseTruncated float64 = math.Ceil(newCurrentBudgetUse*100) / 100
			var adUnit structs.AdUnit
			opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
			err = adUnits.FindOneAndUpdate(
				context.TODO(),
				bson.D{{"_id", result.ID}},
				bson.M{"$set": bson.M{"currentBudgetUse": newCurrentBudgetUseTruncated}},
				opts).
				Decode(&adUnit)
			if err != nil {
				log.Printf("Update Mongo Ad Unit during CRON Failed %v", err)
				continue
			}
			todaysViewsBudgetString := fmt.Sprintf("%f", todaysViewsBudget)
			todaysClicksBudgetString := fmt.Sprintf("%f", todaysClicksBudget)
			configResolutions := make([]string, 0)
			configResolutions = append(configResolutions, "2048", "1440", "720", "540", "360", "240")
			invoiceItems := make([]structs.InvoiceItem, 0)
			invoiceItems = append(invoiceItems, structs.InvoiceItem{
				Name:    "views",
				Details: "Views for Advertising Services on date " + currentTime.Format("2006-01-02") + "with total " + todaysViews.Val() + " views",
				Data:    "views_total:" + todaysViews.Val() + ";views_cost:" + todaysViewsBudgetString,
				Units:   todaysViewsFloat64,
				Cost:    todaysViewsBudget,
				Note:    "Tycoon Systems Network",
				AdTitle: adUnit.AdTitle,
				AdId:    adUnit.ID,
			}, structs.InvoiceItem{
				Name:    "clicks",
				Details: "Clicks for Advertising Services on date " + currentTime.Format("2006-01-02") + "with total " + todaysClicks.Val() + " clicks",
				Data:    "clicks_total:" + todaysClicks.Val() + ";clicks_cost:" + todaysClicksBudgetString,
				Units:   todaysClicksFloat64,
				Cost:    todaysClicksBudget,
				Note:    "Tycoon Systems Network",
				AdTitle: adUnit.AdTitle,
				AdId:    adUnit.ID,
			})
			total := todaysViewsBudget + todaysClicksBudget
			constructInvoice(invoiceItems, total, result.UserId, currentTime.Format("2006-01-02"))
		}
	}
}

func constructInvoice(invoiceItems []structs.InvoiceItem, total float64, customer string, dateOfInvoice string) {
	fmt.Printf("Invoice Items: %v Total %v", invoiceItems, total)
	invoices := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("invoices")
	foundInvoice := &structs.Invoice{}
	err = invoices.FindOne(context.TODO(), bson.D{{"customer", customer}, {"date", dateOfInvoice}}).Decode(&foundInvoice) // Get from video data
	if err != nil {
		fmt.Printf("Find Invoice Err %v", err)
		invoiceDataJson, _ := json.Marshal(invoiceItems)
		invoice := structs.Invoice{
			ID:              primitive.NewObjectID(),
			StripeId:        "",
			Date:            dateOfInvoice,
			Note:            "Tycoon Systems Advertising Services",
			Customer:        customer,
			CustomerAddress: "",
			CustomerNetwork: "Tycoon Systems Corp.",
			Payee:           "Tycoon Systems Corp.",
			PayeeAddress:    "",
			PayeeNetwork:    "Tycoon Systems Corp.",
			Owed:            total,
			Currency:        "USD",
			Data:            string(invoiceDataJson),
			FootDetails:     "Payment Facilitated on Tycoon Systems Platform",
			Thankyou:        "Thankyou For Your Business",
			Paid:            0.0,
			History:         []string{},
		}
		_, err := invoices.InsertOne(context.TODO(), invoice)
		if err != nil {
			log.Printf("Error Inserting New Invoice Document for Customer %v on %v for total of %v", customer, dateOfInvoice, total)
		}
		chargeInvoice(invoice, invoiceItems, dateOfInvoice, true)
	} else {
		fmt.Printf("Found invoice %v", foundInvoice)
		var existingInvoiceJson []structs.InvoiceItem
		json.Unmarshal([]byte(foundInvoice.Data), existingInvoiceJson)
		for i := 0; i < len(invoiceItems); i++ {
			existingInvoiceJson = append(existingInvoiceJson, invoiceItems[i])
		}
		var updatedInvoice structs.Invoice
		invoiceDataJson, _ := json.Marshal(existingInvoiceJson)
		opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
		err = invoices.FindOneAndUpdate(
			context.TODO(),
			bson.D{{"_id", foundInvoice.ID}},
			bson.M{"$set": bson.M{"data": string(invoiceDataJson)}},
			opts).
			Decode(&updatedInvoice)
		if err != nil {
			log.Printf("Update Mongo Invoice during CRON Failed %v", err)
		}
		fmt.Printf("Invoice Existing, Updated %v", updatedInvoice.Data)
		chargeInvoice(updatedInvoice, invoiceItems, dateOfInvoice, false)
	}
}

func chargeInvoice(invoice structs.Invoice, invoiceItems []structs.InvoiceItem, dateOfInvoice string, makeNewInvoice bool) structs.Invoice {
	if devEnv == "true" {
		stripeKey = s3credentials.GetS3Data("stripe", "testkey", "")
		viewsPrice = s3credentials.GetS3Data("stripe", "analyticsDev", "views_price")
		clicksPrice = s3credentials.GetS3Data("stripe", "analyticsDev", "clicks_price")
	}
	stripe.Key = stripeKey
	stripe73.Key = stripeKey
	var stripeCustomer string
	if invoice.CustomerNetwork == "Tycoon Systems Corp." {
		users := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("users")
		record := &structs.User{}
		err := users.FindOne(context.TODO(), bson.D{{"_id", invoice.Customer}}).Decode(&record) // Get from video data
		if err != nil {
			return invoice
		}
		stripeCustomer = record.Payment
	}
	metadata := make(map[string]interface{})
	metadata["views_details"] = invoiceItems[0].Details
	metadata["views_data"] = invoiceItems[0].Data
	metadata["views_cost"] = invoiceItems[0].Cost
	viewsCost := strconv.Itoa(int(math.Floor(invoiceItems[0].Cost * 100)))
	clicksCost := strconv.Itoa(int(math.Floor(invoiceItems[1].Cost * 100)))
	// itap := makeInvoiceItems(invoiceItems, viewsCost, clicksCost, dateOfInvoice)
	// invoiceParams := &stripe.InvoiceParams{
	// 	Customer:    stripe73.String(stripeCustomer),
	// 	AutoAdvance: BoolPointer(false),
	// 	Description: &invoice.Note,
	// 	Footer:      &invoice.FootDetails,
	// }
	// fmt.Printf("%v, %v", itap, invoiceParams)
	//params.InvoiceItems = itap
	// Generated by curl-to-Go: https://mholt.github.io/curl-to-go

	var invoiceId string
	invoices := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("invoices")
	if invoice.StripeId != "" {
		invoiceId = invoice.StripeId
	}

	params := url.Values{}
	params2 := url.Values{}
	params.Add("currency", "USD")
	params2.Add("currency", "USD")
	if invoiceId != "" && invoiceId != "nomatch" {
		params.Add("invoice", invoiceId)
		params2.Add("invoice", invoiceId)
	}
	params.Add("customer", stripeCustomer)
	params.Add("amount", viewsCost)
	params.Add("metadata[type]", "views")
	params.Add("metadata[quantity]", fmt.Sprintf("%f", invoiceItems[0].Units))
	params.Add("description", time.Now().Format("3:04PM")+": Views for Advertising Services on date "+dateOfInvoice+" with total "+strconv.Itoa(int(math.Floor(invoiceItems[0].Units)))+" views at rate $0.00719 on Ad: "+invoiceItems[0].AdTitle+" "+invoiceItems[0].AdId)
	body := strings.NewReader(params.Encode())

	req, err := http.NewRequest("POST", "https://api.stripe.com/v1/invoiceitems", body)
	if err != nil {
		fmt.Printf("err %v", err)
		// handle err
	}
	req.SetBasicAuth(stripeKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	params2.Add("customer", stripeCustomer)
	params2.Add("amount", clicksCost)
	params2.Add("metadata[type]", "clicks")
	params2.Add("metadata[quantity]", fmt.Sprintf("%f", invoiceItems[1].Units))
	params2.Add("description", time.Now().Format("3:04PM")+": Clicks for Advertising Services on date "+dateOfInvoice+" with total "+strconv.Itoa(int(math.Floor(invoiceItems[1].Units)))+" clicks at rate $0.35 on Ad: "+invoiceItems[1].AdTitle+" "+invoiceItems[1].AdId)
	fmt.Printf("Price Views %v %v Clicks %v %v %v", viewsCost, invoiceItems[0].Cost, clicksCost, invoiceItems[1].Cost, stripeCustomer)
	body2 := strings.NewReader(params2.Encode())

	req2, err2 := http.NewRequest("POST", "https://api.stripe.com/v1/invoiceitems", body2)
	if err2 != nil {
		fmt.Printf("err2 %v", err2)
		// handle err
	}
	req2.SetBasicAuth(stripeKey, "")
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	resp2, err2 := http.DefaultClient.Do(req2)
	if err != nil && err2 != nil {
		fmt.Printf("err %v err2 %v", err, err2)
		// handle err
	}
	defer resp.Body.Close()
	defer resp2.Body.Close()
	if makeNewInvoice {
		params3 := url.Values{}
		params3.Add("customer", stripeCustomer)
		params3.Add("currency", "USD")
		params3.Add("auto_advance", "true")
		params3.Add("collection_method", "charge_automatically")
		body3 := strings.NewReader(params3.Encode())

		req3, err3 := http.NewRequest("POST", "https://api.stripe.com/v1/invoices", body3)
		if err3 != nil {
			fmt.Printf("err3 %v", err3)
			// handle err
		}
		req3.SetBasicAuth(stripeKey, "")
		req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp3, err3 := http.DefaultClient.Do(req3)
		if err3 != nil {
			fmt.Printf("err3 %v", err3)
			// handle err
		}

		defer resp3.Body.Close()

		b, err := io.ReadAll(resp3.Body)
		// b, err := ioutil.ReadAll(resp.Body)  Go.1.15 and earlier
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Println(string(b))
		var bodyString string = string(b)
		invoiceId = s3credentials.JsonPointer([]byte(bodyString), "id", "", "")
		fmt.Printf("Invoice Id %v\n", invoiceId)
		if invoiceId != "" && invoiceId != "nomatch" {
			var updatedInvoice structs.Invoice
			opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
			err = invoices.FindOneAndUpdate(
				context.TODO(),
				bson.D{{"_id", invoice.ID}},
				bson.M{"$set": bson.M{"stripeid": invoiceId}},
				opts).
				Decode(&updatedInvoice)
			if err != nil {
				log.Printf("Update Mongo Invoice during CRON Failed %v", err)
			}
		}
	} else {
		invoiceId = invoice.StripeId
	}
	return invoice
}

func makeInvoiceItems(invoiceItems []structs.InvoiceItem, viewsCost int64, clicksCost int64, dateOfInvoice string) []*stripe.InvoiceUpcomingInvoiceItemParams {
	invoiceItemsToParams := []*stripe.InvoiceUpcomingInvoiceItemParams{}
	invoiceItemsToParams = append(invoiceItemsToParams, &stripe.InvoiceUpcomingInvoiceItemParams{
		Amount:      &viewsCost,
		Currency:    StringPointer("USD"),
		Description: StringPointer("Views for Advertising Services on date " + dateOfInvoice + "with total " + fmt.Sprintf("%f", invoiceItems[0].Units) + " views on Ad: " + invoiceItems[0].AdTitle + " " + invoiceItems[0].AdId),
		InvoiceItem: StringPointer("Views"),
		Quantity:    FloatToInt64Pointer(invoiceItems[0].Units),
	})
	invoiceItemsToParams = append(invoiceItemsToParams, &stripe.InvoiceUpcomingInvoiceItemParams{
		Amount:      &clicksCost,
		Currency:    StringPointer("USD"),
		Description: StringPointer("Views for Advertising Services on date " + dateOfInvoice + "with total " + fmt.Sprintf("%f", invoiceItems[1].Units) + " clicks on Ad: " + invoiceItems[1].AdTitle + " " + invoiceItems[1].AdId),
		InvoiceItem: StringPointer("Views"),
		Quantity:    FloatToInt64Pointer(invoiceItems[0].Units),
	})
	return invoiceItemsToParams
}

func FloatToInt64Pointer(f float64) *int64 {
	t := int64(f)
	return &t
}

func BoolPointer(b bool) *bool {
	return &b
}

func StringPointer(s string) *string {
	return &s
}
