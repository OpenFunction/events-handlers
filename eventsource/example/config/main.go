package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type EventSourceConfig struct {
	EventSourceComponent string `json:"eventSourceComponent"`
	EventSourceTopic     string `json:"eventSourceTopic,omitempty"`
	EventBusComponent    string `json:"eventBusComponent,omitempty"`
	EventBusTopic        string `json:"eventBusTopic,omitempty"`
	SinkComponent        string `json:"sinkComponent,omitempty"`
}

func main() {
	ec := &EventSourceConfig{}
	ec.SinkComponent = "http-sink"
	ec.EventBusComponent = "nats-eventbus"
	ec.EventBusTopic = "topic1"
	ec.EventSourceComponent = "kafka-eventsource"

	ecBytes, err := json.Marshal(ec)
	if err != nil {
		fmt.Println(err)
	}
	encodeEnvConfig := base64.StdEncoding.EncodeToString(ecBytes)
	fmt.Println(encodeEnvConfig)
}
