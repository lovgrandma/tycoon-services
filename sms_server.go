package main

import (
	"context"
	"log"
	"github.com/google/uuid"
	"net"
	"reflect"
	"tycoon.systems/tycoon-services/s3credentials"
	"errors"

	pb "tycoon.systems/tycoon-services/sms"
	"google.golang.org/grpc"
)

const (
	port = ":6000"
)

type SmsManagementServer struct {
	pb.UnimplementedSmsManagementServer
}

/* Determine if request to create Sms blast is genuine and if user has permissions. Attempt provision for job */
func (s *SmsManagementServer) CreateNewSmsBlast(ctx context.Context, in *pb.NewMsg) (*pb.Msg, error) {
	if reflect.TypeOf(in.GetContent()).Kind() == reflect.String &&
	reflect.TypeOf(in.GetFrom()).Kind() == reflect.String &&
	reflect.TypeOf(in.GetUsername()).Kind() == reflect.String &&
	reflect.TypeOf(in.GetIdentifier()).Kind() == reflect.String &&
	reflect.TypeOf(in.GetHash()).Kind() == reflect.String {
		if len(in.GetContent()) > 0 && len(in.GetFrom()) > 0 && len(in.GetUsername()) > 0 && len(in.GetIdentifier()) > 0 && len(in.GetHash()) > 0 {
			log.Printf("Received: %v, %v, %v, %v, %v", in.GetContent(), in.GetFrom(), in.GetUsername(), in.GetIdentifier(), in.GetHash())
			s3Data := s3credentials.GetS3Data("mongo", "u", "")
			log.Printf("S3 data %v", s3Data)
			// Access mongo and check user identifier against hash to determine if request should be honoured 
			jobId := uuid.NewString()
			return &pb.Msg{Content: in.GetContent(), From: in.GetFrom(), JobId: jobId }, nil
		}
	}
	err := errors.New("Request to Sms Service failed")
	return &pb.Msg{Content: "Bad Request", From: "Tycoon Services", JobId: "null" }, err
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterSmsManagementServer(s, &SmsManagementServer{})
	log.Printf("Server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to server: %v", err)
	}
}