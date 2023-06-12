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
	returnJobResultPort       = s3credentials.GetS3Data("app", "services", "smsServer")
	returnJobResultAddr       = s3credentials.GetS3Data("app", "prodhost", "")
	routingServicesProd       = s3credentials.GetS3Data("app", "routingServerProd", "")
	routingValidationAuthPort = s3credentials.GetS3Data("app", "routingValidationAuth", "")

	graphqlClient   = &http.Client{}
	graphqlEndpoint = s3credentials.GetS3Data("graphql", "endpoint", "")
)

func GetGraphqlAuth(domain string) string {
	useReturnJobResultAddr := returnJobResultAddr
	useReturnJobResultPort := returnJobResultPort
	var connAddr string
	if os.Getenv("dev") == "true" {
		useReturnJobResultAddr = "localhost"
		if domain != "public" {
			useReturnJobResultPort = routingValidationAuthPort // set to local routing services instance server
		}
		connAddr = useReturnJobResultAddr + ":" + useReturnJobResultPort
	} else {
		connAddr = useReturnJobResultAddr + ":" + useReturnJobResultPort
		if domain != "public" {
			connAddr = routingServicesProd // Set to routing services server
		}
	}
	fmt.Printf("Auth for Graphql Token from: " + connAddr)
	conn, err := grpc.Dial(connAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		fmt.Printf("Err: %v", err)
	}
	c := pb.NewSmsManagementClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := c.GetGraphqlValidMasterLogin(ctx, &pb.Msg{From: "MasterAuth", Content: "", JobId: ""})
	fmt.Printf("Graphql Auth %v %v %v", res, res.Content, err)
	if res != nil && res.Content != "" {
		return res.Content
	}
	return ""
}

func CheckAuthenticRequest(username string, identifier string, hash string, domain string) bool {
	graphqlAuth := GetGraphqlAuth(domain)
	fmt.Println("Auth %v %v %v %v %v", graphqlAuth, username, identifier, hash, domain)
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
			"fetchPolicy": "no-cache",
			"query":       query,
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
			fmt.Printf("Err comparing Passwd %v", err)
			if err == nil {
				return true // Success, authenticated request
			}
		}
	}
	return false
}

func FindUserpByFieldValue(domain string, field string, value string) map[string]interface{} {
	var result map[string]interface{}
	graphqlAuth := GetGraphqlAuth(domain)
	functionName := "findOneUserp"
	if graphqlAuth != "" {
		query := `
			query FindOneUserp($schemaname: String!, $field: String!, $value: String!) {
				findOneUserp(schemaname: $schemaname, field: $field, value: $value) {
					id
					username
					password
					datecreated
					gId
					email
					numbers
					icon
					payment
					subscriptions
					vendor
					key
				}
			}
		`
		httpClient := &http.Client{}
		// Prepare the GraphQL request payload
		payload := map[string]interface{}{
			"fetchPolicy": "no-cache",
			"query":       query,
			"variables": map[string]string{
				"schemaname": domain,
				"field":      field,
				"value":      value,
			},
		}

		// Convert the payload to JSON
		jsonPayload, _ := json.Marshal(payload)

		req, err := http.NewRequest("POST", graphqlEndpoint, bytes.NewBuffer(jsonPayload))

		if err != nil {
			fmt.Println("Error creating request:", err)
			return result
		}
		// Set the request headers (if required)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-auth-token", graphqlAuth)

		// Send the request
		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Println("Error:", err)
			return result
		}
		defer resp.Body.Close()

		// Read the response body
		json.NewDecoder(resp.Body).Decode(&result)

		// Print the response
		if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
			fmt.Printf("Err %v", errors)
		} else {
			if result["data"].(map[string]interface{})[functionName] != nil {
				recordRaw := result["data"].(map[string]interface{})[functionName].(map[string]interface{}) // Access Results
				fmt.Printf("Record Raw %v\n", recordRaw)
				return recordRaw
			}
		}
	}
	var noResult map[string]interface{}
	return noResult
}

func FindLive(domain string, field string, value string, sortField string, sort string, skip string, limit string) map[string]interface{} {
	var result map[string]interface{}
	graphqlAuth := GetGraphqlAuth(domain)
	functionName := "findLive"
	if graphqlAuth != "" {
		query := `
			query FindLive($schemaname: String!, $field: String!, $value: String!, $sortField: String, $sort: String, $skip: String, $limit: String) {
				findLive(schemaname: $schemaname, field: $field, value: $value, sortField: $sortField, sort: $sort, skip: $skip, limit: $limit) {
					id
					author
					status
					publish
					creation
					media
					thumbnail
					thumbtrack
					title
					description
					tags
					production
					cast
					directors
					writers
					timeline
					duration
					staticvideoref
				}
			}
		`
		httpClient := &http.Client{}
		// Prepare the GraphQL request payload
		payload := map[string]interface{}{
			"fetchPolicy": "no-cache",
			"query":       query,
			"variables": map[string]string{
				"schemaname": domain,
				"field":      field,
				"value":      value,
				"sortField":  sortField,
				"sort":       sort,
				"skip":       skip,
				"limit":      limit,
			},
		}

		// Convert the payload to JSON
		jsonPayload, _ := json.Marshal(payload)
		fmt.Printf("Payload %v", payload)
		req, err := http.NewRequest("POST", graphqlEndpoint, bytes.NewBuffer(jsonPayload))

		if err != nil {
			fmt.Println("Error creating request:", err)
			return result
		}
		// Set the request headers (if required)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-auth-token", graphqlAuth)

		// Send the request
		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Println("Error:", err)
			return result
		}
		defer resp.Body.Close()

		// Read the response body
		json.NewDecoder(resp.Body).Decode(&result)

		// Print the response
		if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {
			fmt.Printf("Err %v", errors)
		} else {
			if data, ok := result["data"].(map[string]interface{}); ok {
				if functionNameData, ok2 := data[functionName]; ok2 && functionNameData != nil {
					fmt.Printf("\nValues %v", functionNameData)
					resultSlice, ok := functionNameData.([]interface{})
					if !ok || len(resultSlice) == 0 {
						// Handle the case where the type assertion fails or the slice is empty
						// Return an appropriate value or handle the error
						// For example, you could return an empty slice or an error
						fmt.Printf("Failure")
						return nil
					}

					// Convert the result slice to []map[string]interface{}
					resultMap := make([]map[string]interface{}, len(resultSlice))
					for i, v := range resultSlice {
						if obj, ok := v.(map[string]interface{}); ok {
							resultMap[i] = obj
						} else {
							// Handle the case where an element is not of the expected type
							// Return an appropriate value or handle the error
							// For example, you could return an empty slice or an error
							fmt.Printf("Invalid element type at index %d", i)
							return nil
						}
					}
					return resultMap[0]
				}
			}
		}
	}
	var noResult map[string]interface{}
	return noResult
}

func RunGraphqlQuery(payload map[string]interface{}, reqType string, endp string, graphqlAuth string, functionName string, domain string) (map[string]interface{}, error) {
	fmt.Printf("\n Running Graphql Query %v\n%v\n%v\n%v\n%v\n%v\n", payload, reqType, endp, graphqlAuth, functionName, domain)
	if graphqlAuth == "" {
		graphqlAuth = GetGraphqlAuth(domain)
	}
	if graphqlAuth == "" {
		return nil, fmt.Errorf("Could not get Graphql Auth")
	}
	httpClient := &http.Client{}
	// Convert the payload to JSON
	jsonPayload, _ := json.Marshal(payload)
	fmt.Printf("\nRunning w auth %v %v\n", payload, graphqlAuth)
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
