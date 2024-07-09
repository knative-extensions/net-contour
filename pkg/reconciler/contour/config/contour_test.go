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

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/system"

	. "knative.dev/pkg/configmap/testing"
	_ "knative.dev/pkg/system/testing"
)

func TestContour(t *testing.T) {
	cm, example := ConfigMapsFromTestFile(t, ContourConfigName)

	if _, err := NewContourFromConfigMap(cm); err != nil {
		t.Error("NewContourFromConfigMap(actual) =", err)
	}

	if _, err := NewContourFromConfigMap(example); err != nil {
		t.Error("NewContourFromConfigMap(example) =", err)
	}
}

func TestCORSPolicy(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace(),
			Name:      ContourConfigName,
		},
		Data: map[string]string{
			corsPolicy: `
allowCredentials: true
allowOrigin:
  - "*"
allowMethods:
  - GET
  - POST
  - OPTIONS
allowHeaders:
  - authorization
  - cache-control
exposeHeaders:
  - Content-Length
  - Content-Range
maxAge: "10m"
`,
		},
	}

	cfg, err := NewContourFromConfigMap(cm)
	if err != nil {
		t.Error("NewContourFromConfigMap(corsPolicy) =", err)
		t.FailNow()
	}

	want := &v1.CORSPolicy{
		AllowCredentials: true,
		AllowOrigin:      []string{"*"},
		AllowMethods:     []v1.CORSHeaderValue{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []v1.CORSHeaderValue{"authorization", "cache-control"},
		ExposeHeaders:    []v1.CORSHeaderValue{"Content-Length", "Content-Range"},
		MaxAge:           "10m",
	}
	got := cfg.CORSPolicy
	if !cmp.Equal(got, want) {
		t.Errorf("Got = %v, want: %v, diff:\n%s", got, want, cmp.Diff(got, want))
	}
}
func TestCORSPolicyConfigurationErrors(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
		config  *corev1.ConfigMap
	}{{
		name:    "failure parsing yaml",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				corsPolicy: "moo",
			},
		},
	}, {
		name:    "wrong type",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				corsPolicy: `
allowCredentials: 3
allowOrigin: true
allowMethods: "moo"
allowHeaders:
  - authorization
  - cache-control
exposeHeaders:
  - Content-Length
  - Content-Range
maxAge: "10m"
`,
			},
		},
	}, {
		name:    "incomplete configuration",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				corsPolicy: `
allowHeaders:
  - authorization
  - cache-control
exposeHeaders:
  - Content-Length
  - Content-Range
maxAge: "10m"
`,
			},
		},
	}, {
		name:    "empty value",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				corsPolicy: `
allowCredentials: false
allowOrigin: []
allowMethods:
  - GET
  - POST
  - OPTIONS
allowHeaders:
  - authorization
  - cache-control
exposeHeaders:
  - Content-Length
  - Content-Range
maxAge: "10m"
`,
			},
		},
	}, {
		name:    "empty configuration",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				corsPolicy: "",
			},
		},
	}, {
		name:    "wrong option",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				corsPolicy: `
allowCredentials: true
allowOrigin:
  - "*"
allowMethods:
  - ((GET))
  - POST
  - OPTIONS
allowHeaders:
  - authorization
  - cache-control
exposeHeaders:
  - Content-Length
  - Content-Range
maxAge: "10m"
`,
			},
		},
	}, {
		name:    "invalid duration",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				corsPolicy: `
allowCredentials: true
allowOrigin:
  - "*"
allowMethods:
  - GET
  - POST
  - OPTIONS
allowHeaders:
  - authorization
  - cache-control
exposeHeaders:
  - Content-Length
  - Content-Range
maxAge: "10"
`,
			},
		},
	}, {
		name:    "invalid duration",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				corsPolicy: `
allowCredentials: true
allowOrigin:
  - "*"
allowMethods:
  - GET
  - POST
  - OPTIONS
allowHeaders:
  - authorization
  - cache-control
exposeHeaders:
  - Content-Length
  - Content-Range
maxAge: "-2ms"
`,
			},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewContourFromConfigMap(tt.config)
			t.Log(err)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Test: %q; NewContourFromConfigMap() error = %v, WantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestDefaultTLSSecret(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace(),
			Name:      ContourConfigName,
		},
		Data: map[string]string{
			"default-tls-secret": "some-namespace/some-secret",
		},
	}

	cfg, err := NewContourFromConfigMap(cm)
	if err != nil {
		t.Error("NewContourFromConfigMap(enable-fallback-certificate:true) =", err)
	}

	want := types.NamespacedName{Namespace: "some-namespace", Name: "some-secret"}
	if got := cfg.DefaultTLSSecret; got == nil || *got != want {
		t.Errorf("TLSDefaultSecretName got %q want %q", got, want)
	}

	delete(cm.Data, "default-tls-secret")

	cfg, err = NewContourFromConfigMap(cm)
	if err != nil {
		t.Error("NewContourFromConfigMap(enable-fallback-certificate:false) =", err)
	}

	if cfg.DefaultTLSSecret != nil {
		t.Errorf("TLSDefaultSecretName got %q - want empty", cfg.DefaultTLSSecret)
	}

	// this always requires a namespace
	cm.Data["default-tls-secret"] = "error-name"

	_, err = NewContourFromConfigMap(cm)
	if err == nil {
		t.Errorf("expected an error parsing erroneous 'default-tls-secret'")
	}
}

func TestTimeoutPolicyResponse(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace(),
			Name:      ContourConfigName,
		},
		Data: map[string]string{
			"timeout-policy-response": "60s",
		},
	}

	cfg, err := NewContourFromConfigMap(cm)
	if err != nil {
		t.Error("NewContourFromConfigMap(timeout-policy-response:60s) =", err)
	}

	if got, want := cfg.TimeoutPolicyResponse, "60s"; got != want {
		t.Errorf("TimeoutPolicyResponse got %q want %q", got, want)
	}

	cm.Data["timeout-policy-response"] = "infinity"
	cfg, err = NewContourFromConfigMap(cm)
	if err != nil {
		t.Error("NewContourFromConfigMap(timeout-policy-response:infinity) =", err)
	}

	if got, want := cfg.TimeoutPolicyResponse, "infinity"; got != want {
		t.Errorf("TimeoutPolicyResponse got %q want %q", got, want)
	}

	delete(cm.Data, "timeout-policy-response")
	cfg, err = NewContourFromConfigMap(cm)
	if err != nil {
		t.Error("NewContourFromConfigMap(timeout-policy-response:60s) =", err)
	}

	if cfg.TimeoutPolicyResponse != "infinity" {
		t.Errorf("TimeoutPolicyResponse got %q - want empty", cfg.TimeoutPolicyResponse)
	}

	// format should be as per time.ParseDuration
	cm.Data["timeout-policy-response"] = "60"

	_, err = NewContourFromConfigMap(cm)
	if err == nil {
		t.Errorf("expected an error parsing erroneous 'timeout-policy-response'")
	}

	// This should be "infinity"
	cm.Data["timeout-policy-response"] = "xyz"

	_, err = NewContourFromConfigMap(cm)
	if err == nil {
		t.Errorf("expected an error parsing erroneous 'timeout-policy-response'")
	}
}

func TestTimeoutPolicyIdle(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace(),
			Name:      ContourConfigName,
		},
		Data: map[string]string{
			"timeout-policy-idle": "60s",
		},
	}

	cfg, err := NewContourFromConfigMap(cm)
	if err != nil {
		t.Error("NewContourFromConfigMap(timeout-policy-idle:60s) =", err)
	}

	if got, want := cfg.TimeoutPolicyIdle, "60s"; got != want {
		t.Errorf("TimeoutPolicyIdle got %q want %q", got, want)
	}

	cm.Data["timeout-policy-idle"] = "infinity"
	cfg, err = NewContourFromConfigMap(cm)
	if err != nil {
		t.Error("NewContourFromConfigMap(timeout-policy-idle:infinity) =", err)
	}

	if got, want := cfg.TimeoutPolicyIdle, "infinity"; got != want {
		t.Errorf("TimeoutPolicyIdle got %q want %q", got, want)
	}
	delete(cm.Data, "timeoutPolicy-idle")

	cfg, err = NewContourFromConfigMap(cm)
	if err != nil {
		t.Error("NewContourFromConfigMap(timeout-policy-idle:60s) =", err)
	}

	if cfg.TimeoutPolicyIdle != "infinity" {
		t.Errorf("TimeoutPolicyIdle got %q - want empty", cfg.TimeoutPolicyIdle)
	}

	// format should be as per time.ParseDuration
	cm.Data["timeout-policy-idle"] = "60"

	_, err = NewContourFromConfigMap(cm)
	if err == nil {
		t.Errorf("expected an error parsing erroneous 'timeout-policy-idle'")
	}

	// This should be "infinity"
	cm.Data["timeout-policy-idle"] = "xyz"

	_, err = NewContourFromConfigMap(cm)
	if err == nil {
		t.Errorf("expected an error parsing erroneous 'timeout-policy-idle'")
	}
}

func TestConfigurationErrors(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
		config  *corev1.ConfigMap
	}{{
		name:    "no errors",
		wantErr: false,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				visibilityConfigKey: `
ExternalIP:
  service: foo/bar
  class: baz
ClusterLocal:
  service: blah/bleh
  class: bloop`,
			},
		},
	}, {
		name:    "failure parsing yaml",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				visibilityConfigKey: `
ExternalIP:
  service: foo/bar
  class: baz
ClusterLocal: bad-yaml`,
			},
		},
	}, {
		name:    "missing cluster local",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				visibilityConfigKey: `
ExternalIP:
  service: foo/bar
  class: baz`,
			},
		},
	}, {
		name:    "missing external",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				visibilityConfigKey: `
ClusterLocal:
  service: foo/bar
  class: baz`,
			},
		},
	}, {
		name:    "extra visibility",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				visibilityConfigKey: `
ExternalIP:
  service: foo/bar
  class: baz
ClusterLocal:
  service: blah/bleh
  class: bloop
extra:
  service: blah/bleh
  class: bloop`,
			},
		},
	}, {
		name:    "bad key",
		wantErr: true,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: system.Namespace(),
				Name:      ContourConfigName,
			},
			Data: map[string]string{
				visibilityConfigKey: `
ExternalIP:
  service: foo/bar/extra
  class: baz
ClusterLocal:
  service: blah/bleh
  class: bloop`,
			},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewContourFromConfigMap(tt.config)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Test: %q; NewContourFromConfigMap() error = %v, WantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}
