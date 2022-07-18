module workers.go

go 1.18

replace tycoon.systems/tycoon-services/structs => ../../../structs/

replace tycoon.systems/tycoon-services/ad/ad_queue => ../

replace tycoon.systems/tycoon-services/s3credentials => ../../../api/

require (
	github.com/hibiken/asynq v0.23.0
	tycoon.systems/tycoon-services/ad/ad_queue v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/s3credentials v0.0.0-00010101000000-000000000000
)

require (
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/go-redis/redis/v8 v8.11.2 // indirect
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/montanaflynn/stats v0.0.0-20171201202039-1bf9dbcd8cbe // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.1 // indirect
	github.com/xdg-go/stringprep v1.0.3 // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	go.mongodb.org/mongo-driver v1.10.0 // indirect
	golang.org/x/crypto v0.0.0-20220622213112-05595931fe9d // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	tycoon.systems/tycoon-services/structs v0.0.0-00010101000000-000000000000 // indirect
)
