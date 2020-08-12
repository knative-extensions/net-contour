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
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/signals"
)

const (
	defaultTarget               = "127.0.0.1:9001"
	intervalMS    time.Duration = 500
)

var (
	currentConfig           string
	currentConfigNormalized string
	lastUpdate              time.Time = time.Now()
)

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

	config, normal, err := normalizeJSON(res.Body)
	if err != nil {
		log.Printf("error reading & normalizing json: %s", err)
		return
	}

	if currentConfigNormalized != normal {
		log.Printf("updated config: %s", normal)

		if currentConfig != "" {
			log.Printf("diff: %s", cmp.Diff(currentConfig, config))
		}

		currentConfig = config
		currentConfigNormalized = normal

		lastUpdate = time.Now()
	} else {
		now := time.Now()

		if now.Sub(lastUpdate) > 5*time.Second {
			log.Print("No config update for 5 seconds")
			lastUpdate = now
		}
	}
}

func normalizeJSON(r io.Reader) (string, string, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return "", "", err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", "", err
	}

	norm, err := json.Marshal(raw)
	if err != nil {
		return "", "", err
	}

	// normalize the indent
	indent, err := json.MarshalIndent(raw, " ", " ")
	if err != nil {
		return "", "", err
	}

	return string(indent), string(norm), nil
}
