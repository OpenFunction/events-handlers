package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
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

func main() {
	t := &TriggerEnvConfig{}
	t.BusComponentName = "trigger"
	t.BusTopic = "default"
	t.Port = "5050"

	subA := &SubscriberConfigs{}
	subA.SinkComponentName = "http-sink"

	subB := &SubscriberConfigs{}
	subB.TopicName = "metrics"

	t.Subscribers = append(t.Subscribers, subA)
	t.Subscribers = append(t.Subscribers, subB)

	tBytes, err := json.Marshal(t)
	if err != nil {
		fmt.Println(err)
	}
	encodeEnvConfig := base64.StdEncoding.EncodeToString(tBytes)
	fmt.Println(encodeEnvConfig)
}
