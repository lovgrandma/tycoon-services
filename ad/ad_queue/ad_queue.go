package ad_queue

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"reflect"
	"regexp"
	"strconv"

	"html/template"
	"os"

	"github.com/hibiken/asynq"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"go.mongodb.org/mongo-driver/bson"
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

	"time"

	"google.golang.org/grpc"
	adpb "tycoon.systems/tycoon-services/ad"
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
	client, err          = mongo.Connect(context.TODO(), clientOpts)
	jobQueueAddr         = s3credentials.GetS3Data("redis", "redishost", "") + ":" + s3credentials.GetS3Data("redis", "tycoon_systems_ad_queue_port", "")
	jobClient            = asynq.NewClient(asynq.RedisClientOpt{Addr: jobQueueAddr})
	returnJobResultPort  = "6005"
	returnJobResultAddr  = s3credentials.GetS3Data("app", "prodhost", "")
	vastUploadFolderPath = "../tycoon-services-vast-ad-generation/"
	s3VideoEndpoint      = s3credentials.GetS3Data("awsConfig", "buckets", "tycoon-systems-video")
	s3VideoAdEndpoint    = s3credentials.GetS3Data("awsConfig", "buckets", "tycoon-systems-ads")
	devEnv               = s3credentials.GetS3Data("app", "dev", "")
	adServerEndPoint     = s3credentials.GetS3Data("app", "server", "")
	adServerPort         = s3credentials.GetS3Data("app", "adServerPort", "")
	videoCdn             = s3credentials.GetS3Data("prod", "tycoonSystemsVideo1", "")
	adVideoCdn           = s3credentials.GetS3Data("prod", "tycoonSystemsAds1", "")
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

func GenerateVast(vast structs.VastTag) (string, error) {
	type VastData struct {
		AdId               string
		AdSystem           string
		AdTitle            string
		GeneralTrackingUrl string
		ImpressionUrl      string
		ClickthroughUrl    string
		ClickTrackingUrl   string
		MpegDashUrl        string
		HlsUrl             string
		StaticImageUrl     string
		ErrorUrl           string
	}
	const tmpl = `<VAST version="2.0">
	<Ad id="{{.AdId}}">
		<InLine>
			<AdSystem>{{.AdSystem}}</AdSystem>
			<AdTitle>{{.AdTitle}}</AdTitle>
			<Error>{{.ErrorUrl}}&amp;code=[ERRORCODE]</Error>
			<Impression>{{.ImpressionUrl}}</Impression>
			<Creatives>
				<Creative>
					<Linear>
						<Duration>00:00:30</Duration>
						<TrackingEvents>
							<Tracking event="start">{{.GeneralTrackingUrl}}&amp;action=start</Tracking>
							<Tracking event="firstQuartile">{{.GeneralTrackingUrl}}&amp;action=firstQuartile</Tracking>
							<Tracking event="midpoint">{{.GeneralTrackingUrl}}&amp;action=midpoint</Tracking>
							<Tracking event="thirdQuartile">{{.GeneralTrackingUrl}}&amp;action=thirdQuartile</Tracking>
							<Tracking event="complete">{{.GeneralTrackingUrl}}&amp;action=complete</Tracking>
							<Tracking event="pause">{{.GeneralTrackingUrl}}&amp;action=pause</Tracking>
							<Tracking event="mute">{{.GeneralTrackingUrl}}&amp;action=mute</Tracking>
							<Tracking event="fullscreen">{{.GeneralTrackingUrl}}&amp;action=fullscreen</Tracking>
						</TrackingEvents>
						<VideoClicks>
							<ClickThrough>{{.ClickthroughUrl}}</ClickThrough>
							<ClickTracking>{{.ClickTrackingUrl}}</ClickTracking>
						</VideoClicks>
						<MediaFiles>
							<MediaFile id="GDFP" delivery="streaming" width="426" height="240" type="application/x-mpegURL" minBitrate="49" maxBitrate="258" scalable="true" maintainAspectRatio="true">{{.HlsUrl}}
							</MediaFile>
							<MediaFile id="GDFP" delivery="streaming" width="426" height="240" type="application/dash+xml" minBitrate="49" maxBitrate="258" scalable="true" maintainAspectRatio="true">{{.MpegDashUrl}}</MediaFile>
						</MediaFiles>
						<Icons>
							<Icon program="AdChoices" height="16" width="16" xPosition="right" yPosition="top">
								<StaticResource creativeType="image/jpg">{{.StaticImageUrl}}</StaticResource>
								<IconClicks>
									<IconClickThrough>{{.ClickthroughUrl}}</IconClickThrough>
								</IconClicks>
							</Icon>
						</Icons>
					</Linear>
				</Creative>
			</Creatives>
		</InLine>
	</Ad>
</VAST>`
	t := template.Must(template.New("VastTag").Parse(tmpl))
	data := VastData{
		AdId:               vast.ID,
		AdSystem:           "Tycoon",
		AdTitle:            vast.AdTitle,
		GeneralTrackingUrl: "https://" + vast.TrackingUrl + "/tracking?asi=" + vast.ID,
		ImpressionUrl:      "https://" + vast.TrackingUrl + "/tracking?asi=" + vast.ID + "&action=impression",
		ClickthroughUrl:    vast.ClickthroughUrl,
		ClickTrackingUrl:   "https://" + vast.TrackingUrl + "/tracking?asi=" + vast.ID + "&action=clicktracking",
		MpegDashUrl:        "https://" + vast.TrackingUrl + "/fetchAdVideo?asi=" + vast.ID + "&document=" + vast.DocumentId + "&type=dash",
		HlsUrl:             "https://" + vast.TrackingUrl + "/fetchAdVideo?asi=" + vast.ID + "&document=" + vast.DocumentId + "&type=hls",
		StaticImageUrl:     "https://" + vast.TrackingUrl + "/fetchAdImage?asi=" + vast.ID + "&document=" + vast.DocumentId + "&type=staticImage",
		ErrorUrl:           "https://" + vast.TrackingUrl + "/adError?asi=" + vast.ID,
	}
	t.Execute(os.Stdout, data)
	vastOutput := vastUploadFolderPath + vast.ID + ".xml"
	f, err := os.Create(vastOutput)
	if err != nil {
		fmt.Printf("Err %v", err)
		return "", err
	}
	err = t.Execute(f, data)
	if err != nil {
		fmt.Printf("Err %v", err)
		return "", err
	}
	f.Close()
	return vast.ID + ".xml", nil
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

func GetVideoDuration(document string) (int, error) {
	videos := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("videos")
	record := &structs.Video{}
	err := videos.FindOne(context.TODO(), bson.D{{"_id", document}}).Decode(&record) // Get from video data
	if err != nil {
		return 0, fmt.Errorf("Get Video Duration failed")
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
				"<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/view" + vastQueryParams + " ]]>",
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
										{xml.Name{}, "start", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/track" + vastQueryParams + "&event=start ]]>"},
										{xml.Name{}, "firstQuartile", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/track" + vastQueryParams + "&event=firstquartile ]]>"},
										{xml.Name{}, "midpoint", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/track" + vastQueryParams + "&event=midpoint ]]>"},
										{xml.Name{}, "thirdQuartile", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/track" + vastQueryParams + "&event=thirdquartile ]]>"},
										{xml.Name{}, "complete", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/track" + vastQueryParams + "&event=complete ]]>"},
										{xml.Name{}, "pause", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/track" + vastQueryParams + "&event=pause ]]>"},
										{xml.Name{}, "mute", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/track" + vastQueryParams + "&event=mute ]]>"},
										{xml.Name{}, "fullscreen", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/track" + vastQueryParams + "&event=fullscreen ]]>"},
										{xml.Name{}, "acceptInvitationLinear", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/track" + vastQueryParams + "&event=acceptinvitationlinear ]]>"},
										{xml.Name{}, "closeLinear", "<![CDATA[ " + protocol + root + ":" + adServerPort + "/ads/track" + vastQueryParams + "&event=closelinear ]]>"},
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
										"<![CDATA[ https://pagead2.googlesyndication.com/simgad/4446644594546952943 ]]>",
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

func GenerateMockTimeline(duration int) []interface{} {
	minutes := 5
	requiredRolls := (duration / 60) / minutes
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

func UploadVastS3(fileName string) (string, error) {
	fmt.Printf("%v, %v", fileName, vastUploadFolderPath)
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
		return "", err
	}
	client := s3.NewFromConfig(cfg)
	uploader := manager.NewUploader(client)
	if devEnv == "true" {
		s3VideoAdEndpoint = s3credentials.GetS3Data("awsConfig", "devBuckets", "tycoon-systems-ads-development")
	}
	fmt.Printf("s3VideoAdEndpoint %v\n", s3VideoAdEndpoint)
	upFrom := vastUploadFolderPath
	upTo := "vasttag/"
	f, _ := os.Open(upFrom + fileName)
	r := bufio.NewReader(f)
	fmt.Printf("Uploading: %v %v\n", upFrom+fileName, upTo+fileName)
	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s3VideoAdEndpoint),
		Key:    aws.String(upTo + fileName),
		Body:   r,
	})
	if err != nil {
		return "", err
	}
	f.Close() // Close stream to prevent permissions issue
	return fileName, nil
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

func resolveValidAdsMidRolls(r *http.Request) []structs.AdUnit {
	// Find all valid mid rolls/ads from AdUnits table
	var records []structs.AdUnit
	if client != nil {
		adUnits := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("adunits")
		cursor, err := adUnits.Find(context.TODO(), bson.D{{"status", "live"}, {"publish", true}}) // Get from video data
		if err != nil {
			fmt.Printf("Err %v", err)
			return records
		} else {
			// fmt.Printf("Cursor %v", cursor.ID())
			for cursor.Next(context.TODO()) {
				var result structs.AdUnit
				if err := cursor.Decode(&result); err != nil {
					fmt.Printf("Error %v", err)
					return records
				}
				records = append(records, result)
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
