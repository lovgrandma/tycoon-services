package main

import (
	"context"
	"log"

	// "github.com/google/uuid"
	"errors"
	"net"
	"reflect"

	"tycoon.systems/tycoon-services/ad/ad_queue"
	ad_workers "tycoon.systems/tycoon-services/ad/ad_queue/workers"
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
	adpb "tycoon.systems/tycoon-services/ad"
	pb "tycoon.systems/tycoon-services/sms"
	vpb "tycoon.systems/tycoon-services/video"

	"net/http"
)

const (
	port         = ":6000"
	videoPort    = ":6002"
	adPort       = ":6004"
	adServerPort = ":6010"
)

var (
	supportedAdOrigins = []structs.Origin{{"https://www.tycoon.systems"}, {"www.tycoon.systems"}, {"https://imasdk.googleapis.com"}, {"imasdk.googleapis.com"}, {"http://localhost:3000"}, {"localhost:3000"}, {"https://tycoon-systems-client.local:3000"}, {"tycoon-systems-client.local:3000"}, {"https://tycoon-systems-client.local"}, {"tycoon-systems-client.local"}}
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
			var authenticated bool = security.CheckAuthenticRequest(in.GetUsername(), in.GetIdentifier(), in.GetHash()) // Access mongo and check user identifier against hash to determine if request should be honoured
			if authenticated != false {
				jobProvisioned := ad_queue.ProvisionCreateNewVastCompliantAdVideoJob(structs.VastTag{ID: in.GetUuid(), Socket: in.GetIdentifier(), Status: "Pending", Url: "", DocumentId: in.GetDocumentId(), TrackingUrl: in.GetTrackingUrl(), AdTitle: in.GetAdTitle(), ClickthroughUrl: in.GetClickthroughUrl(), CallToAction: in.GetCallToAction(), StartTime: in.GetStartTime(), EndTime: in.GetEndTime(), PlayTime: in.GetPlayTime()})
				if jobProvisioned != "failed" {
					return &adpb.Vast{Status: "Good", ID: in.GetUuid(), Socket: in.GetIdentifier()}, nil
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
			// log.Printf("Received Sms: %v, %v, %v, %v, %v", in.GetContent(), in.GetFrom(), in.GetUsername(), in.GetIdentifier(), in.GetHash())
			var authenticated bool = security.CheckAuthenticRequest(in.GetUsername(), in.GetIdentifier(), in.GetHash()) // Access mongo and check user identifier against hash to determine if request should be honoured
			if authenticated != false {
				jobProvisioned := sms_queue.ProvisionSmsJob(structs.Msg{Content: in.GetContent(), From: in.GetFrom()})
				if jobProvisioned != "failed" {
					return &pb.Msg{Content: in.GetContent(), From: in.GetFrom(), JobId: jobProvisioned}, nil
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
			// log.Printf("Received Video: %v, %v, %v, %v, %v, %v, %v, %v", in.GetIdentifier(), in.GetUsername(), in.GetSocket(), in.GetDestination(), in.GetFilename(), in.GetPath(), in.GetUuid(), in.GetHash())
			var authenticated bool = security.CheckAuthenticRequest(in.GetUsername(), in.GetIdentifier(), in.GetHash()) // Access mongo and check user identifier against hash to determine if request should be honoured
			if authenticated != false {
				vid := &vpb.Video{Status: "processing", ID: in.GetUuid(), Socket: in.GetSocket(), Destination: "null", Filename: "null", Path: in.GetPath()}
				transcode.UpdateMongoRecord(vid, []structs.MediaItem{}, "waiting", []structs.Thumbnail{}, true) // Build initial record for tracking during processing
				jobProvisioned := video_queue.ProvisionVideoJob(&vpb.Video{Status: "processing", ID: in.GetUuid(), Socket: in.GetSocket(), Destination: in.GetDestination(), Filename: in.GetFilename(), Path: in.GetPath()})
				if jobProvisioned != "failed" {
					return vid, nil
				}
			}
		}
	}
	err := errors.New("Request to Video Service failed")
	return &vpb.Video{Status: "Bad Request", ID: "Tycoon Services", Socket: "null", Destination: "null", Filename: "null", Path: "null"}, err
}

func main() {
	lis, err := net.Listen("tcp", port) // Server for ingesting SMS and general requests
	if err != nil {
		log.Fatalf("Failed to listen on %v: %v", port, err)
	}
	s := grpc.NewServer()
	pb.RegisterSmsManagementServer(s, &SmsManagementServer{})
	go newVideoServer()                  // Server for ingesting video job requests
	go newAdServer()                     // Server for ingesting ad job requests
	go sms_workers.BuildWorkerServer()   // Server for SMS job queue
	go video_workers.BuildWorkerServer() // Server for video job queue
	go ad_workers.BuildWorkerServer()    // Server for ad job queue
	go serveAdCompliantServer()
	go serverCronRoutines() // Initiate server cron routines
	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}

func serveAdCompliantServer() *http.Server {
	http.HandleFunc("/ads/vmap", handleAdRequests)
	http.HandleFunc("/ads/vast", handleAdVastRequests)
	http.HandleFunc("/ads/error", handleAdErrorRequests)
	http.HandleFunc("/ads/view", handleAdViewRequests)
	http.HandleFunc("/ads/track", handleAdTrackRequests)
	log.Printf("Ad Compliant Server listening at %v", adServerPort)
	err := http.ListenAndServe(adServerPort, nil)
	if err != nil {
		log.Fatalf("Failed to run Ad Compliant Server: %v", err)
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
	lis2, err := net.Listen("tcp", videoPort)
	if err != nil {
		log.Fatalf("Failed to listen on %v: %v", videoPort, err)
	}
	v := grpc.NewServer()
	vpb.RegisterVideoManagementServer(v, &VideoManegmentServer{})
	log.Printf("Video Server listening at %v", lis2.Addr())
	if err := v.Serve(lis2); err != nil {
		log.Fatalf("Failed to run Video server: %v", err)
	}
	return grpc.NewServer()
}

func newAdServer() *grpc.Server {
	lis3, err := net.Listen("tcp", adPort)
	if err != nil {
		log.Fatalf("Failed to listen on %v: %v", adPort, err)
	}
	a := grpc.NewServer()
	adpb.RegisterAdManagementServer(a, &AdManagementServer{})
	log.Printf("Ad Server listening at %v", lis3.Addr())
	if err := a.Serve(lis3); err != nil {
		log.Fatalf("Failed to run Ad server: %v", err)
	}
	return grpc.NewServer()
}

func serverCronRoutines() {
	loc, _ := time.LoadLocation("America/New_York")
	s := gocron.NewScheduler(loc)
	job, err := s.Every(1).Day().At("19:30:15").Do(func() {
		currentTime := time.Now()
		log.Printf("CRON %v", currentTime.Format("2006-01-02"))
		ad_queue.AggregateAdAnalytics()
	})
	log.Printf("Job %v Err %v\n", job.NextRun(), err)
	s.StartBlocking()
}
