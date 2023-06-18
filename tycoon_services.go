package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"

	// "github.com/google/uuid"
	"errors"
	"net"
	"reflect"

	"tycoon.systems/tycoon-services/ad/ad_queue"
	ad_workers "tycoon.systems/tycoon-services/ad/ad_queue/workers"
	"tycoon.systems/tycoon-services/s3credentials"
	"tycoon.systems/tycoon-services/security"
	"tycoon.systems/tycoon-services/sms/sms_queue"
	sms_workers "tycoon.systems/tycoon-services/sms/sms_queue/workers"
	"tycoon.systems/tycoon-services/structs"
	"tycoon.systems/tycoon-services/video/video_queue"
	"tycoon.systems/tycoon-services/video/video_queue/transcode"
	video_workers "tycoon.systems/tycoon-services/video/video_queue/workers"

	"time"

	"github.com/go-co-op/gocron"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	adpb "tycoon.systems/tycoon-services/ad"
	pb "tycoon.systems/tycoon-services/sms"
	vpb "tycoon.systems/tycoon-services/video"

	"net/http"
	"net/url"
	"os"
	"regexp"
)

const (
	serviceAddress = "127.0.0.1"
)

var (
	supportedAdOrigins    = []structs.Origin{{"https://www.tycoon.systems"}, {"www.tycoon.systems"}, {"https://imasdk.googleapis.com"}, {"imasdk.googleapis.com"}, {"http://localhost:3000"}, {"localhost:3000"}, {"https://tycoon-systems-client.local:3000"}, {"tycoon-systems-client.local:3000"}, {"https://tycoon-systems-client.local"}, {"tycoon-systems-client.local"}}
	devEnv                = s3credentials.GetS3Data("app", "dev", "")
	sslPath               = s3credentials.GetS3Data("app", "sslPath", "")
	servicesSslPath       = s3credentials.GetS3Data("app", "servicesSslPath", "")
	goodServiceSsl        = s3credentials.GetS3Data("app", "goodServiceSsl", "")
	port                  = ":" + s3credentials.GetS3Data("app", "services", "smsClient")
	videoPort             = ":" + s3credentials.GetS3Data("app", "services", "videoClient")
	adPort                = ":" + s3credentials.GetS3Data("app", "services", "adClient")
	adServerPort          = ":" + s3credentials.GetS3Data("app", "adServerPort", "")
	streamingServicesPort = ":" + s3credentials.GetS3Data("app", "streamingServicesPort", "")
	streamingServer       = s3credentials.GetS3Data("app", "streamingServer", "")
)

type SmsManagementServer struct {
	pb.UnimplementedSmsManagementServer
}

type VideoManegmentServer struct {
	vpb.UnimplementedVideoManagementServer
}

type AdManagementServer struct {
	adpb.UnimplementedAdManagementServer
}

func CheckRequestAuth(domainKey string) string {
	auth := s3credentials.GetS3Data("business", "keys", domainKey)
	if auth != "" {
		return auth
	}
	return ""
}

func (a *AdManagementServer) CreateNewVastCompliantAdVideoJob(ctx context.Context, in *adpb.NewVast) (*adpb.Vast, error) {
	if reflect.TypeOf(in.GetIdentifier()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetDocumentId()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetUsername()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetSocket()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetUuid()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetHash()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetTrackingUrl()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetAdTitle()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetClickthroughUrl()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetCallToAction()).Kind() == reflect.String {
		if len(in.GetIdentifier()) > 0 && len(in.GetUsername()) > 0 && len(in.GetSocket()) > 0 && len(in.GetUuid()) > 0 && len(in.GetHash()) > 0 && len(in.GetTrackingUrl()) > 0 && len(in.GetAdTitle()) > 0 && len(in.GetClickthroughUrl()) > 0 && len(in.GetStartTime()) > 0 && len(in.GetEndTime()) > 0 && len(in.GetPlayTime()) > 0 {
			reqAuth := CheckRequestAuth(in.GetDomainKey())
			if reqAuth != "" {
				var authenticated bool = security.CheckAuthenticRequest(in.GetUsername(), in.GetIdentifier(), in.GetHash(), reqAuth) // Access mongo and check user identifier against hash to determine if request should be honoured
				if authenticated != false {
					jobProvisioned := ad_queue.ProvisionCreateNewVastCompliantAdVideoJob(structs.VastTag{ID: in.GetUuid(), Socket: in.GetIdentifier(), Status: "Pending", Url: "", DocumentId: in.GetDocumentId(), TrackingUrl: in.GetTrackingUrl(), AdTitle: in.GetAdTitle(), ClickthroughUrl: in.GetClickthroughUrl(), CallToAction: in.GetCallToAction(), StartTime: in.GetStartTime(), EndTime: in.GetEndTime(), PlayTime: in.GetPlayTime(), Domain: reqAuth})
					if jobProvisioned != "failed" {
						return &adpb.Vast{Status: "Good", ID: in.GetUuid(), Socket: in.GetIdentifier(), Domain: reqAuth}, nil
					}
				}
			}
		}
	}
	err := errors.New("Request to Ad Service failed")
	return &adpb.Vast{Status: "Bad Request", ID: "Tycoon Services", Socket: "null", Destination: "null", Filename: "null", Path: "null"}, err
}

/* Determine if request to create Sms blast is genuine and if user has permissions. Attempt provision for job */
func (s *SmsManagementServer) CreateNewSmsBlast(ctx context.Context, in *pb.NewMsg) (*pb.Msg, error) {
	if reflect.TypeOf(in.GetContent()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetFrom()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetUsername()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetIdentifier()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetHash()).Kind() == reflect.String {
		if len(in.GetContent()) > 0 && len(in.GetFrom()) > 0 && len(in.GetUsername()) > 0 && len(in.GetIdentifier()) > 0 && len(in.GetHash()) > 0 {
			log.Printf("Received Sms: %v, %v, %v, %v, %v, %v", in.GetContent(), in.GetFrom(), in.GetUsername(), in.GetIdentifier(), in.GetHash(), in.GetDomainKey())
			reqAuth := CheckRequestAuth(in.GetDomainKey())
			if reqAuth != "" {
				var authenticated bool = security.CheckAuthenticRequest(in.GetUsername(), in.GetIdentifier(), in.GetHash(), reqAuth) // Access mongo and check user identifier against hash to determine if request should be honoured
				log.Printf("Authenticated %v", authenticated)
				if authenticated != false {
					jobProvisioned := sms_queue.ProvisionSmsJob(structs.Msg{Content: in.GetContent(), From: in.GetFrom(), Domain: reqAuth})
					if jobProvisioned != "failed" {
						return &pb.Msg{Content: in.GetContent(), From: in.GetFrom(), JobId: jobProvisioned, Domain: reqAuth}, nil
					}
				}
			}
		}
	}
	err := errors.New("Request to Sms Service failed")
	return &pb.Msg{Content: "Bad Request", From: "Tycoon Services", JobId: "null"}, err
}

func (v *VideoManegmentServer) CreateNewVideoUpload(ctx context.Context, in *vpb.NewVideo) (*vpb.Video, error) {
	if reflect.TypeOf(in.GetIdentifier()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetUsername()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetSocket()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetDestination()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetFilename()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetPath()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetUuid()).Kind() == reflect.String &&
		reflect.TypeOf(in.GetHash()).Kind() == reflect.String {
		if len(in.GetIdentifier()) > 0 && len(in.GetUsername()) > 0 && len(in.GetSocket()) > 0 && len(in.GetDestination()) > 0 && len(in.GetFilename()) > 0 && len(in.GetPath()) > 0 && len(in.GetUuid()) > 0 && len(in.GetHash()) > 0 {
			log.Printf("Received Video: %v, %v, %v, %v, %v, %v, %v, %v", in.GetIdentifier(), in.GetUsername(), in.GetSocket(), in.GetDestination(), in.GetFilename(), in.GetPath(), in.GetUuid(), in.GetHash())
			reqAuth := CheckRequestAuth(in.GetDomainKey())
			if reqAuth != "" {
				var authenticated bool = security.CheckAuthenticRequest(in.GetUsername(), in.GetIdentifier(), in.GetHash(), reqAuth) // Access mongo and check user identifier against hash to determine if request should be honoured
				if authenticated != false {
					vid := &vpb.Video{Status: "processing", ID: in.GetUuid(), Socket: in.GetSocket(), Destination: "null", Filename: "null", Path: in.GetPath(), Domain: reqAuth}
					_, _ = transcode.UpdateMongoRecord(vid, []structs.MediaItem{}, "waiting", []structs.Thumbnail{}, true) // Build initial record for tracking during processing
					log.Printf("\nReq Auth %v\n", reqAuth)
					jobProvisioned := video_queue.ProvisionVideoJob(&vpb.Video{Status: "processing", ID: in.GetUuid(), Socket: in.GetSocket(), Destination: in.GetDestination(), Filename: in.GetFilename(), Path: in.GetPath(), Domain: reqAuth})
					if jobProvisioned != "failed" {
						return vid, nil
					}
				}
			}
		}
	}
	err := errors.New("Request to Video Service failed")
	return &vpb.Video{Status: "Bad Request", ID: "Tycoon Services", Socket: "null", Destination: "null", Filename: "null", Path: "null"}, err
}

func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Load server's certificate and private key
	serverCert, err := tls.LoadX509KeyPair(servicesSslPath+"server.crt", servicesSslPath+"server.key")
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert,
	}

	return credentials.NewTLS(config), nil
}

func main() {
	err := os.Setenv("dev", devEnv)
	if err != nil {
		return
	}
	lis, err := net.Listen("tcp", serviceAddress+port) // Server for ingesting SMS and general requests
	if err != nil {
		log.Fatalf("Failed to listen on %v: %v", port, err)
	}
	var s *grpc.Server
	if devEnv == "false" && goodServiceSsl == "true" {
		tlsCredentials, err := loadTLSCredentials()
		if err != nil {
			log.Fatal("Cannot load TLS credentials (Main Server): ", err)
		}
		s = grpc.NewServer(
			grpc.Creds(tlsCredentials),
		)
	} else {
		s = grpc.NewServer()
	}
	pb.RegisterSmsManagementServer(s, &SmsManagementServer{})
	go newVideoServer()                  // Server for ingesting video job requests
	go newAdServer()                     // Server for ingesting ad job requests
	go sms_workers.BuildWorkerServer()   // Server for SMS job queue
	go video_workers.BuildWorkerServer() // Server for video job queue
	go ad_workers.BuildWorkerServer()    // Server for ad job queue
	go serveAdCompliantServer()
	go serveStreamingServer()
	go serverCronRoutines() // Initiate server cron routines
	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}

func logOutput() {
	logFilePath := "logs.txt" // Path to the log file

	// Open the log file in append mode, creating it if it doesn't exist
	logFile, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal("Error opening log file:", err)
	}
	defer logFile.Close()

	// Set the log output to the log file
	log.SetOutput(logFile)
}

func serveAdCompliantServer() *http.Server {
	http.HandleFunc("/ads/vmap", handleAdRequests)
	http.HandleFunc("/ads/vast", handleAdVastRequests)
	http.HandleFunc("/ads/error", handleAdErrorRequests)
	http.HandleFunc("/ads/view", handleAdViewRequests)
	http.HandleFunc("/ads/track", handleAdTrackRequests)
	log.Printf("Ad Compliant Server listening at %v", adServerPort)
	err := http.ListenAndServe(serviceAddress+adServerPort, nil)
	if err != nil {
		log.Fatalf("Failed to run Ad Compliant Server: %v", err)
	}
	return &http.Server{}
}

func serveStreamingServer() *http.Server {
	http.HandleFunc("/stream/publish", handleIngestLiveStreamPublishAuthentication)
	http.HandleFunc("/stream/ingest", handleIncomingStreamPublish)
	log.Printf("Media Compliant Server listening at %v", streamingServicesPort)
	err := http.ListenAndServe(serviceAddress+streamingServicesPort, nil)
	if err != nil {
		log.Fatalf("Failed to run Media Compliant Server: %v", err)
	}
	return &http.Server{}
}

func matchOrigin(w http.ResponseWriter, r *http.Request) (bool, http.ResponseWriter) {
	origin := r.Header.Get("Origin")
	log.Printf("Origin %v", origin)
	for i := range supportedAdOrigins {
		if supportedAdOrigins[i].Name == origin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			return true, w
		}
	}
	return false, w
}

func handleIncomingStreamPublish(w http.ResponseWriter, r *http.Request) {
	log.Println("%v %v %v", r, r.URL, r.Method)
	log.Println("Received Publish request at /stream/ingest")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		// Handle error
		log.Println("Error reading request body:", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// Log the request body
	log.Println("Request body:", string(body))

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query, err := url.ParseQuery(string(body))
	if err != nil {
		log.Println("Error parsing query string:", err)
		http.Error(w, "Error parsing query string", http.StatusInternalServerError)
		return
	}

	// Get the value of the "name" field
	key := query.Get("key")
	domain := query.Get("domain")
	input := query.Get("input")
	bucket := query.Get("bucket")
	if key == "" || domain == "" || input == "" || bucket == "" {
		log.Println("A field is missing")
		http.Error(w, "A field is missing", http.StatusBadRequest)
		return
	}
	// Stream approved
	log.Printf("Stream Approved %v %v %v %v", key, domain, input, bucket)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Stream approved")
	return
}

func handleIngestLiveStreamPublishAuthentication(w http.ResponseWriter, r *http.Request) {
	log.Println("%v %v %v", r, r.URL, r.Method)
	log.Println("Received Publish request at /stream/publish")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		// Handle error
		log.Println("Error reading request body:", err)
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}

	// Log the request body
	log.Println("Request body:", string(body))

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query, err := url.ParseQuery(string(body))
	if err != nil {
		log.Println("Error parsing query string:", err)
		http.Error(w, "Error parsing query string", http.StatusInternalServerError)
		return
	}

	// Get the value of the "name" field
	streamKey := query.Get("name")
	if streamKey == "" {
		log.Println("Name field is missing")
		http.Error(w, "Name field is missing", http.StatusBadRequest)
		return
	}

	// Check if the stream key is valid or authorized
	if isValidStreamKey(streamKey) == true {
		// Start processing the incoming stream from OBS
		log.Printf("Stream Key Valid")
		domain := GetDomainStreamKey(streamKey)
		key := GetStreamIdStreamKey(streamKey)
		user := security.FindUserpByFieldValue(domain, "key", key)
		var userId string
		if len(user) != 0 && len(domain) != 0 && len(key) != 0 {
			if reflect.TypeOf(user["id"]).Kind() == reflect.String {
				userId = user["id"].(string)
			}
			resolvedStream := security.FindLive(domain, "author", userId, "creation", "desc", "", "10")
			if len(resolvedStream) != 0 {
				log.Printf("Resolved Streams %v", resolvedStream)
				var name string
				if reflect.TypeOf(resolvedStream["id"]).Kind() == reflect.String {
					name = resolvedStream["id"].(string)
				}
				log.Printf("Stream Name %v Domain %v", name, domain)
				// Handle the stream ingestion logic here
				bucket := domain + "_live"
				// redirectStream := streamingServer + "/stream/?domain=" + domain + "&key=" + name + "&input=" + streamKey + "&bucket=" + bucket + "&name=" + bucket + "/" + key
				// log.Printf("Redirect Stream %v", redirectStream)
				w.Header().Set("Location", streamingServer+"/hls-live/"+bucket+"/"+key)
				// http.Redirect(w, r, streamingServer+"/hls-live/"+bucket+"/"+key, http.StatusFound)
				w.WriteHeader(http.StatusFound)

				// response := fmt.Sprintf("Name: %s, Domain: %s", name, domain)
				// w.WriteHeader(http.StatusOK)
				// fmt.Fprint(w, response)
				return
			}
		}
	}
	// Send an error response for invalid or unauthorized stream key
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprint(w, "Stream denied")
	return
}

func GetDomainStreamKey(streamKey string) string {
	domainRegex := `^([^-\s]+)`
	reg := regexp.MustCompile(domainRegex) // Compile the regular expression pattern
	matches := reg.FindStringSubmatch(streamKey)
	if len(matches) > 1 {
		return matches[1] // The first submatch (index 1) contains the desired word
	}
	return ""
}

func GetStreamIdStreamKey(streamKey string) string {
	keyRegex := `-[a-zA-Z0-9-]+$`
	// Compile the regular expression pattern
	reg := regexp.MustCompile(keyRegex)

	// Find the submatch
	match := reg.FindString(streamKey)

	if match != "" {
		return match[1:] // Remove the leading hyphen ("-") from the match
	}
	return ""
}

// Verify if the stream key is valid and authorized
func isValidStreamKey(streamKey string) bool {
	// Find the submatches
	var domain string = GetDomainStreamKey(streamKey)
	var key string
	if len(domain) > 1 {
		key = GetStreamIdStreamKey(streamKey)
	}
	log.Printf("Domain %v key %v", domain, key)
	keyValidated := security.FindUserpByFieldValue(domain, "key", key)
	log.Printf("Validated %v", keyValidated)
	if len(keyValidated) != 0 {
		if reflect.TypeOf(keyValidated["key"]).Kind() == reflect.String {
			if len(keyValidated["key"].(string)) > 0 && len(keyValidated["username"].(string)) > 0 {
				return true
			}
		}
	}
	return false
}

func handleAdErrorRequests(w http.ResponseWriter, r *http.Request) {
	f := r.URL.Query().Get("videoplayfailed")
	log.Printf("Ad Failure %v", f)
	_, w = matchOrigin(w, r)
	w.WriteHeader(http.StatusOK)

	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Content-Type", "application/text")
	w.Write([]byte("Success"))
	return
}

func handleAdVastRequests(w http.ResponseWriter, r *http.Request) {
	_, w = matchOrigin(w, r)

	vmap, err := ad_queue.GenerateAndServeVast(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Write(vmap)
}

func handleAdRequests(w http.ResponseWriter, r *http.Request) {
	_, w = matchOrigin(w, r)

	vmap, err := ad_queue.GenerateAndServeVmap(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Write(vmap)
}

func handleAdViewRequests(w http.ResponseWriter, r *http.Request) {
	match, w := matchOrigin(w, r)
	if match == true {
		ad_queue.RecordView(r)
	}
	w.Write([]byte{})
}

func handleAdTrackRequests(w http.ResponseWriter, r *http.Request) {
	match, w := matchOrigin(w, r)
	if match == true {
		ad_queue.HandleTrackRequest(r)
	}
	w.Write([]byte{})
}

func newVideoServer() *grpc.Server {
	lis2, err := net.Listen("tcp", serviceAddress+videoPort)
	if err != nil {
		log.Fatalf("Failed to listen on %v: %v", videoPort, err)
	}
	var v *grpc.Server
	if devEnv == "false" && goodServiceSsl == "true" {
		tlsCredentials, err := loadTLSCredentials()
		if err != nil {
			log.Fatal("Cannot load TLS credentials (Main Server): ", err)
		}
		v = grpc.NewServer(
			grpc.Creds(tlsCredentials),
		)
	} else {
		v = grpc.NewServer()
	}
	vpb.RegisterVideoManagementServer(v, &VideoManegmentServer{})
	log.Printf("Video Server listening at %v", lis2.Addr())
	if err := v.Serve(lis2); err != nil {
		log.Fatalf("Failed to run Video server: %v", err)
	}
	return v
}

func newAdServer() *grpc.Server {
	lis3, err := net.Listen("tcp", serviceAddress+adPort)
	if err != nil {
		log.Fatalf("Failed to listen on %v: %v", adPort, err)
	}
	var a *grpc.Server
	if devEnv == "false" && goodServiceSsl == "true" {
		tlsCredentials, err := loadTLSCredentials()
		if err != nil {
			log.Fatal("Cannot load TLS credentials (Main Server): ", err)
		}
		a = grpc.NewServer(
			grpc.Creds(tlsCredentials),
		)
	} else {
		a = grpc.NewServer()
	}
	adpb.RegisterAdManagementServer(a, &AdManagementServer{})
	log.Printf("Ad Server listening at %v", lis3.Addr())
	if err := a.Serve(lis3); err != nil {
		log.Fatalf("Failed to run Ad server: %v", err)
	}
	return a
}

func serverCronRoutines() {
	loc, _ := time.LoadLocation("America/New_York")
	s := gocron.NewScheduler(loc)
	// Default CRON Time 01:00:00 (1am). Get Current day from yesterday 12 hours ago
	job, err := s.Every(1).Day().At("01:00:00").Do(func() {
		currentTime := time.Now()
		log.Printf("CRON %v", currentTime.Format("2006-01-02"))
		ad_queue.AggregateAdAnalytics(time.Now().Add(-time.Hour * 12))
	})
	ad_queue.AggregateAdAnalytics(time.Now().Add(-time.Hour * 12))
	log.Printf("Job %v Err %v\n", job.NextRun(), err)
	s.StartBlocking()
}
