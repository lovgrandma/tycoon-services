package MessageInfo


import (
	"context"
	"log"
	"math/rand"
	"github.com/google/uuid"
	"net"

	pb "tycoon.systems/tycoon-services/sms"
	"google.golang.org/grpc"
)

const (
	port = ":6000"
)

type SmsManagementServer struct {
	ph.UnimplementedSmsManagementServer
}

func (s *SmsManagementServer) CreateNewSmsBlast(ctx context.Context, in *pb.NewMsg) (*pb.Msg, error) {
	log.PrintF("Received: %v, %v", in.GetContent(), in.GetFrom())
	var jobId string = uuid.New()
	return &pb.Msg{content: in.GetContent(), from: in.getFrom(), jobId: jobId }, nil
}

func main() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterSmsManagementServer(s, &SmsManagementServer{})
	log.Printf("Server listening at %v", lis.addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to server: %v", err)
	}
}