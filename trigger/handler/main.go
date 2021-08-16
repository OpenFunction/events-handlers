package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/dapr/go-sdk/service/common"
	daprd "github.com/dapr/go-sdk/service/grpc"
	"github.com/golang/glog"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"google.golang.org/protobuf/proto"
)

type TriggerEnvConfig struct {
	EventBusComponent string                 `json:"eventBusComponent"`
	Inputs            []*Input               `json:"inputs,omitempty"`
	Subscribers       map[string]*Subscriber `json:"subscribers,omitempty"`
	Port              string                 `json:"port,omitempty"`
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

type TriggerManager struct {
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
	config        TriggerEnvConfig
	client        dapr.Client
	tMgr          TriggerManager
	mainWaitGroup sync.WaitGroup
)

const (
	// TopicNameTmpl => "{Namespace}-{EventSourceName}-{EventName}"
	TopicNameTmpl     = "%s-%s-%s"
	ResetTimeInterval = 60
)

func init() {
	encodedConfig := getEnvVar("CONFIG", "")
	if len(encodedConfig) > 0 {
		configSpec, err := base64.StdEncoding.DecodeString(encodedConfig)
		if err != nil {
			log.Fatalf("failed to decode config string: %v", err)
		}
		if err = json.Unmarshal(configSpec, &config); err != nil {
			log.Fatalf("failed to unmarshal config object: %v", err)
		}
	}

	if config.EventBusComponent == "" {
		log.Fatal("eventbus cannot be none")
	}

	if config.Port == "" {
		config.Port = "5050"
	}

	tMgr = TriggerManager{}
	tMgr.init()

	var inputVars []*exprpb.Decl

	for _, input := range config.Inputs {
		in := input
		topic := in.genTopic()

		// The reset here is equal to the initialization
		tMgr.reset(topic, in.Name)
		tMgr.TopicEventMap[topic] = make(chan *common.TopicEvent)
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
					tMgr.TopicInputMap.Store(topic, &InputStatus{LastMsgTime: time.Now().Unix(), LastEvent: event, Status: true, Name: in.Name})
					// Check if a condition has been satisfied at this point.
					// If the condition is matched, the corresponding subscriber is obtained and a list (matchSubs) of subscribers matching the above condition is returned.
					if matchSubs := tMgr.execCondition(in.Name); matchSubs != nil {
						// Send event to subscribers according to the configuration in matchSubs.
						tMgr.execTrigger(event, matchSubs)
					}
				// When the timer ticker ends
				case <-ticker.C:
					// Reset the corresponding state in triggerManager.TopicInputMap,
					// since the event did not match any condition in ResetTimeInterval.
					tMgr.reset(topic, in.Name)
					break
				}
			}
		}(tMgr.TopicEventMap[topic])
	}

	for condition, subscriber := range config.Subscribers {
		tMgr.ConditionSubscriberMap.Store(condition, subscriber)
	}

	env, _ := cel.NewEnv(
		cel.Declarations(inputVars...),
	)
	tMgr.CelEnv = env
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

	// add topic subscriptions
	for _, input := range config.Inputs {
		if input.Namespace == "" {
			input.Namespace = "default"
		}

		subscription := &common.Subscription{
			PubsubName: config.EventBusComponent,
			Topic:      fmt.Sprintf(TopicNameTmpl, input.Namespace, input.EventSource, input.Event),
		}

		if err = s.AddTopicEventHandler(subscription, triggerHandler); err != nil {
			log.Fatalf("error adding topic subscription: %v", err)
		}
	}

	// start the server
	if err = s.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func triggerHandler(ctx context.Context, e *common.TopicEvent) (retry bool, err error) {
	printEventInfo(e)

	tMgr.TopicEventMap[e.Topic] <- e
	return false, nil
}

func trigger(ctx context.Context, e *common.TopicEvent, sub *Subscriber) {

	if sub.SinkComponent != "" {
		// send the input data to sink
		msg := &dapr.InvokeBindingRequest{
			Name:      sub.SinkComponent,
			Operation: "create",
			Data:      e.Data.([]byte),
		}

		log.Printf("trigger - Send to Sink - ID: %s", e.ID)
		response, err := client.InvokeBinding(ctx, msg)
		if err != nil {
			panic(err)
		}
		log.Printf("trigger - Response - Data: %s, Meta: %v", response.Data, response.Metadata)
	}
	if sub.Topic != "" {
		if err := client.PublishEventfromCustomContent(ctx, config.EventBusComponent, sub.Topic, e.Data); err != nil {
			panic(err)
		}
		log.Printf("trigger - Send to EventBus (filtered topic) - PubsubName: %s, Topic: %s, ID: %s", config.EventBusComponent, sub.Topic, e.ID)
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
	log.Println("---------- ** Events Information ** ----------")
	log.Printf("ID: %s\nTopic: %s\nType: %s\nSource: %s\nSubject: %s\n", e.ID, e.Topic, e.Type, e.Source, e.Subject)
	log.Println("----------------------------------------------")
}

func (t *TriggerManager) getTopicOnlyStatus(statuses map[string]interface{}, inputName string) map[string]interface{} {
	topicStatus := map[string]interface{}{}
	for name, _ := range statuses {
		if name != inputName {
			topicStatus[name] = nil
		} else {
			topicStatus[name] = statuses[name]
		}
	}
	return topicStatus
}

func (t *TriggerManager) reset(topic string, input string) {
	t.TopicInputMap.Store(topic, &InputStatus{LastMsgTime: 0, LastEvent: nil, Status: false, Name: input})
}

func (t *TriggerManager) getInputStatuses() map[string]interface{} {
	statuses := map[string]interface{}{}
	t.TopicInputMap.Range(func(k, v interface{}) bool {
		if input, ok := v.(*InputStatus); ok {
			statuses[input.Name] = input.Status
		}
		return true
	})
	return statuses
}

func (t *TriggerManager) execCondition(inputName string) []*Subscriber {
	var matchSubscribers []*Subscriber
	var topicOut ref.Val
	var out ref.Val
	var err error
	t.ConditionSubscriberMap.Range(func(k, v interface{}) bool {
		ast := compile(t.CelEnv, k.(string), decls.Bool)
		program, _ := t.CelEnv.Program(ast)
		statuses := t.getInputStatuses()
		topicStatus := t.getTopicOnlyStatus(statuses, inputName)
		topicOut, _, err = program.Eval(topicStatus)
		if err != nil {
			topicOut = types.Bool(topicStatus[inputName].(bool))
		}
		out, _, err = program.Eval(statuses)
		if err != nil || out == types.Int(0) {
			out = types.False
		}
		if isTrue(out) && isTrue(topicOut) {
			if subscriber, ok := v.(*Subscriber); ok {
				matchSubscribers = append(matchSubscribers, subscriber)
			}
		}
		return true
	})
	return matchSubscribers
}

func (t *TriggerManager) execTrigger(event *common.TopicEvent, subscribers []*Subscriber) {
	ctx := context.Background()
	for _, subscriber := range subscribers {
		trigger(ctx, event, subscriber)
	}
}

func (t *TriggerManager) init() {
	t.TopicInputMap = &sync.Map{}
	t.TopicEventMap = map[string]chan *common.TopicEvent{}
	t.ConditionSubscriberMap = &sync.Map{}
}

func (i *Input) genTopic() string {
	return fmt.Sprintf(TopicNameTmpl, i.Namespace, i.EventSource, i.Event)
}
