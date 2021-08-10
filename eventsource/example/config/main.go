package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type EventSourceConfig struct {
	EventSourceComponentName string `json:"eventSourceComponentName"`
	EventSourceTopic         string `json:"eventSourceTopic,omitempty"`
	EventBusComponentName    string `json:"eventBusComponentName,omitempty"`
	EventBusTopic            string `json:"eventBusTopic,omitempty"`
	SinkComponentName        string `json:"sinkComponentName,omitempty"`
}

func main() {
	ec := &EventSourceConfig{}
	ec.SinkComponentName = "http-sink"
	ec.EventBusComponentName = "nats-eventbus"
	ec.EventBusTopic = "topic1"
	ec.EventSourceComponentName = "kafka-eventsource"

	ecBytes, err := json.Marshal(ec)
	if err != nil {
		fmt.Println(err)
	}
	encodeEnvConfig := base64.StdEncoding.EncodeToString(ecBytes)
	fmt.Println(encodeEnvConfig)
}
