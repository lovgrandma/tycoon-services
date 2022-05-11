module tycoon.systems/tycoon-services

go 1.18

require (
	github.com/google/uuid v1.3.0
	google.golang.org/grpc v1.46.0
	google.golang.org/protobuf v1.28.0
	tycoon.systems/tycoon-services/security v0.0.0-00010101000000-000000000000
)

require (
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.0.2 // indirect
	github.com/xdg-go/stringprep v1.0.2 // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	go.mongodb.org/mongo-driver v1.9.1 // indirect
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	golang.org/x/net v0.0.0-20220425223048-2871e0cb64e4 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/sys v0.0.0-20220503163025-988cb79eb6c6 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20220503193339-ba3ae3f07e29 // indirect
	tycoon.systems/tycoon-services/s3credentials v0.0.0-00010101000000-000000000000 // indirect
	tycoon.systems/tycoon-services/sms/sms_queue v0.0.0-00010101000000-000000000000 // indirect
	tycoon.systems/tycoon-services/structs v0.0.0-00010101000000-000000000000 // indirect
)

replace tycoon.systems/tycoon-services/s3credentials => ./api/

replace tycoon.systems/tycoon-services/security => ./security/

replace tycoon.systems/tycoon-services/sms/sms_queue => ./sms/sms_queue

replace tycoon.systems/tycoon-services/structs => ./structs/
