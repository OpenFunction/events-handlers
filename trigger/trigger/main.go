package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/dapr/go-sdk/service/common"
	daprd "github.com/dapr/go-sdk/service/grpc"
)

var (
	config TriggerEnvConfig
	client dapr.Client
)

type TriggerEnvConfig struct {
	BusComponentName string               `json:"busComponentName"`
	BusTopic         string               `json:"busTopic,omitempty"`
	Subscribers      []*SubscriberConfigs `json:"subscribers,omitempty"`
	Port             string               `json:"port,omitempty"`
}

type SubscriberConfigs struct {
	SinkComponentName           string `json:"sinkComponentName,omitempty"`
	DeadLetterSinkComponentName string `json:"deadLetterSinkComponentName,omitempty"`
	TopicName                   string `json:"topicName,omitempty"`
	DeadLetterTopicName         string `json:"deadLetterTopicName,omitempty"`
}

func init() {
	encodedConfig := getEnvVar("CONFIG", "")
	if len(encodedConfig) > 0 {
		envConifgSpec, err := base64.StdEncoding.DecodeString(encodedConfig)
		if err != nil {
			log.Fatalf("failed to decode config string: %v", err)
		}
		if err = json.Unmarshal(envConifgSpec, &config); err != nil {
			log.Fatalf("failed to unmarshal config object: %v", err)
		}
	}

	if config.BusComponentName == "" {
		log.Fatal("event source cannot be none")
	}

	if config.Port == "" {
		config.Port = "5050"
	}
}

func main() {
	// create a Dapr service server
	s, err := daprd.NewService(fmt.Sprintf(":%s", config.Port))
	if err != nil {
		log.Fatalf("failed to start the server: %v", err)
	}

	// create the client
	client, err = dapr.NewClient()
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// add some topic subscriptions
	sub := &common.Subscription{
		PubsubName: config.BusComponentName,
		Topic:      config.BusTopic,
	}

	// add a binding invocation handler
	if err = s.AddTopicEventHandler(sub, eventBusHandler); err != nil {
		log.Fatalf("error adding topic subscription: %v", err)
	}

	// start the server
	if err = s.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func eventBusHandler(ctx context.Context, e *common.TopicEvent) (retry bool, err error) {
	log.Printf("trigger - Input - EventBus: %s(%s), Topic: %s, ID: %s, Data: %s", config.BusComponentName, e.PubsubName, e.Topic, e.ID, e.Data)

	if client == nil {
		log.Fatal("client is not available")
	}

	for _, sub := range config.Subscribers {
		if sub.SinkComponentName != "" {
			// send the input data to sink
			msg := &dapr.InvokeBindingRequest{
				Name:      sub.SinkComponentName,
				Operation: "create",
				Data:      e.Data.([]byte),
			}

			log.Printf("trigger - Send to Sink - Msg: %v", msg)
			response, err := client.InvokeBinding(ctx, msg)
			if err != nil {
				panic(err)
			}
			log.Printf("trigger - Response - Data: %s, Meta: %v", response.Data, response.Metadata)
		}
		if sub.TopicName != "" {
			if err := client.PublishEventfromCustomContent(ctx, config.BusComponentName, sub.TopicName, e.Data); err != nil {
				panic(err)
			}
			log.Printf("trigger - Send to EventBus (filtered topic) - PubsubName: %s, Topic: %s, Data: %s", config.BusComponentName, sub.TopicName, e.Data)
		}
	}

	return false, nil
}

func getEnvVar(key, fallbackValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(val)
	}
	return fallbackValue
}
