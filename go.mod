module tycoon.systems/tycoon-services

go 1.18

require (
	github.com/go-bongo/bongo v0.10.4
	github.com/google/uuid v1.3.0
	google.golang.org/grpc v1.46.0
	google.golang.org/protobuf v1.28.0
	tycoon.systems/tycoon-services/s3credentials v0.0.0-00010101000000-000000000000
)

require (
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/mailru/easyjson v0.0.0-20190626092158-b2ccc519800e // indirect
	github.com/maxwellhealth/go-dotaccess v0.0.0-20190924013105-74ea4f4ca4eb // indirect
	github.com/oleiade/reflections v1.0.1 // indirect
	github.com/smartystreets/goconvey v1.7.2 // indirect
	golang.org/x/net v0.0.0-20220425223048-2871e0cb64e4 // indirect
	golang.org/x/sys v0.0.0-20220503163025-988cb79eb6c6 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20220503193339-ba3ae3f07e29 // indirect
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22 // indirect
	gopkg.in/yaml.v2 v2.2.3 // indirect
)

replace tycoon.systems/tycoon-services/s3credentials => ./api/
