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
	"fmt"
	"testing"

	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/net-contour/pkg/reconciler/contour/config"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	netcfg "knative.dev/networking/pkg/config"
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
				HTTPOption: v1alpha1.HTTPOptionEnabled,
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
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}, {
							Name:  "K-Network-Hash",
							Value: "418ee51d5bf437558dd840aa1566207fdb00ef57619ed17c0941e4b91d35b63e",
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
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
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
				HTTPOption: v1alpha1.HTTPOptionEnabled,
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
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "2896c1ae04b8417b5b126af0c6402504b9dc2f5c1da745403ef3fb8f6499dd73",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
					}},
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
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
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "2896c1ae04b8417b5b126af0c6402504b9dc2f5c1da745403ef3fb8f6499dd73",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
					}},
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
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
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "2896c1ae04b8417b5b126af0c6402504b9dc2f5c1da745403ef3fb8f6499dd73",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
					}},
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
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
				HTTPOption: v1alpha1.HTTPOptionEnabled,
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
					Conditions: []v1.MatchCondition{{
						Prefix: "/goo",
					}, {
						Header: &v1.HeaderMatchCondition{
							Name:  "tag",
							Exact: "goo",
						},
					}, {
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "dad77757456e5dbbdffc726e056d8515b1216fbd660348b760433559de682061",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
					}},
				}, {
					Conditions: []v1.MatchCondition{{
						Prefix: "/doo",
					}, {
						Header: &v1.HeaderMatchCondition{
							Name:  "tag",
							Exact: "doo",
						},
					}, {
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "dad77757456e5dbbdffc726e056d8515b1216fbd660348b760433559de682061",
						}},
					},
					Services: []v1.Service{{
						Name:   "doo",
						Port:   124,
						Weight: 100,
					}},
				}, {
					Conditions: []v1.MatchCondition{{
						Prefix: "/goo",
					}, {
						Header: &v1.HeaderMatchCondition{
							Name:  "tag",
							Exact: "goo",
						},
					}},
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
					}},
				}, {
					Conditions: []v1.MatchCondition{{
						Prefix: "/doo",
					}, {
						Header: &v1.HeaderMatchCondition{
							Name:  "tag",
							Exact: "doo",
						},
					}},
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
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
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionRedirected,
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
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}, {
							Name:  "K-Network-Hash",
							Value: "eb7c779b7255f1e762100926308e388803f45a8deb1fa17451c87dd56c098ba0",
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
				}, {
					EnableWebsockets: true,
					PermitInsecure:   false,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
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
		name: "internal and external domains, with TLS, with Redirected",
		modifyConfig: func(c *config.Config) {
			c.Contour.DefaultTLSSecret = &types.NamespacedName{
				Namespace: "foo",
				Name:      "bar",
			}
		},
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionRedirected,
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
									ServiceName:      "goo",
									ServiceNamespace: "foo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
								AppendHeaders: map[string]string{
									"Baz": "blurg",
								},
							}},
						}},
					},
				}, {
					Hosts: []string{
						"bar.foo.svc.cluster.local",
					},
					Visibility: v1alpha1.IngressVisibilityClusterLocal,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName:      "goo",
									ServiceNamespace: "foo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
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
					TLS: &v1.TLS{
						SecretName: "foo/bar",
					},
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   false,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}, {
							Name:  "K-Network-Hash",
							Value: "4cd0cf70bba4aecc751fc3264e328ba19c3460161f4499a28e148eb33d7cc16f",
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
								Value: "blurg",
							}},
						},
					}},
				}, {
					EnableWebsockets: true,
					PermitInsecure:   false,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
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
								Value: "blurg",
							}},
						},
					}},
				}},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-" + privateClass + "-bar.foo",
				Labels: map[string]string{
					DomainHashKey: "9cfdfc6963ce12bea7d12be5e91d11d9f8341f9c",
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
					Fqdn: "bar.foo",
					TLS: &v1.TLS{
						SecretName: "foo/bar",
					},
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "4cd0cf70bba4aecc751fc3264e328ba19c3460161f4499a28e148eb33d7cc16f",
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
								Value: "blurg",
							}},
						},
					}},
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blurg",
							}},
						},
					}},
				}},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-" + privateClass + "-bar.foo.svc",
				Labels: map[string]string{
					DomainHashKey: "f9ce2a330aabcf0eb7da1c9d0aa594339f79d454",
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
					Fqdn: "bar.foo.svc",
					TLS: &v1.TLS{
						SecretName: "foo/bar",
					},
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "4cd0cf70bba4aecc751fc3264e328ba19c3460161f4499a28e148eb33d7cc16f",
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
								Value: "blurg",
							}},
						},
					}},
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blurg",
							}},
						},
					}},
				}},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-" + privateClass + "-bar.foo.svc.cluster.local",
				Labels: map[string]string{
					DomainHashKey: "adc2b09a03a391d630bfcc54e3d3f9be36060617",
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
					Fqdn: "bar.foo.svc.cluster.local",
					TLS: &v1.TLS{
						SecretName: "foo/bar",
					},
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "4cd0cf70bba4aecc751fc3264e328ba19c3460161f4499a28e148eb33d7cc16f",
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
								Value: "blurg",
							}},
						},
					}},
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Protocol: &protocol,
						Port:     123,
						Weight:   100,
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
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
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
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}, {
							Name:  "K-Network-Hash",
							Value: "f234b5bd0c1a9d0cf485037cf836602e16448cf7b93315f24edc63fd1498e350",
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
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
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
				HTTPOption: v1alpha1.HTTPOptionEnabled,
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
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "225764a7e90e21a05c0591ed9ec9f82f7014ce34f3293ecee049ed44c3ab9eb1",
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
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
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
	}, {
		name: "single external domain with TimeoutPolicyResponse and TimeoutPolicyIdle set",
		modifyConfig: func(c *config.Config) {
			c.Contour.TimeoutPolicyResponse = "60s"
			c.Contour.TimeoutPolicyIdle = "60s"
		},
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
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
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "60s",
						Idle:     "60s",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "225764a7e90e21a05c0591ed9ec9f82f7014ce34f3293ecee049ed44c3ab9eb1",
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
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "60s",
						Idle:     "60s",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
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
	}, {
		name: "single external domain with host rewrite",
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"example.com"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							RewriteHost: "www.example.com",
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
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Host",
							Value: "www.example.com",
						}, {
							Name:  "K-Network-Hash",
							Value: "a1d5a1b1e57ab613ff0be8a20021df58150e814d4eb94488cd51802a46aca3dd",
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
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Host",
							Value: "www.example.com",
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
	}, {
		name: "single external domain with ExtensionService annotations",
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
				Annotations: map[string]string{
					ExtensionServiceKey:          "es",
					ExtensionServiceNamespaceKey: "es-ns"},
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
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
					Authorization: &v1.AuthorizationServer{
						ExtensionServiceRef: v1.ExtensionServiceReference{
							Name:      "es",
							Namespace: "es-ns",
						},
					},
				},
				Routes: []v1.Route{{
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "225764a7e90e21a05c0591ed9ec9f82f7014ce34f3293ecee049ed44c3ab9eb1",
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
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
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
			config := &config.Config{
				Contour: &config.Contour{
					VisibilityClasses: map[v1alpha1.IngressVisibility]string{
						v1alpha1.IngressVisibilityClusterLocal: privateClass,
						v1alpha1.IngressVisibilityExternalIP:   publicClass,
					},
					TimeoutPolicyResponse: "infinity",
					TimeoutPolicyIdle:     "infinity",
				},
			}

			if test.modifyConfig != nil {
				test.modifyConfig(config)
			}

			tcs := &testConfigStore{config: config}
			ctx := tcs.ToContext(context.Background())

			got := MakeHTTPProxies(ctx, test.ing, serviceToProtocol)
			if !cmp.Equal(test.want, got) {
				t.Error("MakeHTTPProxies (-want, +got) =", cmp.Diff(test.want, got))
			}
		})
	}
}

func TestMakeProxiesInternalEncryption(t *testing.T) {
	tlsProto := InternalEncryptionProtocol
	h2Proto := InternalEncryptionH2Protocol
	serviceToProtocol := map[string]string{
		"goo":  tlsProto,
		"htwo": h2Proto,
	}

	tests := []struct {
		name         string
		ing          *v1alpha1.Ingress
		want         []*v1.HTTPProxy
		modifyConfig func(*config.Config)
	}{{
		name: "single external domain with internal encryption enabled",
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
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
									ServiceName:      "goo",
									ServiceNamespace: "foo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
								AppendHeaders: map[string]string{
									"Baz":   "blah",
									"Bleep": "bloop",
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
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}, {
							Name:  "K-Network-Hash",
							Value: "6d6a4e524d201b156fae9b8d16e5dc534c6a2c18ca612061c9a5ac4b797affbb",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Port:     123,
						Protocol: &tlsProto,
						UpstreamValidation: &v1.UpstreamValidation{
							CACertificate: fmt.Sprintf("%s/knative-serving-certs", system.Namespace()),
							SubjectName:   "data-plane.knative.dev",
						},
						Weight: 100,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blah",
							}, {
								Name:  "Bleep",
								Value: "bloop",
							}},
						},
					}},
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Port:     123,
						Protocol: &tlsProto,
						UpstreamValidation: &v1.UpstreamValidation{
							CACertificate: fmt.Sprintf("%s/knative-serving-certs", system.Namespace()),
							SubjectName:   "data-plane.knative.dev",
						},
						Weight: 100,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blah",
							}, {
								Name:  "Bleep",
								Value: "bloop",
							}},
						},
					}},
				}},
			},
		}},
	}, {
		name: "single external domain with internal encryption enabled and http/2",
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
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
									ServiceName:      "htwo",
									ServiceNamespace: "foo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
								AppendHeaders: map[string]string{
									"Baz":   "blah",
									"Bleep": "bloop",
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
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}, {
							Name:  "K-Network-Hash",
							Value: "fe69d18ca560b548fd9cd1fe9a417a4de806f573b52353a435e222a23a984127",
						}},
					},
					Services: []v1.Service{{
						Name:     "htwo",
						Port:     123,
						Protocol: &h2Proto,
						UpstreamValidation: &v1.UpstreamValidation{
							CACertificate: fmt.Sprintf("%s/knative-serving-certs", system.Namespace()),
							SubjectName:   "data-plane.knative.dev",
						},
						Weight: 100,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blah",
							}, {
								Name:  "Bleep",
								Value: "bloop",
							}},
						},
					}},
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}},
					},
					Services: []v1.Service{{
						Name:     "htwo",
						Port:     123,
						Protocol: &h2Proto,
						UpstreamValidation: &v1.UpstreamValidation{
							CACertificate: fmt.Sprintf("%s/knative-serving-certs", system.Namespace()),
							SubjectName:   "data-plane.knative.dev",
						},
						Weight: 100,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blah",
							}, {
								Name:  "Bleep",
								Value: "bloop",
							}},
						},
					}},
				}},
			},
		}},
	}, {
		name: "single external domain with internal encryption enabled and http01 challenge endpoints",
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"example.com"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName:      "goo",
									ServiceNamespace: "foo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
								AppendHeaders: map[string]string{
									"Baz":   "blah",
									"Bleep": "bloop",
								},
							}},
						}, {
							Path: "/.well-known/acme-challenge/some-challenge",
							Splits: []v1alpha1.IngressBackendSplit{
								{
									IngressBackend: v1alpha1.IngressBackend{
										ServiceName:      "acme-http-solver",
										ServiceNamespace: "foo",
										ServicePort:      intstr.FromInt(8089),
									},
									Percent: 100,
								},
							},
							/*
								Note for self: http01 challenges only come up for new domains. So if you delete hello and recreate it, a new challenge won't happen, and won't trigger the case where it tries to use tls to the challenge endpoint
								   - path: /.well-known/acme-challenge/DzPfge0gXYvu417QIIQ9IAd9_YaG-rl_qby3HQlBbQA
								     splits:
								     - percent: 100
								       serviceName: cm-acme-http-solver-rczvs
								       serviceNamespace: default
								       servicePort: 8089
							*/

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
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "62a84eab49a55afbf471afc85d08701d4beff2fc957b4d7614048320d0795597",
						}},
					},
					Conditions: []v1.MatchCondition{{
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					Services: []v1.Service{{
						Name:     "goo",
						Port:     123,
						Protocol: &tlsProto,
						UpstreamValidation: &v1.UpstreamValidation{
							CACertificate: fmt.Sprintf("%s/knative-serving-certs", system.Namespace()),
							SubjectName:   "data-plane.knative.dev",
						},
						Weight: 100,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blah",
							}, {
								Name:  "Bleep",
								Value: "bloop",
							}},
						},
					}},
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "K-Network-Hash",
							Value: "62a84eab49a55afbf471afc85d08701d4beff2fc957b4d7614048320d0795597",
						}},
					},
					Conditions: []v1.MatchCondition{{
						Prefix: "/.well-known/acme-challenge/some-challenge",
					}, {
						Header: &v1.HeaderMatchCondition{
							Name:  "K-Network-Hash",
							Exact: "override",
						},
					}},
					Services: []v1.Service{{
						Name:   "acme-http-solver",
						Port:   8089,
						Weight: 100,
					}},
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
					},
					Services: []v1.Service{{
						Name:     "goo",
						Port:     123,
						Protocol: &tlsProto,
						UpstreamValidation: &v1.UpstreamValidation{
							CACertificate: fmt.Sprintf("%s/knative-serving-certs", system.Namespace()),
							SubjectName:   "data-plane.knative.dev",
						},
						Weight: 100,
						RequestHeadersPolicy: &v1.HeadersPolicy{
							Set: []v1.HeaderValue{{
								Name:  "Baz",
								Value: "blah",
							}, {
								Name:  "Bleep",
								Value: "bloop",
							}},
						},
					}},
				}, {
					EnableWebsockets: true,
					PermitInsecure:   true,
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
						Idle:     "infinity",
					},
					Conditions: []v1.MatchCondition{{
						Prefix: "/.well-known/acme-challenge/some-challenge",
					}},
					RetryPolicy: defaultRetryPolicy(),
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{},
					},
					Services: []v1.Service{{
						Name:   "acme-http-solver",
						Port:   8089,
						Weight: 100,
					}},
				}},
			},
		}},
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &config.Config{
				Contour: &config.Contour{
					VisibilityClasses: map[v1alpha1.IngressVisibility]string{
						v1alpha1.IngressVisibilityClusterLocal: privateClass,
						v1alpha1.IngressVisibilityExternalIP:   publicClass,
					},
					TimeoutPolicyResponse: "infinity",
					TimeoutPolicyIdle:     "infinity",
				},
				Network: &netcfg.Config{
					InternalEncryption: true,
				},
			}

			if test.modifyConfig != nil {
				test.modifyConfig(config)
			}

			tcs := &testConfigStore{config: config}
			ctx := tcs.ToContext(context.Background())

			got := MakeHTTPProxies(ctx, test.ing, serviceToProtocol)
			if !cmp.Equal(test.want, got) {
				t.Error("MakeHTTPProxies (-want, +got) =", cmp.Diff(test.want, got))
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
				t.Error("ServiceNames (-want, +got):", cmp.Diff(test.want, got))
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
