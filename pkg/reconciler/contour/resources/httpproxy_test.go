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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/net-contour/pkg/reconciler/contour/config"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/pkg/network"
	"knative.dev/pkg/ptr"
	"knative.dev/pkg/reconciler"
	servingnetwork "knative.dev/serving/pkg/network"
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
		name string
		sec  servingnetwork.HTTPProtocol
		ing  *v1alpha1.Ingress
		want []*v1.HTTPProxy
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
				Name:      "bar-example.com",
				Labels: map[string]string{
					DomainHashKey: "0caaf24ab1a0c33440c06afe99df986365b0781f",
					GenerationKey: "0",
					ParentKey:     "bar",
				},
				Annotations: map[string]string{
					"projectcontour.io/ingress.class": publicClass,
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
				Name:      "bar-foo.bar",
				Labels: map[string]string{
					DomainHashKey: "336d1b3d72e061b98b59d6c793f6a8da217a727a",
					GenerationKey: "0",
					ParentKey:     "bar",
				},
				Annotations: map[string]string{
					"projectcontour.io/ingress.class": privateClass,
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
				Name:      "bar-foo.bar.svc",
				Labels: map[string]string{
					DomainHashKey: "c537bbef14c1570803e5c51c6ca824524c758496",
					GenerationKey: "0",
					ParentKey:     "bar",
				},
				Annotations: map[string]string{
					"projectcontour.io/ingress.class": privateClass,
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
				Name:      "bar-foo.bar.svc.cluster.local",
				Labels: map[string]string{
					DomainHashKey: "6f498a962729705e1c12fdef2c3371c00f5094e9",
					GenerationKey: "0",
					ParentKey:     "bar",
				},
				Annotations: map[string]string{
					"projectcontour.io/ingress.class": privateClass,
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
				Name:      "bar-example.com",
				Labels: map[string]string{
					DomainHashKey: "0caaf24ab1a0c33440c06afe99df986365b0781f",
					GenerationKey: "0",
					ParentKey:     "bar",
				},
				Annotations: map[string]string{
					"projectcontour.io/ingress.class": privateClass,
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
		name: "multiple paths",
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
								Percent: 100,
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
		want: []*v1.HTTPProxy{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-example.com",
				Labels: map[string]string{
					DomainHashKey: "0caaf24ab1a0c33440c06afe99df986365b0781f",
					GenerationKey: "0",
					ParentKey:     "bar",
				},
				Annotations: map[string]string{
					"projectcontour.io/ingress.class": publicClass,
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
					}},
					EnableWebsockets: true,
					PermitInsecure:   true,
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "7d87a7e5d6e51afb3cdf3148b52cce9c2d209da112df5fb99fed39791b574d4e",
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
					}},
					EnableWebsockets: true,
					PermitInsecure:   true,
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "7d87a7e5d6e51afb3cdf3148b52cce9c2d209da112df5fb99fed39791b574d4e",
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
		sec:  servingnetwork.HTTPRedirected,
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
				Name:      "bar-example.com",
				Labels: map[string]string{
					DomainHashKey: "0caaf24ab1a0c33440c06afe99df986365b0781f",
					GenerationKey: "0",
					ParentKey:     "bar",
				},
				Annotations: map[string]string{
					"projectcontour.io/ingress.class": publicClass,
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
		sec:  servingnetwork.HTTPEnabled,
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
				Name:      "bar-example.com",
				Labels: map[string]string{
					DomainHashKey: "0caaf24ab1a0c33440c06afe99df986365b0781f",
					GenerationKey: "0",
					ParentKey:     "bar",
				},
				Annotations: map[string]string{
					"projectcontour.io/ingress.class": publicClass,
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
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sec := test.sec
			if sec == "" {
				sec = servingnetwork.HTTPEnabled
			}

			tcs := &testConfigStore{
				config: &config.Config{
					Contour: &config.Contour{
						VisibilityClasses: map[v1alpha1.IngressVisibility]string{
							v1alpha1.IngressVisibilityClusterLocal: privateClass,
							v1alpha1.IngressVisibilityExternalIP:   publicClass,
						},
					},
					Network: &servingnetwork.Config{
						HTTPProtocol: sec,
					},
				},
			}
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
			got := ServiceNames(context.Background(), test.ing)
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
