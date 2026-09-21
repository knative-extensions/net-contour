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

	"github.com/google/go-cmp/cmp"
	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/net-contour/pkg/reconciler/contour/config"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/pkg/network"
	"knative.dev/pkg/ptr"
)

func TestMakeEndpointProbeIngress(t *testing.T) {
	tcs := &testConfigStore{
		config: &config.Config{
			Contour: &config.Contour{
				VisibilityClasses: map[v1alpha1.IngressVisibility]string{
					v1alpha1.IngressVisibilityClusterLocal: privateClass,
					v1alpha1.IngressVisibilityExternalIP:   publicClass,
				},
			},
		},
	}
	ctx := tcs.ToContext(context.Background())

	tests := []struct {
		name string
		ing  *v1alpha1.Ingress
		prev []*v1.HTTPProxy
		want *v1alpha1.Ingress
	}{{
		name: "single external domain with split (no prev)",
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:  "foo",
				Name:       "bar",
				Generation: 123,
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
		want: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar--ep",
				Annotations: map[string]string{
					EndpointsProbeKey: "true",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"doo.port-124.gen-123.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "doo",
									ServicePort:      intstr.FromInt(124),
								},
								Percent: 100,
							}},
						}},
					},
				}, {
					Hosts:      []string{"goo.port-123.gen-123.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}},
					},
				}},
			},
		},
	}, {
		// I'm not thrilled that the domain is special cased here,
		// but those are the current semantics.
		name: "external visibility with internal domain",
		ing: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:  "foo",
				Name:       "bar",
				Generation: 432,
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
		want: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar--ep",
				Annotations: map[string]string{
					EndpointsProbeKey: "true",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"goo.port-123.gen-432.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}},
					},
				}},
			},
		},
	}, {
		name: "cluster local visibility",
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
		want: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar--ep",
				Annotations: map[string]string{
					EndpointsProbeKey: "true",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"goo.port-123.gen-0.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityClusterLocal,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}},
					},
				}},
			},
		},
	}, {
		name: "https-only",
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
				TLS: []v1alpha1.IngressTLS{{
					Hosts:      []string{"example.com"},
					SecretName: "example",
				}},
			},
		},
		want: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar--ep",
				Annotations: map[string]string{
					EndpointsProbeKey: "true",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionRedirected,
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"goo.port-123.gen-0.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}},
					},
				}},
				TLS: []v1alpha1.IngressTLS{{
					Hosts:      []string{"goo.port-123.gen-0.bar.foo.net-contour.invalid"},
					SecretName: "example",
				}},
			},
		},
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
		want: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar--ep",
				Annotations: map[string]string{
					EndpointsProbeKey: "true",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"goo.port-123.gen-0.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}},
					},
				}},
			},
		},
	}, {
		name: "single external domain with split (w/ prev and overlap)",
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
		prev: []*v1.HTTPProxy{{
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
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
					},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}, {
							Name:  "Host",
							Value: "previous.internal.example",
						}, {
							Name:  "K-Network-Hash",
							Value: "99dbaae65d712842149f0be3a930d0e229226f86fadddd36bb7b87b0a38ffd3e",
						}},
					},
					Services: []v1.Service{{
						Name:   "goo",
						Port:   123,
						Weight: 12,
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
			Status: v1.HTTPProxyStatus{
				CurrentStatus: "valid",
			},
		}},
		want: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar--ep",
				Annotations: map[string]string{
					EndpointsProbeKey: "true",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"doo.port-124.gen-0.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "doo",
									ServicePort:      intstr.FromInt(124),
								},
								Percent: 100,
							}},
						}},
					},
				}, {
					Hosts:      []string{"goo.port-123.gen-0.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}},
					},
				}},
			},
		},
	}, {
		name: "single external domain with split (w/ prev no overlap and host rewrite)",
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
		prev: []*v1.HTTPProxy{{
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
					TimeoutPolicy: &v1.TimeoutPolicy{
						Response: "infinity",
					},
					RequestHeadersPolicy: &v1.HeadersPolicy{
						Set: []v1.HeaderValue{{
							Name:  "Foo",
							Value: "bar",
						}, {
							Name:  "host",
							Value: "previous.internal.example",
						}, {
							Name:  "K-Network-Hash",
							Value: "99dbaae65d712842149f0be3a930d0e229226f86fadddd36bb7b87b0a38ffd3e",
						}},
					},
					Services: []v1.Service{{
						Name:   "kung",
						Port:   123,
						Weight: 12,
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
						Name:   "fu",
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
					// TODO(https://github.com/knative-sandbox/net-certmanager/issues/44): Probe this once fixed.
					Conditions: []v1.MatchCondition{{
						Prefix: "/blah",
					}},
					Services: []v1.Service{{
						Name: "blah",
						Port: 123,
					}},
				}},
			},
			Status: v1.HTTPProxyStatus{
				CurrentStatus: "valid",
			},
		}},
		want: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar--ep",
				Annotations: map[string]string{
					EndpointsProbeKey: "true",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"doo.port-124.gen-0.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "doo",
									ServicePort:      intstr.FromInt(124),
								},
								Percent: 100,
							}},
						}},
					},
				}, {
					Hosts:      []string{"fu.port-124.gen-0.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							RewriteHost: "previous.internal.example",
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "fu",
									ServicePort:      intstr.FromInt(124),
								},
								Percent: 100,
							}},
						}},
					},
				}, {
					Hosts:      []string{"goo.port-123.gen-0.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}},
					},
				}, {
					Hosts:      []string{"kung.port-123.gen-0.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							RewriteHost: "previous.internal.example",
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "kung",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}},
					},
				}},
			},
		},
	}, {
		name: "single external domain with split (w/ invalid prev)",
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
		prev: []*v1.HTTPProxy{{
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
						Name:   "kung",
						Port:   123,
						Weight: 12,
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
						Name:   "fu",
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
					// TODO(https://github.com/knative-sandbox/net-certmanager/issues/44): Probe this once fixed.
					Conditions: []v1.MatchCondition{{
						Prefix: "/blah",
					}},
					Services: []v1.Service{{
						Name: "blah",
						Port: 123,
					}},
				}},
			},
			Status: v1.HTTPProxyStatus{
				CurrentStatus: "invalid",
				Description:   "Spec.Routes unresolved service reference: service \"foo/kung\" not found",
			},
		}},
		want: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar--ep",
				Annotations: map[string]string{
					EndpointsProbeKey: "true",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
				// want ingress only from valid HTTPProxy.
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"doo.port-124.gen-0.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "doo",
									ServicePort:      intstr.FromInt(124),
								},
								Percent: 100,
							}},
						}},
					},
				}, {
					Hosts:      []string{"goo.port-123.gen-0.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}},
					},
				}},
			},
		},
	}, {
		name: "single with host rewrite (host rewrite passes down)",
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
							RewriteHost: "foo.com",
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
		want: &v1alpha1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar--ep",
				Annotations: map[string]string{
					EndpointsProbeKey: "true",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "networking.internal.knative.dev/v1alpha1",
					Kind:               "Ingress",
					Name:               "bar",
					Controller:         ptr.Bool(true),
					BlockOwnerDeletion: ptr.Bool(true),
				}},
			},
			Spec: v1alpha1.IngressSpec{
				HTTPOption: v1alpha1.HTTPOptionEnabled,
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"goo.port-123.gen-0.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							RewriteHost: "foo.com",
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
								Percent: 100,
							}},
						}},
					},
				}},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := MakeEndpointProbeIngress(ctx, test.ing, test.prev)
			if !cmp.Equal(test.want, got) {
				t.Error("MakeHTTPProxies (-want, +got) =", cmp.Diff(test.want, got))
			}
		})
	}
}

func TestMakeEndpointProbeIngressUsesServicePortIdentity(t *testing.T) {
	ctx := (&testConfigStore{
		config: &config.Config{
			Contour: &config.Contour{
				VisibilityClasses: map[v1alpha1.IngressVisibility]string{
					v1alpha1.IngressVisibilityExternalIP: publicClass,
				},
			},
		},
	}).ToContext(context.Background())

	ing := &v1alpha1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "default",
			Name:       "mapping",
			Generation: 3,
		},
		Spec: v1alpha1.IngressSpec{
			Rules: []v1alpha1.IngressRule{{
				Visibility: v1alpha1.IngressVisibilityExternalIP,
				HTTP: &v1alpha1.HTTPIngressRuleValue{
					Paths: []v1alpha1.HTTPIngressPath{{
						Splits: []v1alpha1.IngressBackendSplit{{
							IngressBackend: v1alpha1.IngressBackend{
								ServiceName: "app",
								ServicePort: intstr.FromInt(8080),
							},
						}, {
							IngressBackend: v1alpha1.IngressBackend{
								ServiceName: "metrics",
								ServicePort: intstr.FromString("http"),
							},
						}},
					}},
				},
			}},
		},
	}
	previous := []*v1.HTTPProxy{{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"projectcontour.io/ingress.class": publicClass,
			},
		},
		Spec: v1.HTTPProxySpec{
			Routes: []v1.Route{{
				RequestHeadersPolicy: &v1.HeadersPolicy{
					Set: []v1.HeaderValue{{
						Name:  "Host",
						Value: "app.default.svc.cluster.local",
					}},
				},
				Services: []v1.Service{{
					Name: "app",
					Port: 80,
				}},
			}},
		},
		Status: v1.HTTPProxyStatus{
			CurrentStatus: "valid",
		},
	}}

	want := []v1alpha1.IngressRule{{
		Hosts:      []string{"app.port-80.gen-3.mapping.default.net-contour.invalid"},
		Visibility: v1alpha1.IngressVisibilityExternalIP,
		HTTP: &v1alpha1.HTTPIngressRuleValue{
			Paths: []v1alpha1.HTTPIngressPath{{
				RewriteHost: "app.default.svc.cluster.local",
				Splits: []v1alpha1.IngressBackendSplit{{
					IngressBackend: v1alpha1.IngressBackend{
						ServiceName:      "app",
						ServiceNamespace: "default",
						ServicePort:      intstr.FromInt(80),
					},
					Percent: 100,
				}},
			}},
		},
	}, {
		Hosts:      []string{"app.port-8080.gen-3.mapping.default.net-contour.invalid"},
		Visibility: v1alpha1.IngressVisibilityExternalIP,
		HTTP: &v1alpha1.HTTPIngressRuleValue{
			Paths: []v1alpha1.HTTPIngressPath{{
				Splits: []v1alpha1.IngressBackendSplit{{
					IngressBackend: v1alpha1.IngressBackend{
						ServiceName:      "app",
						ServiceNamespace: "default",
						ServicePort:      intstr.FromInt(8080),
					},
					Percent: 100,
				}},
			}},
		},
	}, {
		Hosts:      []string{"metrics.port-http.gen-3.mapping.default.net-contour.invalid"},
		Visibility: v1alpha1.IngressVisibilityExternalIP,
		HTTP: &v1alpha1.HTTPIngressRuleValue{
			Paths: []v1alpha1.HTTPIngressPath{{
				Splits: []v1alpha1.IngressBackendSplit{{
					IngressBackend: v1alpha1.IngressBackend{
						ServiceName:      "metrics",
						ServiceNamespace: "default",
						ServicePort:      intstr.FromString("http"),
					},
					Percent: 100,
				}},
			}},
		},
	}}

	got := MakeEndpointProbeIngress(ctx, ing, previous)
	if diff := cmp.Diff(want, got.Spec.Rules); diff != "" {
		t.Error("MakeEndpointProbeIngress rules (-want, +got):", diff)
	}
}
