/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"fmt"
	"regexp"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/pkg/configmap"
	"sigs.k8s.io/yaml"
)

const (
	// ContourConfigName is the name of the configmap containing all
	// customizations for contour related features.
	ContourConfigName = "config-contour"

	visibilityConfigKey = "visibility"
	// nolint:gosec // Not an actual secret.
	defaultTLSSecretConfigKey = "default-tls-secret"
	timeoutPolicyIdleKey      = "timeout-policy-idle"
	timeoutPolicyResponseKey  = "timeout-policy-response"
	corsPolicy                = "corsPolicy"
	allowCredentials          = "allowCredentials"
	allowOrigin               = "allowOrigin"
	allowMethods              = "allowMethods"
	allowHeaders              = "allowHeaders"
	exposeHeaders             = "exposeHeaders"
	maxAge                    = "maxAge"
)

// Contour contains contour related configuration defined in the
// contour config map.
type Contour struct {
	VisibilityKeys        map[v1alpha1.IngressVisibility]sets.Set[string]
	VisibilityClasses     map[v1alpha1.IngressVisibility]string
	DefaultTLSSecret      *types.NamespacedName
	TimeoutPolicyResponse string
	TimeoutPolicyIdle     string
	CORSPolicy            *CORSPolicy
}

// TODO (izabelacg) Create type to facilitate field validation? e.g. CORSHeaderValue
type CORSPolicy struct {
	AllowCredentials bool
	AllowOrigin      []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	MaxAge           string
}

type visibilityValue struct {
	Class   string `json:"class"`
	Service string `json:"service"`
}

// NewContourFromConfigMap creates a Contour config from the supplied ConfigMap
func NewContourFromConfigMap(configMap *corev1.ConfigMap) (*Contour, error) {
	var tlsSecret *types.NamespacedName
	var timeoutPolicyResponse = "infinity"
	var timeoutPolicyIdle = "infinity"
	var contourCORSPolicy *CORSPolicy

	if err := configmap.Parse(configMap.Data,
		configmap.AsOptionalNamespacedName(defaultTLSSecretConfigKey, &tlsSecret),
		asContourDuration(timeoutPolicyResponseKey, &timeoutPolicyResponse),
		asContourDuration(timeoutPolicyIdleKey, &timeoutPolicyIdle),
		AsOptionalCORSPolicy(corsPolicy, &contourCORSPolicy),
	); err != nil {
		return nil, err
	}

	v, ok := configMap.Data[visibilityConfigKey]
	if !ok {
		// These are the defaults.
		return &Contour{
			DefaultTLSSecret: tlsSecret,
			VisibilityKeys: map[v1alpha1.IngressVisibility]sets.Set[string]{
				v1alpha1.IngressVisibilityClusterLocal: sets.New("contour-internal/envoy"),
				v1alpha1.IngressVisibilityExternalIP:   sets.New("contour-external/envoy"),
			},
			VisibilityClasses: map[v1alpha1.IngressVisibility]string{
				v1alpha1.IngressVisibilityClusterLocal: "contour-internal",
				v1alpha1.IngressVisibilityExternalIP:   "contour-external",
			},
			TimeoutPolicyResponse: timeoutPolicyResponse,
			TimeoutPolicyIdle:     timeoutPolicyIdle,
			CORSPolicy:            contourCORSPolicy,
		}, nil
	}
	entry := make(map[v1alpha1.IngressVisibility]visibilityValue)
	if err := yaml.Unmarshal([]byte(v), &entry); err != nil {
		return nil, err
	}

	for _, vis := range []v1alpha1.IngressVisibility{
		v1alpha1.IngressVisibilityClusterLocal,
		v1alpha1.IngressVisibilityExternalIP,
	} {
		if _, ok := entry[vis]; !ok {
			return nil, fmt.Errorf("visibility must contain %q with class and service", vis)
		}
	}

	contour := &Contour{
		DefaultTLSSecret:      tlsSecret,
		VisibilityKeys:        make(map[v1alpha1.IngressVisibility]sets.Set[string], 2),
		VisibilityClasses:     make(map[v1alpha1.IngressVisibility]string, 2),
		TimeoutPolicyResponse: timeoutPolicyResponse,
		TimeoutPolicyIdle:     timeoutPolicyIdle,
	}
	for key, value := range entry {
		// Check that the visibility makes sense.
		switch key {
		case v1alpha1.IngressVisibilityClusterLocal, v1alpha1.IngressVisibilityExternalIP:
		default:
			return nil, fmt.Errorf("unrecognized visibility: %q", key)
		}

		// See if the Service is a valid namespace/name token.
		if _, _, err := cache.SplitMetaNamespaceKey(value.Service); err != nil {
			return nil, err
		}
		contour.VisibilityKeys[key] = sets.New(value.Service)
		contour.VisibilityClasses[key] = value.Class
	}
	return contour, nil
}

// AsOptionalCORSPolicy parses the value at key as a CORSPolicy into the target, if it exists
// The nested fields are all required and expected to be set and valid
func AsOptionalCORSPolicy(key string, target **CORSPolicy) configmap.ParseFunc {
	return func(data map[string]string) error {
		if _, ok := data[key]; !ok {
			return nil
		}

		*target = &CORSPolicy{}
		return AsCORSPolicy(key, *target)(data)
	}
}

// AsCORSPolicy parses the value at key as a CORSPolicy into the target, if it exists
// The nested fields are all required and expected to be set and valid
func AsCORSPolicy(key string, target *CORSPolicy) configmap.ParseFunc {
	return func(data map[string]string) error {
		_, ok := data[key]
		if !ok {
			return nil
		}

		err := configmap.AsBool(allowCredentials, &target.AllowCredentials)
		if err != nil {
			// FIXME (izabelacg) how to bubble this up?
			return fmt.Errorf("failed to parse %q", key)
		}
		return nil
	}
}

func asCORSHeadersValue(key string, target *string) configmap.ParseFunc {
	return func(data map[string]string) error {
		if raw, ok := data[key]; ok {
			_, err := regexp.MatchString("^[a-zA-Z0-9!#$%&'*+.^_`|~-]+$", raw)
			if err != nil {
				return fmt.Errorf("failed to parse %q: %w", key, err)
			}
			*target = raw
		}
		return nil
	}
}

func asContourDuration(key string, target *string) configmap.ParseFunc {
	return func(data map[string]string) error {
		if raw, ok := data[key]; ok {
			if raw == "infinity" {
				*target = raw
			} else {
				_, err := time.ParseDuration(raw)
				if err != nil {
					return fmt.Errorf("failed to parse %q: %w", key, err)
				}
				*target = raw
			}
		}
		return nil
	}
}
