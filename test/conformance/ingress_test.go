//go:build e2e
// +build e2e

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

package conformance

import (
	"strconv"
	"testing"

	// This must come first for init ordering.
	_ "knative.dev/networking/test"

	"knative.dev/networking/test/conformance/ingress"
)

const iterations = 11

func TestIngressConformance(t *testing.T) {
	t.Parallel()

	for i := range iterations {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			ingress.RunConformance(t)
		})

		if testing.Short() {
			break
		}
	}
}
