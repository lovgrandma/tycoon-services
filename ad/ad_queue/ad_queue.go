package ad_queue

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"

	"html/template"
	"os"

	"github.com/hibiken/asynq"
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
	vastUploadFolderPath = "../tycoon-services-vast-generation/"
	s3VideoEndpoint      = s3credentials.GetS3Data("awsConfig", "buckets", "tycoon-systems-ads")
	devEnv               = s3credentials.GetS3Data("app", "dev", "")
	adServerEndPoint     = s3credentials.GetS3Data("app", "server", "")
	adServerPort         = s3credentials.GetS3Data("app", "adServerPort", "")
)

const (
	TypeVastGenerate           = "vast:generate"
	_vastAlreadyGeneratedCheck = "Vast has already been generated"
)

func main() {

}

func ProvisionVastJob(vast structs.VastTag) string {
	if reflect.TypeOf(vast.ID).Kind() == reflect.String {
		task, err := NewVastDeliveryTask(vast)
		if err != nil {
			log.Printf("Could not create Vast Generate task at Task Creation: %v", err)
		}
		info, err := jobClient.Enqueue(task)
		if err != nil {
			log.Printf("Could not create Vast Generate task at Enqueue: %v", err)
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
func NewVastDeliveryTask(vast structs.VastTag) (*asynq.Task, error) {
	payload, err := json.Marshal(vast)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeVastGenerate, payload), nil
}

// Unmarshal queued delivery task to determine if in correct format
func HandleVastDeliveryTask(ctx context.Context, t *asynq.Task) error {
	var p structs.VastTag
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}
	log.Printf("Beginning VastTag Generation for ID: %v, Socket: %v", p.ID, p.Socket)
	err := PerformVastGeneration(p)
	if err != nil {
		return fmt.Errorf("Perform VastTag Gemeration failed: %v: %w", err, asynq.SkipRetry)
	}
	return nil
}

func PerformVastGeneration(vast structs.VastTag) error {
	fmt.Printf("Yes! %v, %v, %v, %v, %v, %v, %v, %v", vast.ID, vast.DocumentId, vast.Socket, vast.Status, vast.Url, vast.TrackingUrl, vast.AdTitle, vast.ClickthroughUrl)
	f, err := GenerateVast(vast)
	if err != nil {
		if len(f) > 0 {
			DeleteFile(f, vastUploadFolderPath)
		}
		return err
	}
	var url string
	url, err = UploadVastS3(f)
	if err != nil {
		DeleteFile(f, vastUploadFolderPath)
		return err
	} else if len(url) == 0 {
		DeleteFile(f, vastUploadFolderPath)
		return fmt.Errorf("Error uploading VastTag to S3. No url from UploadVastS3 function")
	}
	UpdateRecord(url, vast.ID)
	DeleteFile(f, vastUploadFolderPath)
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
	var docType string = r.URL.Query().Get("type")      // Get document type
	duration, err := GetVideoDuration(document)         // Get duration to determine length of video for ad breaks
	fmt.Printf("Duration, docType %v, %v,", docType, duration)
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
	fmt.Printf("Rolls %v", validMidRolls)
	var adBreaks []AdBreak
	// Append Pre-roll
	adBreaks = append(adBreaks,
		AdBreak{xml.Name{}, "start", "linear", "preroll",
			AdSource{xml.Name{}, "preroll-ad-1", "false", "true",
				AdTagUri{xml.Name{}, "vast3", "<![CDATA[ https://pubads.g.doubleclick.net/gampad/ads?slotname=/21775744923/external/vmap_ad_samples&sz=640x480&ciu_szs=300x250&cust_params=sample_ar%3Dpremidpostpod&url=&unviewed_position_start=1&output=xml_vast3&impl=s&env=vp&gdfp_req=1&ad_rule=0&useragent=Mozilla/5.0+(Windows+NT+10.0%3B+Win64%3B+x64)+AppleWebKit/537.36+(KHTML,+like+Gecko)+Chrome/103.0.0.0+Safari/537.36,gzip(gfe)&vad_type=linear&vpos=preroll&pod=1&ppos=1&lip=true&min_ad_duration=0&max_ad_duration=30000&vrid=1270234&cmsid=496&video_doc_id=short_onecue&kfa=0&tfcd=0 ]]>"},
			},
		},
	)
	// Append Mid-rolls
	midRoll := 1
	for i := 0; i < len(record.Timeline); i++ {
		if _, ok := record.Timeline[i].(structs.TimelineNode); ok {
			field := record.Timeline[i].(structs.TimelineNode).Type
			time := record.Timeline[i].(structs.TimelineNode).Time
			amount := record.Timeline[i].(structs.TimelineNode).Amount
			if field == "ad-marker" {
				for j := 0; j < amount; j++ {
					midRollNum := j + 1
					adBreaks = append(adBreaks,
						AdBreak{xml.Name{}, resolveSecondsToReadableAdTimeFormat(time), "linear", "midroll-" + strconv.Itoa(midRoll),
							AdSource{xml.Name{}, "midroll-" + strconv.Itoa(midRoll) + "-ad-" + strconv.Itoa(midRollNum), "false", "true",
								AdTagUri{xml.Name{}, "vast3", "<![CDATA[ https://pubads.g.doubleclick.net/gampad/ads?slotname=/21775744923/external/vmap_ad_samples&sz=640x480&ciu_szs=300x250&cust_params=sample_ar%3Dpremidpostpod&url=&unviewed_position_start=1&output=xml_vast3&impl=s&env=vp&gdfp_req=1&ad_rule=0&cue=15000&useragent=Mozilla/5.0+(Windows+NT+10.0%3B+Win64%3B+x64)+AppleWebKit/537.36+(KHTML,+like+Gecko)+Chrome/103.0.0.0+Safari/537.36,gzip(gfe)&vad_type=linear&vpos=midroll&pod=2&mridx=1&rmridx=1&ppos=1&min_ad_duration=0&max_ad_duration=30000&vrid=1270234&cmsid=496&video_doc_id=short_onecue&kfa=0&tfcd=0 ]]>"},
							},
						},
					)
				}
				midRoll = midRoll + 1
			}
		}

	}
	// Append Post-roll
	adBreaks = append(adBreaks,
		AdBreak{xml.Name{}, "end", "linear", "postroll",
			AdSource{xml.Name{}, "postroll-ad-1", "false", "true",
				AdTagUri{xml.Name{}, "vast3", "<![CDATA[ https://pubads.g.doubleclick.net/gampad/ads?slotname=/21775744923/external/vmap_ad_samples&sz=640x480&ciu_szs=300x250&cust_params=sample_ar%3Dpremidpostpod&url=&unviewed_position_start=1&output=xml_vast3&impl=s&env=vp&gdfp_req=1&ad_rule=0&useragent=Mozilla/5.0+(Windows+NT+10.0%3B+Win64%3B+x64)+AppleWebKit/537.36+(KHTML,+like+Gecko)+Chrome/103.0.0.0+Safari/537.36,gzip(gfe)&vad_type=linear&vpos=postroll&pod=3&ppos=1&lip=true&min_ad_duration=0&max_ad_duration=30000&vrid=1270234&cmsid=496&video_doc_id=short_onecue&kfa=0&tfcd=0 ]]>"},
			},
		},
	)
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
		s3VideoEndpoint = s3credentials.GetS3Data("awsConfig", "devBuckets", "tycoon-systems-ads-development")
	}
	fmt.Printf("s3VideoEndpoint %v\n", s3VideoEndpoint)
	upFrom := vastUploadFolderPath
	upTo := "vasttag/"
	f, _ := os.Open(upFrom + fileName)
	r := bufio.NewReader(f)
	fmt.Printf("Uploading: %v %v\n", upFrom+fileName, upTo+fileName)
	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(s3VideoEndpoint),
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
	err := adUnits.FindOne(context.TODO(), bson.D{{"_id", id}}).Decode(&record) // Get from phone number data
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
	return strconv.Itoa(hours) + ":" + strconv.Itoa(minutes) + ":" + strconv.Itoa(seconds)
}

func resolveValidAdsMidRolls(r *http.Request) []structs.AdUnit {
	// Find all valid mid rolls/ads from AdUnits table
	var records []structs.AdUnit
	if client != nil {
		adUnits := client.Database(s3credentials.GetS3Data("mongo", "db", "")).Collection("adunits")
		cursor, err := adUnits.Find(context.TODO(), bson.D{{"publish", true}}) // Get from video data
		if err != nil {
			fmt.Printf("Err %v", err)
			return records
		} else {
			fmt.Printf("Cursor %v", cursor.ID())
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
