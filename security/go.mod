module tycoon.systems/tycoon-services/security

go 1.18

require tycoon.systems/tycoon-services/sms/sms_queue v0.0.0-00010101000000-000000000000

require (
	github.com/go-mgo/mgo v0.0.0-20180705113738-7446a0344b78 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/golang/snappy v0.0.1 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/jtolds/gls v4.20.0+incompatible // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/maxwellhealth/go-dotaccess v0.0.0-20190924013105-74ea4f4ca4eb // indirect
	github.com/oleiade/reflections v1.0.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/smartystreets/assertions v1.13.0 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.0.2 // indirect
	github.com/xdg-go/stringprep v1.0.2 // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	go.mongodb.org/mongo-driver v1.9.1 // indirect
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/text v0.3.5 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22 // indirect
	tycoon.systems/tycoon-services/s3credentials v0.0.0-00010101000000-000000000000 // indirect
	tycoon.systems/tycoon-services/structs v0.0.0-00010101000000-000000000000 // indirect
)

replace tycoon.systems/tycoon-services/sms/sms_queue => ../sms/sms_queue/

replace tycoon.systems/tycoon-services/s3credentials => ../api/

replace tycoon.systems/tycoon-services/structs => ../structs/
