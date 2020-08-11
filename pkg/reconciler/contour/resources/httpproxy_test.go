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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/net-contour/pkg/reconciler/contour/config"
	networkingpkg "knative.dev/networking/pkg"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/pkg/network"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/reconciler"
)

const (
	publicClass  = "im-public-yo"
	privateClass = "this-is-the-private-class"
)

func TestMakeProxies(t *testing.T) {
	protocol := "h2c"
	serviceToProtocol := map[string]string{
		"goo": protocol,
	}

	tests := []struct {
		name         string
		sec          networkingpkg.HTTPProtocol
		ing          *v1alpha1.Ingress
		want         []*v1.HTTPProxy
		modifyConfig func(*config.Config)
	}{{
		name: "single external domain with split",
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"example.com"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							AppendHeaders: map[string]string{
								"Foo": "bar",
							},
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "goo",
									ServicePort: intstr.FromInt(123),
								},
								Percent: 12,
								AppendHeaders: map[string]string{
									"Baz":   "blah",
									"Bleep": "bloop",
								},
							}, {
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "doo",
									ServicePort: intstr.FromInt(124),
								},
								Percent: 88,
								AppendHeaders: map[string]string{
									"Baz": "blurg",
								},
							}},
						}},
					},
				}},
			},
		},
		want: []*v1.HTTPProxy{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-" + publicClass + "-example.com",
				Labels: map[string]string{
					DomainHashKey: "0caaf24ab1a0c33440c06afe99df986365b0781f",
					GenerationKey: "0",
					ParentKey:     "bar",
					ClassKey:      publicClass,
				},
				Annotations: map[string]string{
					ClassKey: publicClass,
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1.HTTPProxySpec{
				VirtualHost: &v1.VirtualHost{
					Fqdn: "example.com",
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
					},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}, {
							Name:  "K-Network-Hash",
							Value: "99dbaae65d712842149f0be3a930d0e229226f86fadddd36bb7b87b0a38ffd3e",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Port:     123,
						Protocol: &protocol,
						Weight:   12,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blah",
							}, {
								Name:  "Bleep",
								Value: "bloop",
							}},
						},
					}, {
						Name:   "doo",
						Port:   124,
						Weight: 88,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blurg",
							}},
						},
					}},
				}},
			},
		}},
	}, {
		// I'm not thrilled that the domain is special cased here,
		// but those are the current semantics.
		name: "external visibility with internal domain",
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{network.GetServiceHostname("foo", "bar")},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "goo",
									ServicePort: intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}},
					},
				}},
			},
		},
		want: []*v1.HTTPProxy{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-" + privateClass + "-foo.bar",
				Labels: map[string]string{
					DomainHashKey: "336d1b3d72e061b98b59d6c793f6a8da217a727a",
					GenerationKey: "0",
					ParentKey:     "bar",
					ClassKey:      privateClass,
				},
				Annotations: map[string]string{
					ClassKey: privateClass,
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1.HTTPProxySpec{
				VirtualHost: &v1.VirtualHost{
					Fqdn: "foo.bar",
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
					},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "fb6afb0467a7a36edf6d1144ed747e9942a57b82f425e6571113bef5081978b5",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
					}},
				}},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-" + privateClass + "-foo.bar.svc",
				Labels: map[string]string{
					DomainHashKey: "c537bbef14c1570803e5c51c6ca824524c758496",
					GenerationKey: "0",
					ParentKey:     "bar",
					ClassKey:      privateClass,
				},
				Annotations: map[string]string{
					ClassKey: privateClass,
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1.HTTPProxySpec{
				VirtualHost: &v1.VirtualHost{
					Fqdn: "foo.bar.svc",
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
					},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "fb6afb0467a7a36edf6d1144ed747e9942a57b82f425e6571113bef5081978b5",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
					}},
				}},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-" + privateClass + "-foo.bar.svc.cluster.local",
				Labels: map[string]string{
					DomainHashKey: "6f498a962729705e1c12fdef2c3371c00f5094e9",
					GenerationKey: "0",
					ParentKey:     "bar",
					ClassKey:      privateClass,
				},
				Annotations: map[string]string{
					ClassKey: privateClass,
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1.HTTPProxySpec{
				VirtualHost: &v1.VirtualHost{
					Fqdn: "foo.bar.svc.cluster.local",
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
					},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "fb6afb0467a7a36edf6d1144ed747e9942a57b82f425e6571113bef5081978b5",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
					}},
				}},
			},
		}},
	}, {
		name: "cluster local visibility with retry policy and timeout",
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"example.com"},
					Visibility: v1alpha1.IngressVisibilityClusterLocal,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Retries: &v1alpha1.HTTPRetry{
								Attempts:      34,
								PerTryTimeout: &metav1.Duration{14 * time.Minute},
							},
							Timeout: &metav1.Duration{46 * time.Minute},
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "goo",
									ServicePort: intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}},
					},
				}},
			},
		},
		want: []*v1.HTTPProxy{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-" + privateClass + "-example.com",
				Labels: map[string]string{
					DomainHashKey: "0caaf24ab1a0c33440c06afe99df986365b0781f",
					GenerationKey: "0",
					ParentKey:     "bar",
					ClassKey:      privateClass,
				},
				Annotations: map[string]string{
					ClassKey: privateClass,
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1.HTTPProxySpec{
				VirtualHost: &v1.VirtualHost{
					Fqdn: "example.com",
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					RetryPolicy: &v1.RetryPolicy{
						NumRetries:    34,
						PerTryTimeout: "14m0s",
					},
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "46m0s",
					},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "b53cd0662ebbe11c2eebbcab7728ae3992b7e9b7167bed423b2aa10646dc3e8e",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
					}},
				}},
			},
		}},
	}, {
		name: "multiple paths with header conditions",
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"example.com"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Path: "/goo",
							Headers: map[string]v1alpha1.HeaderMatch{
								"tag": {
									Exact: "goo",
								},
							},
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "goo",
									ServicePort: intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}, {
							Path: "/doo",
							Headers: map[string]v1alpha1.HeaderMatch{
								"tag": {
									Exact: "doo",
								},
							},
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "doo",
									ServicePort: intstr.FromInt(124),
								},
								Percent: 100,
							}},
						}},
					},
				}},
			},
		},
		want: []*v1.HTTPProxy{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-" + publicClass + "-example.com",
				Labels: map[string]string{
					DomainHashKey: "0caaf24ab1a0c33440c06afe99df986365b0781f",
					GenerationKey: "0",
					ParentKey:     "bar",
					ClassKey:      publicClass,
				},
				Annotations: map[string]string{
					ClassKey: publicClass,
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1.HTTPProxySpec{
				VirtualHost: &v1.VirtualHost{
					Fqdn: "example.com",
				},
				Routes: []v1.Route{{
					Conditions: []v1.Condition{{
						Prefix: "/goo",
					}, {
						Header: &v1.HeaderCondition{
							Name:  "tag",
							Exact: "goo",
						},
					}},
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
					},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "c464aab77f2a40e9bf60890d0b57471049d2cd5377f99e36d3a0e3906a84ff70",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
					}},
				}, {
					Conditions: []v1.Condition{{
						Prefix: "/doo",
					}, {
						Header: &v1.HeaderCondition{
							Name:  "tag",
							Exact: "doo",
						},
					}},
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
					},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "c464aab77f2a40e9bf60890d0b57471049d2cd5377f99e36d3a0e3906a84ff70",
						}},
					},
					Services: []v1.Service{{
						Name:   "doo",
						Port:   124,
						Weight: 100,
					}},
				}},
			},
		}},
	}, {
		name: "single external domain with TLS, and only TLS",
		sec:  networkingpkg.HTTPRedirected,
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"example.com"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							AppendHeaders: map[string]string{
								"Foo": "bar",
							},
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "goo",
									ServicePort: intstr.FromInt(123),
								},
								Percent: 12,
								AppendHeaders: map[string]string{
									"Baz": "blah",
								},
							}, {
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "doo",
									ServicePort: intstr.FromInt(124),
								},
								Percent: 88,
								AppendHeaders: map[string]string{
									"Baz": "blurg",
								},
							}},
						}},
					},
				}},
				TLS: []v1alpha1.IngressTLS{{
					Hosts:           []string{"example.com"},
					SecretNamespace: "foo",
					SecretName:      "bar",
				}},
			},
		},
		want: []*v1.HTTPProxy{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-" + publicClass + "-example.com",
				Labels: map[string]string{
					DomainHashKey: "0caaf24ab1a0c33440c06afe99df986365b0781f",
					GenerationKey: "0",
					ParentKey:     "bar",
					ClassKey:      publicClass,
				},
				Annotations: map[string]string{
					ClassKey: publicClass,
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1.HTTPProxySpec{
				VirtualHost: &v1.VirtualHost{
					Fqdn: "example.com",
					TLS: &v1.TLS{
						SecretName: "foo/bar",
					},
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   false,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
					},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}, {
							Name:  "K-Network-Hash",
							Value: "7b03a20b9872f4e43fb6ab07c484ba4f6701d838bccfea40e49c02ac074ecf33",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   12,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blah",
							}},
						},
					}, {
						Name:   "doo",
						Port:   124,
						Weight: 88,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blurg",
							}},
						},
					}},
				}},
			},
		}},
	}, {
		name: "single external domain with TLS, but allowing non-TLS",
		sec:  networkingpkg.HTTPEnabled,
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"example.com"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							AppendHeaders: map[string]string{
								"Foo": "bar",
							},
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "goo",
									ServicePort: intstr.FromInt(123),
								},
								Percent: 12,
								AppendHeaders: map[string]string{
									"Baz": "blah",
								},
							}, {
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "doo",
									ServicePort: intstr.FromInt(124),
								},
								Percent: 88,
								AppendHeaders: map[string]string{
									"Baz": "blurg",
								},
							}},
						}},
					},
				}},
				TLS: []v1alpha1.IngressTLS{{
					Hosts:           []string{"example.com"},
					SecretNamespace: "foo",
					SecretName:      "bar",
				}},
			},
		},
		want: []*v1.HTTPProxy{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-" + publicClass + "-example.com",
				Labels: map[string]string{
					DomainHashKey: "0caaf24ab1a0c33440c06afe99df986365b0781f",
					GenerationKey: "0",
					ParentKey:     "bar",
					ClassKey:      publicClass,
				},
				Annotations: map[string]string{
					ClassKey: publicClass,
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1.HTTPProxySpec{
				VirtualHost: &v1.VirtualHost{
					Fqdn: "example.com",
					TLS: &v1.TLS{
						SecretName: "foo/bar",
					},
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
					},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}, {
							Name:  "K-Network-Hash",
							Value: "7b03a20b9872f4e43fb6ab07c484ba4f6701d838bccfea40e49c02ac074ecf33",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   12,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blah",
							}},
						},
					}, {
						Name:   "doo",
						Port:   124,
						Weight: 88,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blurg",
							}},
						},
					}},
				}},
			},
		}},
	}, {
		name: "single external domain with default tls secret set by operator",
		modifyConfig: func(c *config.Config) {
			c.Contour.DefaultTLSSecret = &types.NamespacedName{
				Namespace: "some-admin-namespace",
				Name:      "some-secret",
			}
		},
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"example.com"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "goo",
									ServicePort: intstr.FromInt(123),
								},
								Percent: 100,
								AppendHeaders: map[string]string{
									"Baz": "blah",
								},
							}},
						}},
					},
				}},
			},
		},
		want: []*v1.HTTPProxy{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-" + publicClass + "-example.com",
				Labels: map[string]string{
					DomainHashKey: "0caaf24ab1a0c33440c06afe99df986365b0781f",
					GenerationKey: "0",
					ParentKey:     "bar",
					ClassKey:      publicClass,
				},
				Annotations: map[string]string{
					ClassKey: publicClass,
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1.HTTPProxySpec{
				VirtualHost: &v1.VirtualHost{
					Fqdn: "example.com",
					TLS: &v1.TLS{
						SecretName: "some-admin-namespace/some-secret",
					},
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
					},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "f87d6dc22c28a3558c40fc7c774c8656f79011ca70d21103d469c310ac5c0bc7",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blah",
							}},
						},
					}},
				}},
			},
		}},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sec := test.sec
			if sec == "" {
				sec = networkingpkg.HTTPEnabled
			}

			config := &config.Config{
				Contour: &config.Contour{
					VisibilityClasses: map[v1alpha1.IngressVisibility]string{
						v1alpha1.IngressVisibilityClusterLocal: privateClass,
						v1alpha1.IngressVisibilityExternalIP:   publicClass,
					},
				},
				Network: &networkingpkg.Config{
					HTTPProtocol: sec,
				},
			}

			if test.modifyConfig != nil {
				test.modifyConfig(config)
			}

			tcs := &testConfigStore{config: config}
			ctx := tcs.ToContext(context.Background())

			got := MakeHTTPProxies(ctx, test.ing, serviceToProtocol)
			if !cmp.Equal(test.want, got) {
				t.Errorf("MakeHTTPProxies (-want, +got) = %s", cmp.Diff(test.want, got))
			}
		})
	}
}

func TestServiceNames(t *testing.T) {
	tests := []struct {
		name string
		ing  *v1alpha1.Ingress
		want sets.String
	}{{
		name: "empty",
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"example.com"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Path: "/goo",
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "goo",
									ServicePort: intstr.FromInt(123),
								},
								Percent: 12,
							}, {
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "boo",
									ServicePort: intstr.FromInt(124),
								},
								Percent: 88,
							}},
						}, {
							Path: "/doo",
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName: "doo",
									ServicePort: intstr.FromInt(124),
								},
								Percent: 100,
							}},
						}},
					},
				}},
			},
		},
		want: sets.NewString("goo", "boo", "doo"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sns := ServiceNames(context.Background(), test.ing)
			got := make(sets.String, len(sns))
			for key := range sns {
				got.Insert(key)
			}
			if !cmp.Equal(test.want, got) {
				t.Errorf("ServiceNames (-want, +got): %s", cmp.Diff(test.want, got))
			}
		})
	}
}

type testConfigStore struct {
	config *config.Config
}

func (t *testConfigStore) ToContext(ctx context.Context) context.Context {
	return config.ToContext(ctx, t.config)
}

var _ reconciler.ConfigStore = (*testConfigStore)(nil)
