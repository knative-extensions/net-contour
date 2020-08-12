/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"knative.dev/pkg/signals"
)

const defaultTarget = "127.0.0.1:9001"
const intervalMS time.Duration = 500

var currentConfig string
var lastUpdate time.Time = time.Now()

func main() {
	target := os.Getenv("TARGET")
	if target == "" {
		target = defaultTarget
	}

	interval := intervalMS
	if intVal, err := strconv.ParseInt(os.Getenv("INTERVAL_MS"), 10, 64); err == nil {
		interval = time.Duration(intVal)
	}

	ctx := signals.NewContext()

	ticker := time.NewTicker(interval * time.Millisecond)
outer:
	for {
		select {
		case <-ticker.C:
			doDump(ctx, target)
		case <-ctx.Done():
			break outer
		}
	}
}

func doDump(ctx context.Context, target string) {
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s/config_dump?include_eds", target), nil)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("error getting config: %s\n", err)
		return
	}

	defer res.Body.Close()

	config, err := normalizeJSON(res.Body)
	if err != nil {
		log.Printf("error reading & normalizing json: %s", err)
		return
	}

	if currentConfig != config {
		log.Printf("updated config: %s", config)
		currentConfig = config
		lastUpdate = time.Now()
	} else {
		now := time.Now()

		if now.Sub(lastUpdate) > 5*time.Second {
			log.Print("No config update for 5 seconds")
			lastUpdate = now
		}
	}
}

func normalizeJSON(res io.Reader) (string, error) {
	dec := json.NewDecoder(res)

	var raw map[string]interface{}
	if err := dec.Decode(&raw); err != nil {
		return "", err
	}

	bytes, err := json.Marshal(raw)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
