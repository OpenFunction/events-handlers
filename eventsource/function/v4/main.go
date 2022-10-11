package eventsource

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"

	ofctx "github.com/OpenFunction/functions-framework-go/context"
	"k8s.io/klog/v2"
)

var (
	config Config
)

type Config struct {
	EventBusOutputName string `json:"eventBusOutputName,omitempty"`
	EventBusTopic      string `json:"eventBusTopic,omitempty"`
	SinkOutputName     string `json:"sinkOutputName,omitempty"`
	LogLevel           string `json:"logLevel,omitempty"`
	Port               string `json:"port,omitempty"`
}

func getEnvVar(key, fallbackValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(val)
	}
	return fallbackValue
}

func init() {
	encodedConfig := getEnvVar("CONFIG", "")
	if len(encodedConfig) > 0 {
		envConifgSpec, err := base64.StdEncoding.DecodeString(encodedConfig)
		if err != nil {
			klog.Fatalf("failed to decode config string: %v", err)
		}
		if err = json.Unmarshal(envConifgSpec, &config); err != nil {
			klog.Fatalf("failed to unmarshal config object: %v", err)
		}
	}

	if config.Port == "" {
		config.Port = "5050"
	}

	logLevel := klog.Level(1)
	if config.LogLevel != "" {
		logLevel.Set(config.LogLevel)
	}
}

func EventSourceHandler(ctx ofctx.Context, in []byte) (ofctx.Out, error) {

	if config.EventBusOutputName != "" {
		klog.V(2).Info("eventsource - Send data to EventBus")
		if _, err := ctx.Send(config.EventBusOutputName, in); err != nil {
			klog.Exitf("failed to send data to eventbus: %v", err)
		}
	}
	if config.SinkOutputName != "" {
		klog.V(2).Info("eventsource - Send data to Sink")
		if _, err := ctx.Send(config.SinkOutputName, in); err != nil {
			klog.Exitf("failed to send data to sink: %v", err)
		}
	}

	return ctx.ReturnOnSuccess(), nil
}
