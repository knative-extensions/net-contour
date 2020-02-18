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
	"errors"
	"fmt"
	"testing"
	"time"

	fakecontourclient "knative.dev/net-contour/pkg/client/injection/client/fake"
	fakeservingclient "knative.dev/serving/pkg/client/injection/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/net-contour/pkg/reconciler/contour/config"
	"knative.dev/net-contour/pkg/reconciler/contour/resources"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/network"
	"knative.dev/pkg/reconciler"
	"knative.dev/serving/pkg/apis/networking"
	"knative.dev/serving/pkg/apis/networking/v1alpha1"
	servingnetwork "knative.dev/serving/pkg/network"
	spresources "knative.dev/serving/pkg/resources"

	. "knative.dev/net-contour/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

func TestReconcile(t *testing.T) {
	table := TableTest{{
		Name: "bad workqueue key",
		Key:  "too/many/parts",
	}, {
		Name: "key not found",
		Key:  "foo/not-found",
	}, {
		Name: "skip ingress not matching class key",
		Key:  "ns/name",
		Objects: []runtime.Object{
			ing("name", "ns", withBasicSpec, withAnnotation(map[string]string{
				networking.IngressClassAnnotationKey: "fake-controller",
			})),
		},
	}, {
		Name: "skip ingress marked for deletion",
		Key:  "ns/name",
		Objects: []runtime.Object{
			ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				i.SetDeletionTimestamp(&metav1.Time{time.Now()})
			}),
		},
	}, {
		Name: "first reconcile basic ingress",
		Key:  "ns/name",
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
		}, servicesAndEndpoints...),
		WantCreates: mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour)),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
				i.Status.MarkLoadBalancerReady(
					[]v1alpha1.LoadBalancerIngressStatus{},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: publicSvc,
					}},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: privateSvc,
					}})
			}),
		}},
		WantDeleteCollections: []clientgotesting.DeleteCollectionActionImpl{{
			ListRestrictions: clientgotesting.ListRestrictions{
				Labels: deleteSelector(t, 0),
				Fields: fields.Everything(),
			},
		}},
	}, {
		Name: "steady state basic ingress",
		Key:  "ns/name",
		Objects: append(append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// What we got from the initial reconciliation (above)
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
				i.Status.MarkLoadBalancerReady(
					[]v1alpha1.LoadBalancerIngressStatus{},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: publicSvc,
					}},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: privateSvc,
					}})
			}),
		}, mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour))...), servicesAndEndpoints...),
		// We still issue a DeleteCollection each reconcile to make sure things not of the
		// current generation are cleaned up.
		WantDeleteCollections: []clientgotesting.DeleteCollectionActionImpl{{
			ListRestrictions: clientgotesting.ListRestrictions{
				Labels: deleteSelector(t, 0),
				Fields: fields.Everything(),
			},
		}},
	}, {
		Name: "basic ingress changed",
		Key:  "ns/name",
		Objects: append(append([]runtime.Object{
			ing("name", "ns", withContour, withGeneration(1), withBasicSpec2),
		}, mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour))...), servicesAndEndpoints...),
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: mustMakeProxies(t,
				ing("name", "ns", withContour, withGeneration(1), withBasicSpec2),
			)[0],
		}},
		WantDeleteCollections: []clientgotesting.DeleteCollectionActionImpl{{
			ListRestrictions: clientgotesting.ListRestrictions{
				// We delete the things that don't match the generation being reconciled.
				Labels: deleteSelector(t, 1),
				Fields: fields.Everything(),
			},
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withContour, withGeneration(1), withBasicSpec2, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
				i.Status.MarkLoadBalancerReady(
					[]v1alpha1.LoadBalancerIngressStatus{},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: publicSvc,
					}},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: privateSvc,
					}})
				// The rest would likely have carried over from the previous reconcile,
				// but omitting it from the input object is less verbose.  The salient
				// difference is this:
				i.Status.ObservedGeneration = 1
			}),
		}},
	}, {
		Name: "first reconcile multi-httpproxy ingress",
		Key:  "ns/name",
		Objects: append([]runtime.Object{
			ing("name", "ns", withMultiProxySpec, withContour),
		}, servicesAndEndpoints...),
		WantCreates: mustMakeProxies(t, ing("name", "ns", withMultiProxySpec, withContour)),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withMultiProxySpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
				i.Status.MarkLoadBalancerReady(
					[]v1alpha1.LoadBalancerIngressStatus{},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: publicSvc,
					}},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: privateSvc,
					}})
			}),
		}},
		WantDeleteCollections: []clientgotesting.DeleteCollectionActionImpl{{
			ListRestrictions: clientgotesting.ListRestrictions{
				Labels: deleteSelector(t, 0),
				Fields: fields.Everything(),
			},
		}},
	}, {
		Name:    "error creating http proxy",
		Key:     "ns/name",
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "httpproxies"),
		},
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
		}, servicesAndEndpoints...),
		WantCreates: mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour)),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
			}),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create httpproxies"),
		},
	}, {
		Name:    "error updating http proxy",
		Key:     "ns/name",
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("update", "httpproxies"),
		},
		Objects: append(append([]runtime.Object{
			ing("name", "ns", withContour, withGeneration(1), withBasicSpec2),
		}, mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour))...), servicesAndEndpoints...),
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: mustMakeProxies(t,
				ing("name", "ns", withContour, withGeneration(1), withBasicSpec2),
			)[0],
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withContour, withGeneration(1), withBasicSpec2, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
			}),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for update httpproxies"),
		},
	}, {
		Name:    "error deleting collection",
		Key:     "ns/name",
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("delete-collection", "httpproxies"),
		},
		Objects: append(append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				i.Status.InitializeConditions()
			}),
		}, mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour))...), servicesAndEndpoints...),
		WantDeleteCollections: []clientgotesting.DeleteCollectionActionImpl{{
			ListRestrictions: clientgotesting.ListRestrictions{
				Labels: deleteSelector(t, 0),
				Fields: fields.Everything(),
			},
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for delete-collection httpproxies"),
		},
	}, {
		Name:    "error updating status",
		Key:     "ns/name",
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("update", "ingresses"),
		},
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
		}, servicesAndEndpoints...),
		WantCreates: mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour)),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
				i.Status.MarkLoadBalancerReady(
					[]v1alpha1.LoadBalancerIngressStatus{},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: publicSvc,
					}},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: privateSvc,
					}})
			}),
		}},
		WantDeleteCollections: []clientgotesting.DeleteCollectionActionImpl{{
			ListRestrictions: clientgotesting.ListRestrictions{
				Labels: deleteSelector(t, 0),
				Fields: fields.Everything(),
			},
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "name": inducing failure for update ingresses`),
		},
	}, {
		Name: "first reconcile, missing services",
		Key:  "ns/name",
		Objects: []runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
		},
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
			}),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `service "goo" not found`),
		},
	}, {
		Name: "first reconcile, missing endpoints",
		Key:  "ns/name",
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
		}, services...),
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
			}),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `endpoints "goo" not found`),
		},
	}, {
		Name: "first reconcile, empty endpoints",
		Key:  "ns/name",
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
			// The Endpoints is present, but it has no ready addresses.
			&corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns",
					Name:      "goo",
				},
				Subsets: []corev1.EndpointSubset{{
					NotReadyAddresses: []corev1.EndpointAddress{{
						IP: "10.0.0.1",
					}},
				}},
			},
		}, services...),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkIngressNotReady("EndpointsNotReady", `Waiting for Endpoints "goo" to have ready addresses.`)
			}),
		}},
	}}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			client:          fakeservingclient.Get(ctx),
			contourClient:   fakecontourclient.Get(ctx),
			lister:          listers.GetIngressLister(),
			contourLister:   listers.GetHTTPProxyLister(),
			serviceLister:   listers.GetK8sServiceLister(),
			endpointsLister: listers.GetEndpointsLister(),
			recorder:        controller.GetEventRecorder(ctx),
			tracker:         &NullTracker{},
			statusManager: &fakeStatusManager{
				FakeIsReady: func(context.Context, *v1alpha1.Ingress) (bool, error) {
					return true, nil
				},
			},
			configStore: &testConfigStore{
				config: defaultConfig,
			},
		}
	}))
}

func TestReconcileProberNotReady(t *testing.T) {
	table := TableTest{{
		Name: "first reconcile basic ingress",
		Key:  "ns/name",
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
		}, servicesAndEndpoints...),
		WantCreates: mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour)),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
				i.Status.MarkLoadBalancerNotReady()
			}),
		}},
		WantDeleteCollections: []clientgotesting.DeleteCollectionActionImpl{{
			ListRestrictions: clientgotesting.ListRestrictions{
				Labels: deleteSelector(t, 0),
				Fields: fields.Everything(),
			},
		}},
	}}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			client:          fakeservingclient.Get(ctx),
			contourClient:   fakecontourclient.Get(ctx),
			lister:          listers.GetIngressLister(),
			contourLister:   listers.GetHTTPProxyLister(),
			serviceLister:   listers.GetK8sServiceLister(),
			endpointsLister: listers.GetEndpointsLister(),
			recorder:        controller.GetEventRecorder(ctx),
			tracker:         &NullTracker{},
			statusManager: &fakeStatusManager{
				FakeIsReady: func(context.Context, *v1alpha1.Ingress) (bool, error) {
					return false, nil
				},
			},
			configStore: &testConfigStore{
				config: defaultConfig,
			},
		}
	}))
}

func TestReconcileProbeError(t *testing.T) {
	theError := errors.New("this is the error")

	table := TableTest{{
		Name:    "first reconcile basic ingress",
		Key:     "ns/name",
		WantErr: true,
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
		}, servicesAndEndpoints...),
		WantCreates: mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour)),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
			}),
		}},
		WantDeleteCollections: []clientgotesting.DeleteCollectionActionImpl{{
			ListRestrictions: clientgotesting.ListRestrictions{
				Labels: deleteSelector(t, 0),
				Fields: fields.Everything(),
			},
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", fmt.Sprintf("failed to probe Ingress ns/name: %v", theError)),
		},
	}}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		return &Reconciler{
			client:          fakeservingclient.Get(ctx),
			contourClient:   fakecontourclient.Get(ctx),
			lister:          listers.GetIngressLister(),
			contourLister:   listers.GetHTTPProxyLister(),
			serviceLister:   listers.GetK8sServiceLister(),
			endpointsLister: listers.GetEndpointsLister(),
			recorder:        controller.GetEventRecorder(ctx),
			tracker:         &NullTracker{},
			statusManager: &fakeStatusManager{
				FakeIsReady: func(context.Context, *v1alpha1.Ingress) (bool, error) {
					return false, theError
				},
			},
			configStore: &testConfigStore{
				config: defaultConfig,
			},
		}
	}))
}

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
			AutoTLS:      false,
			HTTPProtocol: servingnetwork.HTTPEnabled,
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
	withAnnotation(map[string]string{
		networking.IngressClassAnnotationKey: ContourIngressClassName,
	})(i)
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
