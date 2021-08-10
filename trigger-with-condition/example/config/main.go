package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type TriggerEnvConfig struct {
	BusComponent string                 `json:"busComponent"`
	Inputs       []*Input               `json:"busTopic,omitempty"`
	Subscribers  map[string]*Subscriber `json:"subscribers,omitempty"`
	Port         string                 `json:"port,omitempty"`
}

type Input struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace,omitempty"`
	EventSource string `json:"eventSource"`
	Event       string `json:"event"`
}

type Subscriber struct {
	SinkComponent   string `json:"sinkComponent,omitempty"`
	DLSinkComponent string `json:"deadLetterSinkComponent,omitempty"`
	Topic           string `json:"topic,omitempty"`
	DLTopic         string `json:"deadLetterTopic,omitempty"`
}

func main() {
	t := &TriggerEnvConfig{}
	t.Subscribers = map[string]*Subscriber{}
	t.BusComponent = "trigger"
	t.Port = "5050"

	inputA := &Input{
		Name:        "A",
		Namespace:   "default",
		EventSource: "my-eventsource",
		Event:       "sample-one",
	}
	inputB := &Input{
		Name:        "B",
		Namespace:   "default",
		EventSource: "my-eventsource",
		Event:       "sample-two",
	}
	t.Inputs = append(t.Inputs, inputA)
	t.Inputs = append(t.Inputs, inputB)

	subA := &Subscriber{}
	subA.SinkComponent = "http-sink"

	subB := &Subscriber{}
	subB.Topic = "metrics"

	t.Subscribers["A || B"] = subA
	t.Subscribers["A && B"] = subB

	tBytes, err := json.Marshal(t)
	if err != nil {
		fmt.Println(err)
	}
	encodeEnvConfig := base64.StdEncoding.EncodeToString(tBytes)
	fmt.Println(encodeEnvConfig)
}
