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

	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
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
	corsPolicy                = "cors-policy"
)

// Contour contains contour related configuration defined in the
// contour config map.
type Contour struct {
	VisibilityKeys        map[v1alpha1.IngressVisibility]sets.Set[string]
	VisibilityClasses     map[v1alpha1.IngressVisibility]string
	DefaultTLSSecret      *types.NamespacedName
	TimeoutPolicyResponse string
	TimeoutPolicyIdle     string
	CORSPolicy            *v1.CORSPolicy
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
	var contourCORSPolicy *v1.CORSPolicy

	if err := configmap.Parse(configMap.Data,
		configmap.AsOptionalNamespacedName(defaultTLSSecretConfigKey, &tlsSecret),
		asContourDuration(timeoutPolicyResponseKey, &timeoutPolicyResponse),
		asContourDuration(timeoutPolicyIdleKey, &timeoutPolicyIdle),
	); err != nil {
		return nil, err
	}

	cors, ok := configMap.Data[corsPolicy]
	if ok {
		if err := yaml.Unmarshal([]byte(cors), &contourCORSPolicy); err != nil {
			return nil, err
		}

		if len(contourCORSPolicy.AllowOrigin) == 0 || len(contourCORSPolicy.AllowMethods) == 0 {
			return nil, fmt.Errorf("the following fields are required but are missing or empty: %s.allowOrigin and %s.allowMethods", corsPolicy, corsPolicy)
		}

		fields := [][]v1.CORSHeaderValue{
			contourCORSPolicy.AllowMethods,
			contourCORSPolicy.AllowHeaders,
			contourCORSPolicy.ExposeHeaders,
		}
		userFriendlyError := []string{corsPolicy + ".allowMethods", corsPolicy + ".allowHeaders", corsPolicy + ".exposeHeaders"}
		for i, field := range fields {
			if len(field) > 0 {
				validOption := regexp.MustCompile("^[a-zA-Z0-9!#$%&'*+.^_`|~-]+$")
				for _, option := range field {
					if !validOption.MatchString(string(option)) {
						return nil, fmt.Errorf("option %q is invalid for %s", option, userFriendlyError[i])
					}
				}
			}
		}

		if len(contourCORSPolicy.MaxAge) > 0 {
			validOption := regexp.MustCompile(`^(((\d*(\.\d*)?h)|(\d*(\.\d*)?m)|(\d*(\.\d*)?s)|(\d*(\.\d*)?ms)|(\d*(\.\d*)?us)|(\d*(\.\d*)?µs)|(\d*(\.\d*)?ns))+|0)$`)
			if !validOption.MatchString(contourCORSPolicy.MaxAge) {
				return nil, fmt.Errorf("%s.maxAge is invalid. Must be 0 or \\d*(h|m|s|ms|us|ns)", corsPolicy)
			}
		}
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
		CORSPolicy:            contourCORSPolicy,
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
