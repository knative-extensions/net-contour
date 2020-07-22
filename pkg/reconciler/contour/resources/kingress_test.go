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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/pkg/network"
	"knative.dev/pkg/ptr"
)

func TestMakeEndpointProbeIngress(t *testing.T) {
	tests := []struct {
		name string
		ing  *v1alpha1.Ingress
		want *v1alpha1.Ingress
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
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"doo.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "doo",
									ServicePort:      intstr.FromInt(124),
								},
							}},
						}},
					},
				}, {
					Hosts:      []string{"goo.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
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
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"goo.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
							}},
						}},
					},
				}},
			},
		},
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
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"goo.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityClusterLocal,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
							}},
						}},
					},
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
				Rules: []v1alpha1.IngressRule{{
					Hosts:      []string{"doo.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "doo",
									ServicePort:      intstr.FromInt(124),
								},
							}},
						}},
					},
				}, {
					Hosts:      []string{"goo.bar.foo.net-contour.invalid"},
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceNamespace: "foo",
									ServiceName:      "goo",
									ServicePort:      intstr.FromInt(123),
								},
							}},
						}},
					},
				}},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := MakeEndpointProbeIngress(context.Background(), test.ing)
			if !cmp.Equal(test.want, got) {
				t.Errorf("MakeHTTPProxies (-want, +got) = %s", cmp.Diff(test.want, got))
			}
		})
	}
}
