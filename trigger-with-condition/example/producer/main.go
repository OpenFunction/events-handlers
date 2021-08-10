package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	dapr "github.com/dapr/go-sdk/client"
)

var (
	targetName  = getEnvVar("TARGET_NAME", "")
	targetTopic = getEnvVar("TARGET_TOPIC", "")
)

type payload struct {
	Data      map[string]interface{} `json:"data"`
	Operation string                 `json:"operation"`
}

func main() {
	// create the client
	client, err := dapr.NewClient()
	if err != nil {
		panic(err)
	}
	defer client.Close()

	ctx := context.Background()

	n := 0

	for {
		n++
		p := &payload{}
		p.Data = map[string]interface{}{"orderId": n, "message": "Hi"}
		p.Operation = "create"

		fmt.Println(p)

		body, _ := json.Marshal(p)
		if err := client.PublishEvent(ctx, targetName, targetTopic, body); err != nil {
			panic(err)
		}
		fmt.Println("data published")

		time.Sleep(5 * time.Second)
	}
}

func getEnvVar(key, fallbackValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(val)
	}
	return fallbackValue
}
