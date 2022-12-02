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
