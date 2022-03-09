package v2

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	ofctx "github.com/OpenFunction/functions-framework-go/openfunction-context"
	"github.com/dapr/go-sdk/service/common"
	"github.com/golang/glog"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	log "github.com/sirupsen/logrus"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"google.golang.org/protobuf/proto"
)

type Config struct {
	EventBusComponent string                 `json:"eventBusComponent,omitempty"`
	Inputs            []*Input               `json:"inputs,omitempty"`
	Subscribers       map[string]*Subscriber `json:"subscribers,omitempty"`
	LogLevel          string                 `json:"logLevel,omitempty"`
	Port              string                 `json:"port,omitempty"`
}

type Input struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace,omitempty"`
	EventSource string `json:"eventSource"`
	Event       string `json:"event"`
}

type Subscriber struct {
	SinkOutputName       string `json:"sinkOutputName,omitempty"`
	DLSinkOutputName     string `json:"dlSinkOutputName,omitempty"`
	EventBusOutputName   string `json:"eventBusOutputName,omitempty"`
	DLEventBusOutputName string `json:"dlEventBusOutputName,omitempty"`
}

type TriggerMgr struct {
	// key: condition, value: *Subscriber
	ConditionSubscriberMap *sync.Map
	// key: topic, value: *InputStatus
	TopicInputMap *sync.Map
	TopicEventMap map[string]chan *common.TopicEvent
	CelEnv        *cel.Env
}

type InputStatus struct {
	Name        string
	LastMsgTime int64
	LastEvent   *common.TopicEvent
	Status      bool
}

var (
	config         Config
	funcContext    *ofctx.OpenFunctionContext
	triggerManager TriggerMgr
	mainWaitGroup  sync.WaitGroup
)

const (
	// TopicNameTmpl => "{Namespace}-{EventSourceName}-{EventName}"
	TopicNameTmpl     = "%s-%s-%s"
	ResetTimeInterval = 60
)

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

	triggerManager = TriggerMgr{}
	triggerManager.init()

	var inputVars []*exprpb.Decl

	for _, input := range config.Inputs {
		in := input
		topic := in.genTopic()

		// The reset here is equal to the initialization
		triggerManager.reset(topic, in.Name)
		triggerManager.TopicEventMap[topic] = make(chan *common.TopicEvent)
		inputVars = append(inputVars, decls.NewVar(in.Name, decls.Bool))

		// We create a goroutine for each topic (input).
		// This unique goroutine will be awakened by the channel corresponding to the topic.
		mainWaitGroup.Add(1)
		go func(ch <-chan *common.TopicEvent) {
			defer mainWaitGroup.Done()

			// Set a timer ticker to reset the state of the InputStatus corresponding to the topic
			// when no new events are received after the ResetTimeInterval.
			ticker := time.NewTicker(ResetTimeInterval * time.Second)
			defer ticker.Stop()
			event := &common.TopicEvent{}
			for {
				select {
				// When listening for an event (*common.TopicEvent) in the specified channel.
				case event = <-ch:
					// Reset the timer because we received a new message.
					ticker.Reset(ResetTimeInterval * time.Second)
					// Update the value of the corresponding topic in triggerManager.TopicInputMap.
					triggerManager.TopicInputMap.Store(topic, &InputStatus{LastMsgTime: time.Now().Unix(), LastEvent: event, Status: true, Name: in.Name})
					// Check if a condition has been satisfied at this point.
					// If the condition is matched, the corresponding subscriber is obtained and a list (matchSubs) of subscribers matching the above condition is returned.
					if matchSubs := triggerManager.execCondition(); matchSubs != nil {
						// Send event to subscribers according to the configuration in matchSubs.
						triggerManager.execTrigger(event, matchSubs)
					}
				// When the timer ticker ends
				case <-ticker.C:
					// Reset the corresponding state in triggerManager.TopicInputMap,
					// since the event did not match any condition in ResetTimeInterval.
					triggerManager.reset(topic, in.Name)
					break
				}
			}
		}(triggerManager.TopicEventMap[topic])
	}

	for condition, subscriber := range config.Subscribers {
		triggerManager.ConditionSubscriberMap.Store(condition, subscriber)
	}

	env, _ := cel.NewEnv(
		cel.Declarations(inputVars...),
	)
	triggerManager.CelEnv = env
}

func TriggerHandler(ctx *ofctx.OpenFunctionContext, in []byte) ofctx.RetValue {
	funcContext = ctx
	printEventInfo(ctx.Event.TopicEvent)
	triggerManager.TopicEventMap[ctx.Event.TopicEvent.Topic] <- ctx.Event.TopicEvent
	return ctx.ReturnWithSuccess()
}

func trigger(ctx context.Context, e *common.TopicEvent, sub *Subscriber) {

	if sub.SinkOutputName != "" {
		// send the input data to sink
		log.Debugf("trigger - Send to Sink - ID: %s", e.ID)
		response, err := funcContext.Send(sub.SinkOutputName, e.Data.([]byte))
		if err != nil {
			log.Error(err)
			panic(err)
		}
		log.Debugf("trigger - Response - Data: %s", response)
	}
	if sub.EventBusOutputName != "" {
		_, err := funcContext.Send(sub.EventBusOutputName, e.Data.([]byte))
		if err != nil {
			log.Error(err)
			panic(err)
		}
		log.Debugf("trigger - Send to EventBus (filtered topic) - ID: %s", e.ID)
	}
}

func getEnvVar(key, fallbackValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(val)
	}
	return fallbackValue
}

func compile(env *cel.Env, expr string, exprType *exprpb.Type) *cel.Ast {
	ast, iss := env.Compile(expr)
	if iss.Err() != nil {
		glog.Exit(iss.Err())
	}
	if !proto.Equal(ast.ResultType(), exprType) {
		glog.Exitf(
			"Got %v, wanted %v result type",
			ast.ResultType(),
			exprType)
	}
	return ast
}

func isTrue(val ref.Val) bool {
	switch val {
	case types.True:
		return true
	default:
		return false
	}
}

func printEventInfo(e *common.TopicEvent) {
	log.Debugln("---------- ** Events Information ** ----------")
	log.Debugf("ID: %s\nTopic: %s\nType: %s\nSource: %s\nSubject: %s\n", e.ID, e.Topic, e.Type, e.Source, e.Subject)
	log.Debugln("----------------------------------------------")
}

func (t *TriggerMgr) reset(topic string, input string) {
	t.TopicInputMap.Store(topic, &InputStatus{LastMsgTime: 0, LastEvent: nil, Status: false, Name: input})
}

func (t *TriggerMgr) getInputStatuses() map[string]interface{} {
	statuses := map[string]interface{}{}
	t.TopicInputMap.Range(func(k, v interface{}) bool {
		if input, ok := v.(*InputStatus); ok {
			statuses[input.Name] = input.Status
		}
		return true
	})
	return statuses
}

func (t *TriggerMgr) execCondition() []*Subscriber {
	var matchSubscribers []*Subscriber
	t.ConditionSubscriberMap.Range(func(k, v interface{}) bool {
		ast := compile(t.CelEnv, k.(string), decls.Bool)
		program, _ := t.CelEnv.Program(ast)
		inputStatuses := t.getInputStatuses()
		out, _, _ := program.Eval(inputStatuses)
		if isTrue(out) {
			if subscriber, ok := v.(*Subscriber); ok {
				matchSubscribers = append(matchSubscribers, subscriber)
			}
		}
		return true
	})
	return matchSubscribers
}

func (t *TriggerMgr) execTrigger(event *common.TopicEvent, subscribers []*Subscriber) {
	ctx := context.Background()
	for _, subscriber := range subscribers {
		trigger(ctx, event, subscriber)
	}
}

func (t *TriggerMgr) init() {
	t.TopicInputMap = &sync.Map{}
	t.TopicEventMap = map[string]chan *common.TopicEvent{}
	t.ConditionSubscriberMap = &sync.Map{}
}

func (i *Input) genTopic() string {
	return fmt.Sprintf(TopicNameTmpl, i.Namespace, i.EventSource, i.Event)
}
