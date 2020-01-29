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

package contour

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/net-contour/pkg/reconciler/contour/config"
	"knative.dev/net-contour/pkg/reconciler/contour/resources"
	"knative.dev/pkg/network"
	"knative.dev/serving/pkg/apis/networking/v1alpha1"
	servingnetwork "knative.dev/serving/pkg/network"
	"knative.dev/serving/pkg/reconciler"
	spresources "knative.dev/serving/pkg/resources"
)

var (
	publicNS      = "public-contour"
	publicName    = "envoy-stuff"
	publicKey     = fmt.Sprintf("%s/%s", publicNS, publicName)
	publicSvc     = network.GetServiceHostname(publicName, publicNS)
	privateNS     = "crouching-cont0ur"
	privateName   = "hidden-envoy"
	privateKey    = fmt.Sprintf("%s/%s", privateNS, privateName)
	privateSvc    = network.GetServiceHostname(privateName, privateNS)
	defaultConfig = &config.Config{
		Contour: &config.Contour{
			VisibilityKeys: map[v1alpha1.IngressVisibility]sets.String{
				v1alpha1.IngressVisibilityClusterLocal: sets.NewString(privateKey),
				v1alpha1.IngressVisibilityExternalIP:   sets.NewString(publicKey),
			},
		},
		Network: &servingnetwork.Config{
			AutoTLS: false,
		},
	}
)

var (
	services = []runtime.Object{
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "goo",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name: "http",
				}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "doo",
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name: "http2",
				}},
			},
		},
	}
	endpoints = []runtime.Object{
		&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "goo",
			},
			Subsets: []corev1.EndpointSubset{{
				Addresses: []corev1.EndpointAddress{{
					IP: "10.0.0.1",
				}},
			}},
		},
		&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "doo",
			},
			Subsets: []corev1.EndpointSubset{{
				Addresses: []corev1.EndpointAddress{{
					IP: "192.168.1.1",
				}},
			}},
		},
	}
	servicesAndEndpoints = append(append([]runtime.Object{}, services...), endpoints...)
)

func mustMakeProxies(t *testing.T, i *v1alpha1.Ingress) (objs []runtime.Object) {
	t.Helper()
	ctx := (&testConfigStore{config: defaultConfig}).ToContext(context.Background())
	ps := resources.MakeHTTPProxies(ctx, i, map[string]string{
		"doo": "h2c",
	})
	for _, p := range ps {
		objs = append(objs, p)
	}
	return
}

func deleteSelector(t *testing.T, generation int) labels.Selector {
	l, err := labels.Parse(fmt.Sprintf("%s=name,%s!=%d",
		resources.ParentKey, resources.GenerationKey, generation))
	if err != nil {
		t.Fatalf("labels.Parse() = %v", err)
	}
	return l
}

type IngressOption func(*v1alpha1.Ingress)

func ing(name, namespace string, opts ...IngressOption) *v1alpha1.Ingress {
	i := &v1alpha1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

func withBasicSpec(i *v1alpha1.Ingress) {
	i.Spec = v1alpha1.IngressSpec{
		Rules: []v1alpha1.IngressRule{{
			Hosts:      []string{"example.com"},
			Visibility: v1alpha1.IngressVisibilityExternalIP,
			HTTP: &v1alpha1.HTTPIngressRuleValue{
				Paths: []v1alpha1.HTTPIngressPath{{
					Splits: []v1alpha1.IngressBackendSplit{{
						IngressBackend: v1alpha1.IngressBackend{
							ServiceName:      "goo",
							ServiceNamespace: i.Namespace,
							ServicePort:      intstr.FromInt(123),
						},
						Percent: 100,
					}},
				}},
			},
		}},
	}
}

func withBasicSpec2(i *v1alpha1.Ingress) {
	i.Spec = v1alpha1.IngressSpec{
		Rules: []v1alpha1.IngressRule{{
			Hosts:      []string{"example.com"},
			Visibility: v1alpha1.IngressVisibilityExternalIP,
			HTTP: &v1alpha1.HTTPIngressRuleValue{
				Paths: []v1alpha1.HTTPIngressPath{{
					Splits: []v1alpha1.IngressBackendSplit{{
						IngressBackend: v1alpha1.IngressBackend{
							ServiceName:      "doo",
							ServiceNamespace: i.Namespace,
							ServicePort:      intstr.FromInt(123),
						},
						Percent: 100,
					}},
				}},
			},
		}},
	}
}

func withMultiProxySpec(i *v1alpha1.Ingress) {
	i.Spec = v1alpha1.IngressSpec{
		Rules: []v1alpha1.IngressRule{{
			Hosts:      []string{"foo.com", "bar.com"},
			Visibility: v1alpha1.IngressVisibilityExternalIP,
			HTTP: &v1alpha1.HTTPIngressRuleValue{
				Paths: []v1alpha1.HTTPIngressPath{{
					Splits: []v1alpha1.IngressBackendSplit{{
						IngressBackend: v1alpha1.IngressBackend{
							ServiceName:      "goo",
							ServiceNamespace: i.Namespace,
							ServicePort:      intstr.FromInt(123),
						},
						Percent: 100,
					}},
				}},
			},
		}},
	}
}

func withAnnotation(ann map[string]string) IngressOption {
	return func(i *v1alpha1.Ingress) {
		i.Annotations = spresources.UnionMaps(i.Annotations, ann)
	}
}

func withGeneration(gen int64) IngressOption {
	return func(i *v1alpha1.Ingress) {
		i.Generation = gen
	}
}

func withContour(i *v1alpha1.Ingress) {
	// TODO(mattmoor): Uncomment once the annotation lands.
	// withAnnotation(map[string]string{
	// 	networking.IngressClassAnnotationKey: ContourIngressClassName,
	// })(i)
}

type fakeStatusManager struct {
	FakeIsReady func(context.Context, *v1alpha1.Ingress) (bool, error)
}

func (m *fakeStatusManager) IsReady(ctx context.Context, ing *v1alpha1.Ingress) (bool, error) {
	return m.FakeIsReady(ctx, ing)
}

type testConfigStore struct {
	config *config.Config
}

func (t *testConfigStore) ToContext(ctx context.Context) context.Context {
	return config.ToContext(ctx, t.config)
}

var _ reconciler.ConfigStore = (*testConfigStore)(nil)
