package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type SourceEnvConfig struct {
	SourceComponentName string      `json:"sourceComponentName"`
	SourceTopic         string      `json:"sourceTopic,omitempty"`
	BusConfigs          []BusConfig `json:"busConfigs,omitempty"`
	SinkComponentName   string      `json:"sinkComponentName,omitempty"`
	Port                string      `json:"port,omitempty"`
}

type BusConfig struct {
	BusComponentName string `json:"busComponentName"`
	BusTopic         string `json:"busTopic"`
}

func main() {
	ec := &SourceEnvConfig{}
	ec.SinkComponentName = "http-sink"
	ec.Port = "5050"
	ec.SourceComponentName = "kafka-eventsource"

	defaultEventBus := &BusConfig{}
	defaultEventBus.BusComponentName = "nats-eventbus"
	defaultEventBus.BusTopic = "topic1"

	ec.BusConfigs = append(ec.BusConfigs, *defaultEventBus)

	ecBytes, err := json.Marshal(ec)
	if err != nil {
		fmt.Println(err)
	}
	encodeEnvConfig := base64.StdEncoding.EncodeToString(ecBytes)
	fmt.Println(encodeEnvConfig)
}
