package workers

import (
	"log"

	"github.com/hibiken/asynq"
	"tycoon.systems/tycoon-services/ad/ad_queue"
	"tycoon.systems/tycoon-services/s3credentials"
)

var (
	jobQueueAddr = s3credentials.GetS3Data("redis", "redishost", "") + ":" + s3credentials.GetS3Data("redis", "tycoon_systems_ad_queue_port", "")
)

func main() {

}

func BuildWorkerServer() {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: jobQueueAddr},
		asynq.Config{
			Concurrency: 10, // total concurrent workers
			Queues: map[string]int{
				"critical": 6,
				"default":  3,
				"low":      1,
			},
		},
	)
	mux := asynq.NewServeMux()
	mux.HandleFunc(ad_queue.TypeVastGenerate, ad_queue.HandleCreateVastCompliantAdVideoTask)
	if err := srv.Run(mux); err != nil {
		log.Printf("Could not run Job Queue Server: %v", err)
	}
}
