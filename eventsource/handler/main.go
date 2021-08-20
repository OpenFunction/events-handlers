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
	config EventSourceConfig
	client dapr.Client
)

type EventSourceConfig struct {
	EventSourceComponent string `json:"eventSourceComponent"`
	EventSourceTopic     string `json:"eventSourceTopic,omitempty"`
	EventBusComponent    string `json:"eventBusComponent,omitempty"`
	EventBusTopic        string `json:"eventBusTopic,omitempty"`
	SinkComponent        string `json:"sinkComponent,omitempty"`
	Port                 string `json:"port,omitempty"`
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

	if config.EventSourceComponent == "" {
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

	sub := &common.Subscription{}
	if config.EventSourceTopic != "" {
		// add some topic subscriptions
		sub.PubsubName = config.EventSourceComponent
		sub.Topic = config.EventSourceTopic

		// add a binding invocation handler
		if err = s.AddTopicEventHandler(sub, eventSourceTopicHandler); err != nil {
			log.Fatalf("error adding topic subscription: %v", err)
		}
	} else {
		// add a binding invocation handler
		if err = s.AddBindingInvocationHandler(config.EventSourceComponent, eventSourceBindingsHandler); err != nil {
			log.Fatalf("error adding binding handler: %v", err)
		}
	}

	// start the server
	if err = s.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func eventSourceBindingsHandler(ctx context.Context, in *common.BindingEvent) ([]byte, error) {
	log.Printf("eventsource - Input - Data: %s, Meta: %v", in.Data, in.Metadata)

	if client == nil {
		log.Fatal("client is not available")
	}

	var ret []byte
	if config.SinkComponent != "" {
		// send the input data to sink
		msg := &dapr.InvokeBindingRequest{
			Name:      config.SinkComponent,
			Operation: "create",
			Data:      in.Data,
			Metadata:  in.Metadata,
		}

		log.Printf("eventsource - Send to Sink - Msg: %v", msg)
		response, err := client.InvokeBinding(ctx, msg)
		if err != nil {
			panic(err)
		}
		log.Printf("eventsource - Response - Data: %s, Meta: %v", response.Data, response.Metadata)
		ret = response.Data
	}

	if config.EventBusComponent != "" {
		if err := client.PublishEvent(ctx, config.EventBusComponent, config.EventBusTopic, in.Data); err != nil {
			panic(err)
		}
		log.Printf("eventsource - Send to EventBus - PubsubName: %s, Topic: %s, Data: %s", config.EventBusComponent, config.EventBusTopic, in.Data)
	}
	return ret, nil
}

func eventSourceTopicHandler(ctx context.Context, e *common.TopicEvent) (retry bool, err error) {
	log.Printf("eventsource - Input - EventSource: %s, Topic: %s, ID: %s, Data: %s", e.PubsubName, e.Topic, e.ID, e.Data)

	if client == nil {
		log.Fatal("client is not available")
	}

	if config.SinkComponent != "" {
		// send the input data to sink
		msg := &dapr.InvokeBindingRequest{
			Name:      config.SinkComponent,
			Operation: "create",
			Data:      e.Data.([]byte),
		}

		log.Printf("eventsource - Send to Sink - Msg: %v", msg)
		response, err := client.InvokeBinding(ctx, msg)
		if err != nil {
			panic(err)
		}
		log.Printf("eventsource - Response - Data: %s, Meta: %v", response.Data, response.Metadata)
		return false, nil
	}

	if config.EventBusComponent != "" {
		if err := client.PublishEventfromCustomContent(ctx, config.EventBusComponent, config.EventBusTopic, e.Data); err != nil {
			panic(err)
		}
		log.Printf("eventsource - Send to EventBus - PubsubName: %s, Topic: %s, Data: %s", config.EventBusComponent, config.EventBusTopic, e.Data)
	}
	return false, nil
}

func getEnvVar(key, fallbackValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(val)
	}
	return fallbackValue
}
