package eventsource

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"

	ofctx "github.com/OpenFunction/functions-framework-go/openfunction-context"
	log "github.com/sirupsen/logrus"
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
			log.Fatalf("failed to decode config string: %v", err)
		}
		if err = json.Unmarshal(envConifgSpec, &config); err != nil {
			log.Fatalf("failed to unmarshal config object: %v", err)
		}
	}

	if config.Port == "" {
		config.Port = "5050"
	}

	switch config.LogLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.DebugLevel)
	}
}

func EventSourceHandler(ctx *ofctx.OpenFunctionContext, in []byte) ofctx.RetValue {
	var err error
	if config.EventBusOutputName != "" {
		_, err = ctx.Send(config.EventBusOutputName, in)
	}
	if config.SinkOutputName != "" {
		_, err = ctx.Send(config.SinkOutputName, in)
	}
	if err != nil {
		log.Error(err)
		panic(err)
	}

	return ctx.ReturnWithSuccess()
}
