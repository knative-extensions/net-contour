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

package resources

import (
	"context"
	"crypto/sha1" //nolint:gosec // no strong cryptography needed
	"fmt"
	"sort"
	"strconv"
	"strings"

	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/net-contour/pkg/reconciler/contour/config"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/networking/pkg/certificates"
	netcfg "knative.dev/networking/pkg/config"
	netheader "knative.dev/networking/pkg/http/header"
	"knative.dev/networking/pkg/ingress"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/network"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/system"
)

type ServiceInfo struct {
	Port            intstr.IntOrString
	RawVisibilities sets.Set[string]
	// If the Host header sent to this service needs to be rewritten,
	// then track that so we can send it for probing.
	RewriteHost string

	// TODO(https://github.com/knative-sandbox/net-certmanager/issues/44): Remove this.
	HasPath bool
}

func (si *ServiceInfo) Visibilities() (vis []v1alpha1.IngressVisibility) {
	for _, v := range sets.List(si.RawVisibilities) {
		vis = append(vis, v1alpha1.IngressVisibility(v))
	}
	return
}

func ServiceNames(ctx context.Context, ing *v1alpha1.Ingress) map[string]ServiceInfo {
	s := map[string]ServiceInfo{}
	for _, rule := range ing.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			for _, split := range path.Splits {
				si, ok := s[split.ServiceName]
				if !ok {
					si = ServiceInfo{
						Port:            split.ServicePort,
						RawVisibilities: sets.New[string](),
						HasPath:         path.Path != "",
						RewriteHost:     path.RewriteHost,
					}
				}
				si.RawVisibilities.Insert(string(rule.Visibility))
				s[split.ServiceName] = si
			}
		}
	}
	return s
}

func defaultRetryPolicy() *v1.RetryPolicy {
	return &v1.RetryPolicy{
		NumRetries: 2,
		RetryOn: []v1.RetryOn{
			"cancelled",
			"connect-failure",
			"refused-stream",
			"resource-exhausted",
			"retriable-status-codes",

			// In addition to what Istio specifies (above),
			// also retry connection resets.
			"reset",
		},
	}
}

func addHostEntries(entries map[string]v1alpha1.IngressTLS, list []v1alpha1.IngressTLS) {
	for _, tls := range list {
		for _, host := range tls.Hosts {
			entries[host] = tls
		}
	}
}

func tlsEntries(ing *v1alpha1.Ingress) map[string]v1alpha1.IngressTLS {
	external := ing.GetIngressTLSForVisibility(v1alpha1.IngressVisibilityExternalIP)
	internal := ing.GetIngressTLSForVisibility(v1alpha1.IngressVisibilityClusterLocal)

	entries := make(map[string]v1alpha1.IngressTLS, len(external)+len(internal))
	addHostEntries(entries, external)
	addHostEntries(entries, internal)
	return entries
}

func MakeHTTPProxies(ctx context.Context, ing *v1alpha1.Ingress, serviceToProtocol map[string]string) []*v1.HTTPProxy {
	cfg := config.FromContext(ctx)

	ing = ing.DeepCopy()
	ingress.InsertProbe(ing)

	tlsEntries := tlsEntries(ing)

	var allowInsecure bool
	switch ing.Spec.HTTPOption {
	case v1alpha1.HTTPOptionRedirected:
		allowInsecure = false
	case v1alpha1.HTTPOptionEnabled:
		allowInsecure = true
	default:
		allowInsecure = false
	}

	proxies := []*v1.HTTPProxy{}
	for _, rule := range ing.Spec.Rules {
		class := cfg.Contour.VisibilityClasses[rule.Visibility]

		routes := make([]v1.Route, 0, len(rule.HTTP.Paths))
		for _, path := range rule.HTTP.Paths {
			top := &v1.TimeoutPolicy{
				Response: cfg.Contour.TimeoutPolicyResponse,
				Idle:     cfg.Contour.TimeoutPolicyIdle,
			}

			// By default retry on connection problems twice.
			// This matches the default behavior of Istio:
			// https://istio.io/latest/docs/concepts/traffic-management/#retries
			// However, in addition to the codes specified by istio
			retry := defaultRetryPolicy()

			preSplitHeaders := &v1.HeadersPolicy{
				Set: make([]v1.HeaderValue, 0, len(path.AppendHeaders)),
			}
			for key, value := range path.AppendHeaders {
				preSplitHeaders.Set = append(preSplitHeaders.Set, v1.HeaderValue{
					Name:  key,
					Value: value,
				})
			}

			if path.RewriteHost != "" {
				preSplitHeaders.Set = append(preSplitHeaders.Set, v1.HeaderValue{
					Name:  "Host",
					Value: path.RewriteHost,
				})
			}

			// This should never be empty due to the InsertProbe
			sort.Slice(preSplitHeaders.Set, func(i, j int) bool {
				return preSplitHeaders.Set[i].Name < preSplitHeaders.Set[j].Name
			})

			svcs := make([]v1.Service, 0, len(path.Splits))
			for _, split := range path.Splits {
				svc := v1.Service{
					Name:   split.ServiceName,
					Port:   split.ServicePort.IntValue(),
					Weight: int64(split.Percent),
				}

				postSplitHeaders := &v1.HeadersPolicy{
					Set: make([]v1.HeaderValue, 0, len(split.AppendHeaders)),
				}

				hasOriginalHostKey := false
				for key, value := range split.AppendHeaders {
					postSplitHeaders.Set = append(postSplitHeaders.Set, v1.HeaderValue{
						Name:  key,
						Value: value,
					})
					if key == netheader.OriginalHostKey {
						hasOriginalHostKey = true
					}
				}
				if len(postSplitHeaders.Set) > 0 {
					sort.Slice(postSplitHeaders.Set, func(i, j int) bool {
						return postSplitHeaders.Set[i].Name < postSplitHeaders.Set[j].Name
					})
				} else {
					postSplitHeaders = nil
				}

				svc.RequestHeadersPolicy = postSplitHeaders

				if proto, ok := serviceToProtocol[split.ServiceName]; ok {
					// In order for domain mappings to work with internal
					// encryption, need to unencrypt traffic back to the envoy.
					// See
					// https://github.com/knative-sandbox/net-contour/issues/862
					// Can identify domain mappings by the presence of the RewriteHost field on
					// the Path in combination with the "K-Original-Host" key in appendHeaders on
					// the split
					if path.RewriteHost != "" && hasOriginalHostKey {
						svc.Protocol = ptr.String("h2c")
					} else {
						svc.Protocol = ptr.String(proto)
					}
				}

				if cfg.Network != nil && cfg.Network.SystemInternalTLSEnabled() {
					svc.UpstreamValidation = &v1.UpstreamValidation{
						CACertificate: fmt.Sprintf("%s/%s", system.Namespace(), netcfg.ServingRoutingCertName),
						SubjectName:   certificates.DataPlaneUserSAN(ing.Namespace),
						SubjectNames: []string{
							certificates.DataPlaneUserSAN(ing.Namespace),
							certificates.DataPlaneRoutingSAN,
						},
					}
				}

				if strings.Contains(path.Path, HTTPChallengePath) {
					// make sure http01 challenge doesn't get encrypted or use http2
					svc.Protocol = nil
					svc.UpstreamValidation = nil
				}

				svcs = append(svcs, svc)
			}

			var conditions []v1.MatchCondition
			if path.Path != "" {
				conditions = append(conditions, v1.MatchCondition{
					Prefix: path.Path,
				})
			}
			for header, match := range path.Headers {
				conditions = append(conditions, v1.MatchCondition{
					Header: &v1.HeaderMatchCondition{
						Name:  header,
						Exact: match.Exact,
					},
				})
			}

			if len(conditions) > 1 {
				sort.Slice(conditions, func(i, j int) bool {
					hasPrefixLHS := conditions[i].Prefix != ""
					hasPrefixRHS := conditions[j].Prefix != ""
					if hasPrefixLHS && !hasPrefixRHS {
						return true
					}
					if !hasPrefixLHS && hasPrefixRHS {
						return false
					}
					return conditions[i].Header.Name > conditions[j].Header.Name
				})
			}
			ai := allowInsecure
			if rule.Visibility == v1alpha1.IngressVisibilityClusterLocal {
				ai = true
			}

			routes = append(routes, v1.Route{
				Conditions:           conditions,
				TimeoutPolicy:        top,
				RetryPolicy:          retry,
				Services:             svcs,
				EnableWebsockets:     true,
				RequestHeadersPolicy: preSplitHeaders,
				PermitInsecure:       ai,
			})
		}

		base := v1.HTTPProxy{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ing.Namespace,
				Labels: map[string]string{
					GenerationKey: strconv.FormatInt(ing.Generation, 10),
					ParentKey:     ing.Name,
					ClassKey:      class,
				},
				Annotations: map[string]string{
					ClassKey: class,
				},
				OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(ing)},
			},
			Spec: v1.HTTPProxySpec{
				// VirtualHost: filled in below
				Routes: routes,
			},
		}

		for _, originalHost := range rule.Hosts {
			for _, host := range sets.List(ingress.ExpandedHosts(sets.New(originalHost))) {
				hostProxy := base.DeepCopy()

				class := class

				// Ideally these would just be marked ClusterLocal :(
				if strings.HasSuffix(originalHost, network.GetClusterDomainName()) {
					class = cfg.Contour.VisibilityClasses[v1alpha1.IngressVisibilityClusterLocal]
					hostProxy.Annotations[ClassKey] = class
					hostProxy.Labels[ClassKey] = class
				}

				hostProxy.Name = kmeta.ChildName(ing.Name+"-"+class+"-", host)

				hostProxy.Spec.VirtualHost = &v1.VirtualHost{
					Fqdn: host,
				}

				if cfg.Contour.CORSPolicy != nil && rule.Visibility == v1alpha1.IngressVisibilityExternalIP {
					hostProxy.Spec.VirtualHost.CORSPolicy = cfg.Contour.CORSPolicy
				}

				// Set ExtensionService if annotation is present
				if extensionService, ok := ing.Annotations[ExtensionServiceKey]; ok {
					hostProxy.Spec.VirtualHost.Authorization = &v1.AuthorizationServer{}
					hostProxy.Spec.VirtualHost.Authorization.ExtensionServiceRef = v1.ExtensionServiceReference{
						Name: extensionService,
					}

					if extensionServiceNamespace, ok := ing.Annotations[ExtensionServiceNamespaceKey]; ok {
						hostProxy.Spec.VirtualHost.Authorization.ExtensionServiceRef.Namespace = extensionServiceNamespace
					}
				}

				//nolint:gosec // No strong cryptography needed.
				hostProxy.Labels[DomainHashKey] = fmt.Sprintf("%x", sha1.Sum([]byte(host)))

				if tls, ok := tlsEntries[host]; ok {
					// TODO(mattmoor): How do we deal with custom secret schemas?
					hostProxy.Spec.VirtualHost.TLS = &v1.TLS{
						SecretName: fmt.Sprintf("%s/%s", tls.SecretNamespace, tls.SecretName),
					}
				} else if s := cfg.Contour.DefaultTLSSecret; s != nil && rule.Visibility == v1alpha1.IngressVisibilityExternalIP {
					hostProxy.Spec.VirtualHost.TLS = &v1.TLS{SecretName: s.String()}
				}

				proxies = append(proxies, hostProxy)
			}
		}
	}

	return proxies
}
