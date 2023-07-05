module network_functions

go 1.18

replace tycoon.systems/tycoon-services => ../../../

replace tycoon.systems/tycoon-services/s3credentials => ../../api/

replace tycoon.systems/tycoon-services/structs => ../../structs/

replace tycoon.systems/tycoon-services/network => ./../

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/hibiken/asynq v0.24.1
	google.golang.org/grpc v1.56.1
	tycoon.systems/tycoon-services/network v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/s3credentials v0.0.0-00010101000000-000000000000
	tycoon.systems/tycoon-services/structs v0.0.0-00010101000000-000000000000
)

require (
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/redis/go-redis/v9 v9.0.3 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	go.mongodb.org/mongo-driver v1.11.2 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
)
