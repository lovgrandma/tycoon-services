package security

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"tycoon.systems/tycoon-services/s3credentials"

	"net/http"

	"github.com/go-redis/redis/v8"

	"os"
	"time"

	"google.golang.org/grpc"
	pb "tycoon.systems/tycoon-services/sms"

	"bytes"
	"encoding/json"
)

var (
	db = s3credentials.GetS3Data("mongo", "db", "")

	tycoonSystemsQueuePort = redis.NewClient(&redis.Options{
		Addr:     s3credentials.GetS3Data("redis", "redishost", "") + ":" + s3credentials.GetS3Data("redis", "tycoon_systems_queue_port", ""),
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	returnJobResultPort = s3credentials.GetS3Data("app", "services", "smsServer")
	returnJobResultAddr = s3credentials.GetS3Data("app", "prodhost", "")

	graphqlClient   = &http.Client{}
	graphqlEndpoint = s3credentials.GetS3Data("graphql", "endpoint", "")
)

func GetGraphqlAuth() string {
	useReturnJobResultAddr := returnJobResultAddr
	if os.Getenv("dev") == "true" {
		useReturnJobResultAddr = "localhost"
	}
	conn, err := grpc.Dial(useReturnJobResultAddr+":"+returnJobResultPort, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		fmt.Printf("Err: %v", err)
	}
	c := pb.NewSmsManagementClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := c.GetGraphqlValidMasterLogin(ctx, &pb.Msg{From: "MasterAuth", Content: "", JobId: ""})
	if res != nil && res.Content != "" {
		return res.Content
	}
	return ""
}

func CheckAuthenticRequest(username string, identifier string, hash string, domain string) bool {
	graphqlAuth := GetGraphqlAuth()
	if graphqlAuth != "" {
		query := `
			query FindOneUserp($schemaname: String!, $field: String!, $value: String!) {
				findOneUserp(schemaname: $schemaname, field: $field, value: $value) {
					id
					username
					datecreated
					gId
					email
					numbers
					icon
					payment
					subscriptions
					vendor
				}
			}
		`
		httpClient := &http.Client{}
		// Prepare the GraphQL request payload
		payload := map[string]interface{}{
			"query": query,
			"variables": map[string]string{
				"schemaname": domain,
				"field":      "id",
				"value":      identifier,
			},
		}

		// Convert the payload to JSON
		jsonPayload, _ := json.Marshal(payload)

		req, err := http.NewRequest("POST", graphqlEndpoint, bytes.NewBuffer(jsonPayload))

		if err != nil {
			fmt.Println("Error creating request:", err)
			return false
		}
		// Set the request headers (if required)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-auth-token", graphqlAuth)

		// Send the request
		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Println("Error:", err)
			return false
		}
		defer resp.Body.Close()

		// Read the response body
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		// Print the response
		if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
			fmt.Printf("Err %v", errors)
		} else {
			// response["data"].(map[string]interface{}) // Access Results
			err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(identifier))
			if err == nil {
				return true // Success, authenticated request
			}
		}
	}
	return false
}

func RunGraphqlQuery(payload map[string]interface{}, reqType string, endp string, graphqlAuth string, functionName string) (map[string]interface{}, error) {
	if graphqlAuth == "" {
		graphqlAuth = GetGraphqlAuth()
	}
	if graphqlAuth == "" {
		return nil, fmt.Errorf("Could not get Graphql Auth")
	}
	httpClient := &http.Client{}
	// Convert the payload to JSON
	jsonPayload, _ := json.Marshal(payload)

	req, err := http.NewRequest(reqType, endp, bytes.NewBuffer(jsonPayload))

	if err != nil {
		return nil, fmt.Errorf("Error creating request %v", err)
	}
	// Set the request headers (if required)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-auth-token", graphqlAuth)

	// Send the request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Error creating request %v", err)
	}
	defer resp.Body.Close()

	// Read the response body
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
		fmt.Printf("Err %v", errors)
		return nil, fmt.Errorf("Error %v", errors)
	}
	fmt.Printf("Result %v\n", result)
	if result["data"].(map[string]interface{})[functionName] != nil {
		recordRaw := result["data"].(map[string]interface{})[functionName].(map[string]interface{}) // Access Results
		fmt.Printf("Record Raw %v\n", recordRaw)
		return recordRaw, nil
	}
	return nil, nil
}
