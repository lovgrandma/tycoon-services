module transcode.go

go 1.18

replace tycoon.systems/tycoon-services/s3credentials => ../../../api/

replace tycoon.systems/tycoon-services/structs => ../../../structs/

replace tycoon.systems/tycoon-services/security => ../../../security

replace tycoon.systems/tycoon-services/sms => ../../../sms

replace tycoon.systems/tycoon-services/video => ../../

require (
	github.com/aws/aws-sdk-go-v2 v1.16.5
	github.com/aws/aws-sdk-go-v2/config v1.15.9
	github.com/aws/aws-sdk-go-v2/credentials v1.12.4
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.11.14
	github.com/aws/aws-sdk-go-v2/service/rekognition v1.18.3
	github.com/aws/aws-sdk-go-v2/service/s3 v1.26.10
	github.com/aws/smithy-go v1.11.3
	github.com/u2takey/ffmpeg-go v0.4.1
	go.mongodb.org/mongo-driver v1.11.2
	tycoon.systems/tycoon-services/s3credentials v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/structs v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/video v0.0.0-00010101000000-000000000000
)

require (
	github.com/aws/aws-sdk-go v1.38.20 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.4.1 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.5 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.12 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.0.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.9.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.1.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.13.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.16.6 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/montanaflynn/stats v0.0.0-20171201202039-1bf9dbcd8cbe // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/u2takey/go-utils v0.3.1 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.1 // indirect
	github.com/xdg-go/stringprep v1.0.3 // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	golang.org/x/crypto v0.0.0-20220622213112-05595931fe9d // indirect
	golang.org/x/net v0.0.0-20211112202133-69e39bad7dc2 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013 // indirect
	google.golang.org/grpc v1.46.2 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
)
