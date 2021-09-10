/*
Copyright 2020 The Knative Authors.

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

package config

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	logtesting "knative.dev/pkg/logging/testing"

	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	. "knative.dev/pkg/configmap/testing"
)

func TestStoreLoadWithContext(t *testing.T) {
	store := NewStore(logtesting.TestLogger(t))

	contourConfig := ConfigMapFromTestFile(t, ContourConfigName)
	store.OnConfigChanged(contourConfig)
	config := FromContext(store.ToContext(context.Background()))

	expectedContour, _ := NewContourFromConfigMap(contourConfig)
	if diff := cmp.Diff(expectedContour, config.Contour); diff != "" {
		t.Error("Unexpected contour config (-want, +got):", diff)
	}
}

func TestStoreImmutableConfig(t *testing.T) {
	store := NewStore(logtesting.TestLogger(t))

	store.OnConfigChanged(ConfigMapFromTestFile(t, ContourConfigName))

	config := store.Load()

	mutated := map[v1alpha1.IngressVisibility]string{
		"foo": "bar",
	}
	config.Contour.VisibilityClasses = mutated

	newConfig := store.Load()

	if cmp.Equal(newConfig.Contour.VisibilityClasses, mutated) {
		t.Error("Contour config is not immutable")
	}
}
