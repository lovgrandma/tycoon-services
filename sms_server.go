package main

import (
	"context"
	"log"
	"github.com/google/uuid"
	"net"
	"tycoon.systems/tycoon-services/s3credentials"

	pb "tycoon.systems/tycoon-services/sms"
	"google.golang.org/grpc"
)

const (
	port = ":6000"
)

type SmsManagementServer struct {
	pb.UnimplementedSmsManagementServer
}

func (s *SmsManagementServer) CreateNewSmsBlast(ctx context.Context, in *pb.NewMsg) (*pb.Msg, error) {
	log.Printf("Received: %v, %v, %v, %v, %v", in.GetContent(), in.GetFrom(), in.GetUsername(), in.GetIdentifier(), in.GetHash());
	s3Data := s3credentials.GetS3Data("frontend", "local", "")
	log.Printf("S3 data %v", s3Data)
	jobId := uuid.NewString()
	return &pb.Msg{Content: in.GetContent(), From: in.GetFrom(), JobId: jobId }, nil
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