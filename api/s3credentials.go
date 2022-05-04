package s3credentials

import (
    "encoding/json"
    "log"
    "fmt"
    "strconv"
)

func main() {

}

func returnRawJsonBytes() []byte {
    jsonData := []byte(`{ 
        "app": {
            "port": 5300,
            "server": "3.22.158.110"
        },
        "awsConfig": {
            "accessKeyId": "AKIAIPCMZT3QFEP2YAXQ",
            "secretAccessKey": "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",
            "region": "us-east-2",
            "snsTopicArnId": "arn:aws:sns:us-east-2:546584803456:AmazonRekognition",
            "roleArnId": "arn:aws:iam::546584803456:user/rekognitionUserAccess",
            "sqsQueue": "https://sqs.us-east-2.amazonaws.com/546584803456/AmazonRekognition"
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
            "addressAuth": "mongodb://miniup:123abc000zzz@0.0.0.0:27017/admin"
        },
        "redis": {
            "redishost": "127.0.0.1",
            "redisport": 6379,
            "videoviewslikesport": 6380,
            "articlereadslikesport": 6381,
            "adviewsport": 6382,
            "dailyadlimitsport": 6383,
            "channelsubscriptionsport": 6384,
            "mpstnumbersport": 6385
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
            "passwordResetSid": "VAe19c574e6c956aa74a216963b3071b4f"
        },
        "stripe": {
            "testkey": "sk_test_51IEf8CAu2LTYmPsqs4Qhdwu5oQ9QAt9kpc98Jz8bfkx37ub0p1J9W0U1Id584zl4TEsgVdsjvJrXANp4QPI3YQ9E00MBxqsb3C",
            "key": "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",
            "service": {
                "Basic": "price_1KdhTGAu2LTYmPsqxQUgBpjR"
            },
            "testService": {
                "Basic": "price_1KdhlkAu2LTYmPsqSetl1816"
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
        }
    }`)
    return jsonData
}

func GetS3Data(key string, nested string, nested2 string) string {
    var match string = "nomatch"
    var s3Data interface{}
    err := json.Unmarshal(returnRawJsonBytes(), &s3Data)
    if (err == nil) {
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
            fmt.Println("String", a, bb, i, parent)
            if i == 0 && a == key {
                return bb
            } else if (i == 1 && a == nested && parent == key) {
                return bb
            } else if (i == 2 && a == nested2 && parent == nested) {
                return bb
            }
        case int:
            fmt.Println("Int", a, bb, i, parent)
            if i == 0 && a == key {
                return strconv.Itoa(bb)
            } else if (i == 1 && a == nested && parent == key) {
                return strconv.Itoa(bb)
            } else if (i == 2 && a == nested2 && parent == nested) {
                return strconv.Itoa(bb)
            }
        case float64:
            fmt.Println("Float", a, bb, i, parent)
            if i == 0 && a == key {
                return strconv.FormatFloat(bb, 'f', 0, 64)
            } else if (i == 1 && a == nested && parent == key) {
                return strconv.FormatFloat(bb, 'f', 0, 64)
            } else if (i == 2 && a == nested2 && parent == nested) {
                return strconv.FormatFloat(bb, 'f', 0, 64)
            }
        case []interface{}: // If type array
            continue
        case interface{}:
            fmt.Println("Interface", a, bb, a, i)
            n := bb.(map[string]interface{})
            match := runSwitch(key, nested, nested2, n, i, a)
            if (match != "nomatch") {
                return match
            }
        default:
            break
        }
    }
    return "nomatch"
}

func readArray(key string, nested string, array []interface{}, i int, parent string) string {
    for i, u := range array {
        fmt.Println("Nested Array", i, u)
        
    }
    return "nomatch"
}

// var s3Data interface{}
    // err := json.Unmarshal(jsonData, &s3Data)
    // if (err == nil) {
    //     m := s3Data.(map[string]interface{})
    //     // log.Printf("%v", m)
    //     for a, b := range m {
    //         switch bb := b.(type) {
    //         case string:
    //             fmt.Println("String", a, bb)
    //         case int:
    //             fmt.Println("Int", a, bb)
    //         case float64:
    //             fmt.Println("Float", a, bb)
    //         case []interface{}: // If type array
    //             fmt.Println("Interface Array", a)
    //             for i, u := range bb {
    //                 fmt.Println("Array", i, u)
    //                 // if (i == key) {
    //                 //     return i
    //                 // }
    //             }
    //         case interface{}:
    //             fmt.Println("Interface", a)
    //             n := b.(map[string]interface{})
    //             for c, d := range n {
    //                 fmt.Println(c, d)
    //                 switch dd := d.(type) {
    //                 case string:
    //                     fmt.Println("String", c, dd)
    //                 case int:
    //                     fmt.Println("Int", c, dd)
    //                 case float64:
    //                     fmt.Println("Float", c, dd)
    //                 case []interface{}:
    //                     match := readInterface(key, nested, bb)
    //                     if (match != "nomatch") {
    //                         return match
    //                     }
    //                 case interface{}:
    //                     o := dd.(map[string]interface{})
    //                     for e, f := range o {
    //                         fmt.Println("Nested Obj", e, f)
    //                     }
    //                 }
    //             }
    //             fmt.Println("Object v%", n)
    //         default:
    //             fmt.Println(a, "is of a type I don't know how to handle")
    //         }
    //     }
    // } else {
    //     log.Printf("Error %v", err)
    // }

// func doSwitch(key string, nested string, inter map[string]interface{}) string {
//     var match string = "nomatch"
//     for c, d := range inter {
//         fmt.Println(c, d)
//         switch dd := d.(type) {
//         case string:
//             fmt.Println("String", c, dd)
//             if c == key {
//                 return dd
//             }
//         case int:
//             fmt.Println("Int", c, dd)
//             if c == key {
//                 return strconv.Itoa(dd)
//             }
//         case float64:
//             fmt.Println("Float", c, dd)
//             if c == key {
//                 return strconv.FormatFloat(dd, 'E', -1, 32)
//             }
//         case []interface{}:
//             match := readArray(key, nested, dd)
//             if (match != "nomatch") {
//                 return match
//             }
//         case interface{}:
//             o := dd.(map[string]interface{})
//             match := readInterface(key, nested, o)
//             if (match != "nomatch") {
//                 return match
//             }
//         }
//     }
//     return match
// }

// func readInterface(key string, nested string, inter interface{}) string {
//     o := inter.(map[string]interface{})
//     for e, f := range o {
//         fmt.Println("Nested Obj", e, f)
//     }
//     return "nomatch"
// }

// func readArray(key string, nested string, array []interface{}) string {
//     for i, u := range array {
//         fmt.Println("Nested Array", i, u)
//     }
//     return "nomatch"
// }