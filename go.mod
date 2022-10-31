module tycoon.systems/tycoon-services

go 1.18

require (
	github.com/go-co-op/gocron v1.17.1
	google.golang.org/grpc v1.48.0
	tycoon.systems/tycoon-services/ad v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/ad/ad_queue v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/ad/ad_queue/workers v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/security v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/sms v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/sms/sms_queue v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/sms/sms_queue/workers v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/structs v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/video v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/video/video_queue v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/video/video_queue/transcode v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/video/video_queue/workers v0.0.0-00010101000000-000000000000
)

require (
	github.com/aws/aws-sdk-go v1.38.20 // indirect
	github.com/aws/aws-sdk-go-v2 v1.16.5 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.4.1 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.15.9 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.12.4 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.5 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.11.14 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.0.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.9.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.1.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.13.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/rekognition v1.18.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/s3 v1.26.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.16.6 // indirect
	github.com/aws/smithy-go v1.11.3 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-redis/redis/v8 v8.11.5 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/hibiken/asynq v0.23.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/montanaflynn/stats v0.0.0-20171201202039-1bf9dbcd8cbe // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/stripe/stripe-go v70.15.0+incompatible // indirect
	github.com/stripe/stripe-go/v73 v73.14.0 // indirect
	github.com/twilio/twilio-go v0.25.0 // indirect
	github.com/u2takey/ffmpeg-go v0.4.1 // indirect
	github.com/u2takey/go-utils v0.3.1 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.1 // indirect
	github.com/xdg-go/stringprep v1.0.3 // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	go.mongodb.org/mongo-driver v1.10.0 // indirect
	golang.org/x/crypto v0.0.0-20220622213112-05595931fe9d // indirect
	golang.org/x/net v0.0.0-20220425223048-2871e0cb64e4 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/sys v0.0.0-20220503163025-988cb79eb6c6 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/genproto v0.0.0-20220503193339-ba3ae3f07e29 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	tycoon.systems/tycoon-services/s3credentials v0.0.0-00010101000000-000000000000 // indirect
	tycoon.systems/tycoon-services/sms/sms_utility v0.0.0-00010101000000-000000000000 // indirect
)

replace tycoon.systems/tycoon-services/s3credentials => ./api/

replace tycoon.systems/tycoon-services/security => ./security/

replace tycoon.systems/tycoon-services/sms/sms_queue => ./sms/sms_queue

replace tycoon.systems/tycoon-services/structs => ./structs/

replace tycoon.systems/tycoon-services/sms/sms_queue/workers => ./sms/sms_queue/workers/

replace tycoon.systems/tycoon-services/video/video_queue/workers => ./video/video_queue/workers/

replace tycoon.systems/tycoon-services/sms/sms_utility => ./sms/sms_utility

replace tycoon.systems/tycoon-services/sms => ./sms

replace tycoon.systems/tycoon-services/video => ./video

replace tycoon.systems/tycoon-services/ad => ./ad

replace tycoon.systems/tycoon-services/video/video_queue => ./video/video_queue

replace tycoon.systems/tycoon-services/video/video_queue/transcode => ./video/video_queue/transcode/

replace tycoon.systems/tycoon-services/ad/ad_queue => ./ad/ad_queue

replace tycoon.systems/tycoon-services/ad/ad_queue/workers => ./ad/ad_queue/workers/
