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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	. "knative.dev/pkg/configmap/testing"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"
)

func TestContour(t *testing.T) {
	cm, example := ConfigMapsFromTestFile(t, ContourConfigName)

	if _, err := NewContourFromConfigMap(cm); err != nil {
		t.Errorf("NewContourFromConfigMap(actual) = %v", err)
	}

	if _, err := NewContourFromConfigMap(example); err != nil {
		t.Errorf("NewContourFromConfigMap(example) = %v", err)
	}
}

func TestDefaultTLSSecret(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: system.Namespace(),
			Name:      ContourConfigName,
		},
		Data: map[string]string{
			"default-tls-secret-name": "some-namespace/some-secret",
		},
	}

	cfg, err := NewContourFromConfigMap(cm)
	if err != nil {
		t.Errorf("NewContourFromConfigMap(enable-fallback-certificate:true) = %v", err)
	}

	if got, want := cfg.DefaultTLSSecretName, "some-namespace/some-secret"; got != want {
		t.Errorf("TLSDefaultSecretName got %q want %q", got, want)
	}

	delete(cm.Data, "default-tls-secret-name")

	cfg, err = NewContourFromConfigMap(cm)
	if err != nil {
		t.Errorf("NewContourFromConfigMap(enable-fallback-certificate:false) = %v", err)
	}

	if cfg.DefaultTLSSecretName != "" {
		t.Errorf("TLSDefaultSecretName got %q - want empty", cfg.DefaultTLSSecretName)
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
