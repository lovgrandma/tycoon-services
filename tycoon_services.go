package main

import (
	"context"
	"log"
	// "github.com/google/uuid"
	"net"
	"reflect"
	"tycoon.systems/tycoon-services/security"
	"tycoon.systems/tycoon-services/sms/sms_queue"
	"tycoon.systems/tycoon-services/video/video_queue"
	"tycoon.systems/tycoon-services/structs"
	sms_workers "tycoon.systems/tycoon-services/sms/sms_queue/workers"
	"tycoon.systems/tycoon-services/video/video_queue/transcode"
	video_workers "tycoon.systems/tycoon-services/video/video_queue/workers"
	"errors"

	pb "tycoon.systems/tycoon-services/sms"
	vpb "tycoon.systems/tycoon-services/video"
	"google.golang.org/grpc"
)

const (
	port = ":6000"
	videoPort = ":6002"
)

type SmsManagementServer struct {
	pb.UnimplementedSmsManagementServer
}

type VideoManegmentServer struct {
	vpb.UnimplementedVideoManagementServer
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
					return &pb.Msg{Content: in.GetContent(), From: in.GetFrom(), JobId: jobProvisioned }, nil
				}
			}
		}
	}
	err := errors.New("Request to Sms Service failed")
	return &pb.Msg{Content: "Bad Request", From: "Tycoon Services", JobId: "null" }, err
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
	go newVideoServer() // Server for ingesting video job requests
	go sms_workers.BuildWorkerServer() // Server for SMS job queue
	go video_workers.BuildWorkerServer() // Server for video job queue
	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
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