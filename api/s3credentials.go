package s3credentials

import (
	"encoding/json"
	"log"

	// "fmt"
	"reflect"
	"strconv"
)

func main() {

}

func returnRawJsonBytes() []byte {
	jsonData := []byte(`{ 
        "app": {
            "port": 5300,
            "server": "3.22.158.110",
            "prodhost": "127.0.0.1",
            "dev": "true",
            "adServerPort": 6010,
            "sslPath": "../../ssl/live/"
        },
        "awsConfig": {
            "accessKeyId": "AKIAIPCMZT3QFEP2YAXQ",
            "secretAccessKey": "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",
            "region": "us-east-2",
            "rekognitionSnsTopicArnId": "arn:aws:sns:us-east-2:546584803456:TycoonProfanityCheck",
            "rekognitionRoleArnId": "arn:aws:iam::546584803456:role/SimpleRekognitionService",
            "rekognitionSqsQueue": "https://sqs.us-east-2.amazonaws.com/546584803456/TycoonProfanityCheck",
            "rekognitionSqsQueueArn": "arn:aws:sqs:us-east-2:546584803456:TycoonProfanityCheck",
            "buckets": [
                "tycoon-systems-video",
                "tycoon-systems-ads"
            ],
            "devBuckets": [
                "tycoon-systems-video-development",
                "tycoon-systems-ads-development"
            ],
            "mediaBucketLocation1": "us-east-2"
        },
        "cloudFrontKeysPath": {
            "public": "./routes/api/keys/rsa-APKAJ6JGAKCNOGWOEZTA.pem",
            "private": "./routes/api/keys/pk-APKAJ6JGAKCNOGWOEZTA.pem"
        },
        "cdn": {
            "cloudFront1": "https://d3oyqm71scx51z.cloudfront.net"
        },
        "neo": {
            "address": "bolt://127.0.0.1:7687",
            "username": "neo4j",
            "password": "git2003hp7474%"
        },
        "mongo": { 
            "u": "miniup",
            "p": "123abc000zzz",
            "authDb": "admin",
            "address": "mongodb://0.0.0.0:27017/mpst",
            "addressAuth": "mongodb://miniup:123abc000zzz@0.0.0.0:27017/admin",
            "db": "mpst"
        },
        "redis": {
            "redishost": "127.0.0.1",
            "redisport": 6379,
            "videoviewslikesport": 6380,
            "articlereadslikesport": 6381,
            "adviewsport": 6382,
            "dailyadlimitsport": 6383,
            "channelsubscriptionsport": 6384,
            "mpstnumbersport": 6385,
            "tycoon_systems_queue_port": 6386,
            "tycoon_systems_video_queue_port": 6387,
            "tycoon_systems_ad_queue_port": 6388,
            "tycoon_systems_ad_analytics_port": 6389
        },
        "twilio": {
            "sid": "ACdebcfdac964631e37cfb607dfb5204b2",
            "authToken": "b484433b975910c4f672f124bc0cd713",
            "testSid": "AC5d692f13857dd0de1c20042a710a8c22",
            "testAuthToken": "54a2354c42d0a50603c52acac3498277",
            "number1": "+16472772735",
            "testnum": "+15005550006"
        },
        "twilioMpst": {
            "sid": "ACbb5a32d2a5583574a20081e989c7a5d5",
            "authToken": "10d8e3b8d91e74e704f79a24efdc5daf",
            "number1": "+15878472951",
            "verifySid": "VAd7de6af9d6acc780ad2550bad2c25716",
            "passwordResetSid": "VAe19c574e6c956aa74a216963b3071b4f",
            "messagingServiceSid": "MG8acfe8f785fa20d02e4e601c1c1ca934",
            "notifyServicesSid": "ISb2335db9f58b9e7a4140bc8910004dbf"
        },
        "stripe": {
            "testkey": "sk_test_51IEf8CAu2LTYmPsqs4Qhdwu5oQ9QAt9kpc98Jz8bfkx37ub0p1J9W0U1Id584zl4TEsgVdsjvJrXANp4QPI3YQ9E00MBxqsb3C",
            "key": "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",
            "service": {
                "Basic": "price_1KdhTGAu2LTYmPsqxQUgBpjR"
            },
            "testService": {
                "Basic": "price_1KdhlkAu2LTYmPsqSetl1816"
            },
            "analyticsLive": {
                "views_price": "price_1LxkmdAu2LTYmPsq39aQTW7i",
                "clicks_price": "price_1LxkmvAu2LTYmPsqD57zCbic"
            },
            "analyticsDev": {
                "views_price": "price_1Lxki4Au2LTYmPsqWKVYvlx0",
                "clicks_price": "price_1LxkjsAu2LTYmPsqLY5XPhD7"
            }
        },
        "frontend": {
            "local": "localhost:3000",
            "live": "www.minipost.app"
        },
        "filter": {
            "goodVideos": [ "good", "nudegood" ]
        },
        "googleOAuth": {
            "clientId": "169701902623-9a74mqcbqr38uj87qm8tm3190cicaa7m.apps.googleusercontent.com"
        },
        "dev": {
            "auth": "auth891for019dev000env",
            "tycoonSystemsVideo1": "https://d2wj75ner94uq0.cloudfront.net",
            "tycoonSystemsAds1": "https://d266bcgu84hcve.cloudfront.net"
        },
        "prod": {
            "tycoonSystemsVideo1": "https://d1qvcd9vvgazlp.cloudfront.net",
            "tycoonSystemsAds1": "https://d3mc900e7ry112.cloudfront.net"
        }
    }`)
	return jsonData
}

func JsonPointer(data []byte, key string, nested string, nested2 string) string {
	var match string = "nomatch"
	var jsonData interface{}
	err := json.Unmarshal(data, &jsonData)
	if err == nil {
		m := jsonData.(map[string]interface{})
		match = runSwitch(key, nested, nested2, m, -1, "")
	} else {
		log.Printf("Error %v", err)
	}
	return match
}

func GetS3Data(key string, nested string, nested2 string) string {
	var match string = "nomatch"
	var s3Data interface{}
	err := json.Unmarshal(returnRawJsonBytes(), &s3Data)
	if err == nil {
		m := s3Data.(map[string]interface{})
		match = runSwitch(key, nested, nested2, m, -1, "")
	} else {
		log.Printf("Error %v", err)
	}
	return match
}

func runSwitch(key string, nested string, nested2 string, m map[string]interface{}, i int, parent string) string {
	i += 1
	for a, b := range m {
		switch bb := b.(type) {
		case string:
			if i == 0 && a == key {
				return bb
			} else if i == 1 && a == nested && parent == key {
				return bb
			} else if i == 2 && a == nested2 && parent == nested {
				return bb
			}
		case int:
			// fmt.Println("Int", a, bb, i, parent)
			if i == 0 && a == key {
				return strconv.Itoa(bb)
			} else if i == 1 && a == nested && parent == key {
				return strconv.Itoa(bb)
			} else if i == 2 && a == nested2 && parent == nested {
				return strconv.Itoa(bb)
			}
		case float64:
			// fmt.Println("Float", a, bb, i, parent)
			if i == 0 && a == key {
				return strconv.FormatFloat(bb, 'f', 0, 64)
			} else if i == 1 && a == nested && parent == key {
				return strconv.FormatFloat(bb, 'f', 0, 64)
			} else if i == 2 && a == nested2 && parent == nested {
				return strconv.FormatFloat(bb, 'f', 0, 64)
			}
		case []interface{}: // If type array
			if i == 0 && a == key {
				for i = 0; i < len(bb); i++ {
					if bb[i] == key {
						return nested
					}
				}
			} else if i == 1 && a == nested && parent == key {
				for i = 0; i < len(bb); i++ {
					if bb[i] == nested2 {
						return nested2
					}
				}
			}
			continue
		case interface{}:
			// fmt.Println("Interface", a, bb, a, i)
			if reflect.TypeOf(bb).Kind() == reflect.Map {
				n := bb.(map[string]interface{})
				match := runSwitch(key, nested, nested2, n, i, a)
				if match != "nomatch" {
					return match
				}
			}
		default:
			break
		}
	}
	return "nomatch"
}
