package network

import (
	context "context"
	"fmt"

	"tycoon.systems/tycoon-services/s3credentials"
	"tycoon.systems/tycoon-services/structs"

	"time"

	"os"

	"google.golang.org/grpc"
	pb "tycoon.systems/tycoon-services/network"
)

var (
	returnJobResultPort           = s3credentials.GetS3Data("app", "services", "smsServer")
	returnJobResultAddr           = s3credentials.GetS3Data("app", "prodhost", "")
	routingServicesProd           = s3credentials.GetS3Data("app", "routingServerProd", "")
	routingValidationAuthEndpoint = s3credentials.GetS3Data("app", "routingValidationAuthEndpoint", "")
)

func main() {

}

func NotifyRoom(msg structs.Msg) {
	// useReturnJobResultAddr := returnJobResultAddr
	// if os.Getenv("dev") == "true" {
	// 	useReturnJobResultAddr = "localhost"
	// }

	useReturnJobResultAddr := returnJobResultAddr
	useReturnJobResultPort := returnJobResultPort
	var connAddr string
	if os.Getenv("dev") == "true" {
		useReturnJobResultAddr = "localhost"
		connAddr = useReturnJobResultAddr + ":" + useReturnJobResultPort
		if msg.Domain != "tycoon" {
			connAddr = routingValidationAuthEndpoint // set to local routing services instance server
		}
	} else {
		connAddr = useReturnJobResultAddr + ":" + useReturnJobResultPort
		if msg.Domain != "tycoon" {
			connAddr = routingServicesProd // Set to routing services server
		}
	}
	fmt.Printf("Conn Addr %v Other %v %v", connAddr, useReturnJobResultAddr, returnJobResultPort)
	conn, err := grpc.Dial(connAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		fmt.Printf("Err: %v", err)
	}
	fmt.Printf("Notify Room %v", msg)
	if err == nil {
		defer conn.Close()
		c := pb.NewNetworkManagementClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		c.NotifyRoom(ctx, &pb.Msg{From: msg.From, Content: msg.Content, JobId: msg.JobId, Domain: msg.Domain, User: msg.User, Function: "NotifyRoom"})
	}
}
