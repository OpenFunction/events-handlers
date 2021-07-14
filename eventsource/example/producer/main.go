package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	target     = "http://localhost:3500/v1.0/bindings/%s"
	targetName = getEnvVar("TARGET_NAME", "")
)

type payload struct {
	Data      map[string]interface{} `json:"data"`
	Operation string                 `json:"operation"`
}

func main() {
	url := fmt.Sprintf(target, targetName)

	n := 0

	for {
		n++
		p := &payload{}
		p.Data = map[string]interface{}{"orderId": n, "message": "Hi"}
		p.Operation = "create"

		fmt.Println(p)

		body, _ := json.Marshal(p)
		resp, _ := http.Post(url, "application/json", strings.NewReader(string(body)))
		fmt.Println(resp)

		time.Sleep(5 * time.Second)
	}
}

func getEnvVar(key, fallbackValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(val)
	}
	return fallbackValue
}
