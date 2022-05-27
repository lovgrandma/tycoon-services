package workers

import (
	"tycoon.systems/tycoon-services/s3credentials"
	"tycoon.systems/tycoon-services/video/video_queue"
	"github.com/hibiken/asynq"
	"log"
)

var (
	jobQueueAddr = s3credentials.GetS3Data("redis", "redishost", "") + ":" + s3credentials.GetS3Data("redis", "tycoon_systems_video_queue_port", "")
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
	mux.HandleFunc(video_queue.TypeVideoProcess, video_queue.HandleVideoProcessTask)
	if err := srv.Run(mux); err != nil {
		log.Printf("Could not run Job Queue Server: %v", err)
	}
}