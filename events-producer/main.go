package main

import (
	"context"
	"encoding/json"
	"log"
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
		body, _ := json.Marshal(p)

		if targetTopic != "" {
			if err := client.PublishEvent(ctx, targetName, targetTopic, body); err != nil {
				panic(err)
			}
			log.Printf("Send data to Pubsub %s (topic %s)\n", targetName, targetTopic)
		} else {
			msg := &dapr.InvokeBindingRequest{
				Name:      targetName,
				Operation: "create",
				Data:      body,
			}
			_, err := client.InvokeBinding(ctx, msg)
			if err != nil {
				panic(err)
			}
			log.Printf("Send data to Bindings %s\n", targetName)
		}

		time.Sleep(5 * time.Second)
	}
}

func getEnvVar(key, fallbackValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(val)
	}
	return fallbackValue
}
