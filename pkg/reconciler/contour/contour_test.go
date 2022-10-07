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

	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"knative.dev/pkg/logging"

	fakecontourclient "knative.dev/net-contour/pkg/client/injection/client/fake"
	fakeingressclient "knative.dev/networking/pkg/client/injection/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/net-contour/pkg/reconciler/contour/config"
	"knative.dev/net-contour/pkg/reconciler/contour/resources"
	"knative.dev/networking/pkg/apis/networking"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	ingressreconciler "knative.dev/networking/pkg/client/injection/reconciler/networking/v1alpha1/ingress"
	netconfig "knative.dev/networking/pkg/config"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/network"
	"knative.dev/pkg/reconciler"

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
				i.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
			}),
		},
	}, {
		Name: "first reconcile basic ingress",
		Key:  "ns/name",
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
		}, servicesAndEndpoints...),
		WantCreates: []runtime.Object{mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour))},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkLoadBalancerNotReady()
				i.Status.MarkIngressNotReady("EndpointsNotReady", "Waiting for Envoys to receive Endpoints data.")
			}),
		}},
	}, {
		Name:    "first reconcile basic ingress (failure creating prober)",
		Key:     "ns/name",
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("create", "ingresses"),
		},
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
		}, servicesAndEndpoints...),
		WantCreates: []runtime.Object{mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour))},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
			}),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for create ingresses"),
		},
	}, {
		Name: "first reconcile basic ingress (endpoints probe succeeded)",
		Key:  "ns/name",
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
			mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour), makeItReady),
		}, servicesAndEndpoints...),
		WantCreates: mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour)),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
				i.Status.MarkLoadBalancerReady(
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: publicSvc,
						IP:             publicSvcIP,
					}},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: privateSvc,
						IP:             privateSvcIP,
					}})
			}),
		}},
		WantDeletes: []clientgotesting.DeleteActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: "ns",
				Resource:  v1alpha1.SchemeGroupVersion.WithResource("ingresses"),
			},
			Name: "name--ep",
		}},
	}, {
		Name:    "failure deleting endpoints probe",
		Key:     "ns/name",
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("delete", "ingresses"),
		},
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
			mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour), makeItReady),
		}, servicesAndEndpoints...),
		WantCreates: mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour)),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
				i.Status.MarkLoadBalancerReady(
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: publicSvc,
						IP:             publicSvcIP,
					}},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: privateSvc,
						IP:             privateSvcIP,
					}})
			}),
		}},
		WantDeletes: []clientgotesting.DeleteActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: "ns",
				Resource:  v1alpha1.SchemeGroupVersion.WithResource("ingresses"),
			},
			Name: "name--ep",
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", "inducing failure for delete ingresses"),
		},
	}, {
		Name: "first reconcile basic ingress (endpoints probe not ready)",
		Key:  "ns/name",
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkLoadBalancerNotReady()
				i.Status.MarkIngressNotReady("EndpointsNotReady", "Waiting for Envoys to receive Endpoints data.")
			}),
			mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour)),
		}, servicesAndEndpoints...),
	}, {
		Name: "endpoints prober needs update",
		Key:  "ns/name",
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
			mustMakeProbe(t, ing("name", "ns", withBasicSpec2, withContour)),
		}, servicesAndEndpoints...),
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour)),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkLoadBalancerNotReady()
				i.Status.MarkIngressNotReady("EndpointsNotReady", "Waiting for Envoys to receive Endpoints data.")
			}),
		}},
	}, {
		Name:    "endpoints prober needs update (failure updating)",
		Key:     "ns/name",
		WantErr: true,
		WithReactors: []clientgotesting.ReactionFunc{
			InduceFailure("update", "ingresses"),
		},
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
			mustMakeProbe(t, ing("name", "ns", withBasicSpec2, withContour)),
		}, servicesAndEndpoints...),
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour)),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
			}),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "name": inducing failure for update ingresses`),
		},
	}, {
		Name: "steady state basic ingress (with probe)",
		Key:  "ns/name",
		Objects: append(append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour, makeItReady),
			mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour), makeItReady),
		}, mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour))...), servicesAndEndpoints...),
		WantDeletes: []clientgotesting.DeleteActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: "ns",
				Resource:  v1alpha1.SchemeGroupVersion.WithResource("ingresses"),
			},
			Name: "name--ep",
		}},
	}, {
		Name: "steady state basic ingress (no probe)",
		Key:  "ns/name",
		Objects: append(append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour, makeItReady),
		}, mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour))...), servicesAndEndpoints...),
	}, {
		Name: "basic ingress changed",
		Key:  "ns/name",
		Objects: append(append([]runtime.Object{
			ing("name", "ns", withContour, withGeneration(1), withBasicSpec2),
			mustMakeProbe(t, ing("name", "ns", withBasicSpec2, withContour, withGeneration(1)), makeItReady),
		}, mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour))...), servicesAndEndpoints...),
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: mustMakeProxies(t,
				ing("name", "ns", withContour, withGeneration(1), withBasicSpec2),
			)[0],
		}},
		WantDeletes: []clientgotesting.DeleteActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: "ns",
				Resource:  v1alpha1.SchemeGroupVersion.WithResource("ingresses"),
			},
			Name: "name--ep",
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
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: publicSvc,
						IP:             publicSvcIP,
					}},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: privateSvc,
						IP:             privateSvcIP,
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
			mustMakeProbe(t, ing("name", "ns", withMultiProxySpec, withContour), makeItReady),
		}, servicesAndEndpoints...),
		WantCreates: mustMakeProxies(t, ing("name", "ns", withMultiProxySpec, withContour)),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withMultiProxySpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
				i.Status.MarkLoadBalancerReady(
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: publicSvc,
						IP:             publicSvcIP,
					}},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: privateSvc,
						IP:             privateSvcIP,
					}})
			}),
		}},
		WantDeletes: []clientgotesting.DeleteActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: "ns",
				Resource:  v1alpha1.SchemeGroupVersion.WithResource("ingresses"),
			},
			Name: "name--ep",
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
			mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour), makeItReady),
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
			mustMakeProbe(t, ing("name", "ns", withBasicSpec2, withContour, withGeneration(1)), makeItReady),
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
				i.Status.ObservedGeneration = 1
				i.Status.MarkIngressNotReady("NewObservedGenFailure", "unsuccessfully observed a new generation")
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
			ing("name", "ns", withContour, withGeneration(1), withBasicSpec2),
			mustMakeProbe(t, ing("name", "ns", withBasicSpec2, withContour, withGeneration(1)), makeItReady),
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
				i.Status.ObservedGeneration = 1
				i.Status.MarkIngressNotReady("NewObservedGenFailure", "unsuccessfully observed a new generation")
			}),
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
			mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour), makeItReady),
		}, servicesAndEndpoints...),
		WantCreates: mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour)),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
				i.Status.MarkLoadBalancerReady(
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: publicSvc,
						IP:             publicSvcIP,
					}},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: privateSvc,
						IP:             privateSvcIP,
					}})
			}),
		}},
		WantDeletes: []clientgotesting.DeleteActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: "ns",
				Resource:  v1alpha1.SchemeGroupVersion.WithResource("ingresses"),
			},
			Name: "name--ep",
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "UpdateFailed", `Failed to update status for "name": inducing failure for update ingresses`),
		},
	}, {
		Name: "first reconcile, missing services",
		Key:  "ns/name--ep",
		Objects: []runtime.Object{
			mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour)),
		},
		WantErr: true,
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour), func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
			}),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `service "goo" not found`),
		},
	}}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			ingressClient: fakeingressclient.Get(ctx),
			contourClient: fakecontourclient.Get(ctx),
			ingressLister: listers.GetIngressLister(),
			contourLister: listers.GetHTTPProxyLister(),
			serviceLister: listers.GetK8sServiceLister(),
			tracker:       &NullTracker{},
			statusManager: &fakeStatusManager{
				FakeIsReady: func(context.Context, *v1alpha1.Ingress) (bool, error) {
					return true, nil
				},
			},
		}

		ingr := ingressreconciler.NewReconciler(ctx, logging.FromContext(ctx), fakeingressclient.Get(ctx),
			listers.GetIngressLister(), controller.GetEventRecorder(ctx), r, ContourIngressClassName,
			controller.Options{
				ConfigStore: &testConfigStore{
					config: defaultConfig,
				}})

		return ingr
	}))
}

func TestReconcileInternalEncryption(t *testing.T) {
	table := TableTest{{
		Name: "first reconcile basic ingress",
		Key:  "ns/name",
		Objects: append([]runtime.Object{
			ing("name", "ns", withTLSServiceSpec, withContour),
		}, tlsServiceAndEndpoint...),
		WantCreates: []runtime.Object{mustMakeProbeWithConfig(t, ing("name", "ns", withTLSServiceSpec, withContour), internalEncryptionConfig)},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withTLSServiceSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkLoadBalancerNotReady()
				i.Status.MarkIngressNotReady("EndpointsNotReady", "Waiting for Envoys to receive Endpoints data.")
			}),
		}},
	}, {
		Name: "first reconcile basic ingress (endpoints probe succeeded)",
		Key:  "ns/name",
		Objects: append([]runtime.Object{
			ing("name", "ns", withTLSServiceSpec, withContour),
			mustMakeProbe(t, ing("name", "ns", withTLSServiceSpec, withContour), makeItReady),
		}, tlsServiceAndEndpoint...),
		WantCreates: mustMakeProxiesWithConfig(t, ing("name", "ns", withTLSServiceSpec, withContour), internalEncryptionConfig),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withTLSServiceSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
				i.Status.MarkLoadBalancerReady(
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: publicSvc,
						IP:             publicSvcIP,
					}},
					[]v1alpha1.LoadBalancerIngressStatus{{
						DomainInternal: privateSvc,
						IP:             privateSvcIP,
					}})
			}),
		}},
		WantDeletes: []clientgotesting.DeleteActionImpl{{
			ActionImpl: clientgotesting.ActionImpl{
				Namespace: "ns",
				Resource:  v1alpha1.SchemeGroupVersion.WithResource("ingresses"),
			},
			Name: "name--ep",
		}},
	}}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			ingressClient: fakeingressclient.Get(ctx),
			contourClient: fakecontourclient.Get(ctx),
			ingressLister: listers.GetIngressLister(),
			contourLister: listers.GetHTTPProxyLister(),
			serviceLister: listers.GetK8sServiceLister(),
			tracker:       &NullTracker{},
			statusManager: &fakeStatusManager{
				FakeIsReady: func(context.Context, *v1alpha1.Ingress) (bool, error) {
					return true, nil
				},
			},
		}
		return ingressreconciler.NewReconciler(ctx, logging.FromContext(ctx), fakeingressclient.Get(ctx),
			listers.GetIngressLister(), controller.GetEventRecorder(ctx), r, ContourIngressClassName,
			controller.Options{
				ConfigStore: &testConfigStore{
					config: internalEncryptionConfig,
				}})
	}))
}

func TestReconcileProberNotReady(t *testing.T) {
	table := TableTest{{
		Name: "first reconcile basic ingress",
		Key:  "ns/name",
		Objects: append([]runtime.Object{
			ing("name", "ns", withBasicSpec, withContour),
			mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour), makeItReady),
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
	}}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			ingressClient: fakeingressclient.Get(ctx),
			contourClient: fakecontourclient.Get(ctx),
			ingressLister: listers.GetIngressLister(),
			contourLister: listers.GetHTTPProxyLister(),
			serviceLister: listers.GetK8sServiceLister(),
			tracker:       &NullTracker{},
			statusManager: &fakeStatusManager{
				FakeIsReady: func(context.Context, *v1alpha1.Ingress) (bool, error) {
					return false, nil
				},
			},
		}
		return ingressreconciler.NewReconciler(ctx, logging.FromContext(ctx), fakeingressclient.Get(ctx),
			listers.GetIngressLister(), controller.GetEventRecorder(ctx), r, ContourIngressClassName,
			controller.Options{
				ConfigStore: &testConfigStore{
					config: defaultConfig,
				}})
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
			mustMakeProbe(t, ing("name", "ns", withBasicSpec, withContour), makeItReady),
		}, servicesAndEndpoints...),
		WantCreates: mustMakeProxies(t, ing("name", "ns", withBasicSpec, withContour)),
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: ing("name", "ns", withBasicSpec, withContour, func(i *v1alpha1.Ingress) {
				// These are the things we expect to change in status.
				i.Status.InitializeConditions()
				i.Status.MarkNetworkConfigured()
			}),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", fmt.Sprintf("failed to probe Ingress ns/name: %v", theError)),
		},
	}}

	table.Test(t, MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		r := &Reconciler{
			ingressClient: fakeingressclient.Get(ctx),
			contourClient: fakecontourclient.Get(ctx),
			ingressLister: listers.GetIngressLister(),
			contourLister: listers.GetHTTPProxyLister(),
			serviceLister: listers.GetK8sServiceLister(),
			tracker:       &NullTracker{},
			statusManager: &fakeStatusManager{
				FakeIsReady: func(context.Context, *v1alpha1.Ingress) (bool, error) {
					return false, theError
				},
			},
		}

		ingr := ingressreconciler.NewReconciler(ctx, logging.FromContext(ctx), fakeingressclient.Get(ctx),
			listers.GetIngressLister(), controller.GetEventRecorder(ctx), r, ContourIngressClassName,
			controller.Options{
				ConfigStore: &testConfigStore{
					config: defaultConfig,
				}})

		// The Reconciler won't do any work until it becomes the leader.
		if la, ok := ingr.(reconciler.LeaderAware); ok {
			la.Promote(reconciler.UniversalBucket(), func(reconciler.Bucket, types.NamespacedName) {})
		}
		return ingr
	}))
}

var (
	publicNS      = "public-contour"
	publicName    = "envoy-stuff"
	publicKey     = fmt.Sprintf("%s/%s", publicNS, publicName)
	publicSvc     = network.GetServiceHostname(publicName, publicNS)
	publicSvcIP   = "1.2.3.4"
	privateNS     = "crouching-cont0ur"
	privateName   = "hidden-envoy"
	privateKey    = fmt.Sprintf("%s/%s", privateNS, privateName)
	privateSvc    = network.GetServiceHostname(privateName, privateNS)
	privateSvcIP  = "5.6.7.8"
	defaultConfig = &config.Config{
		Contour: &config.Contour{
			VisibilityKeys: map[v1alpha1.IngressVisibility]sets.String{
				v1alpha1.IngressVisibilityClusterLocal: sets.NewString(privateKey),
				v1alpha1.IngressVisibilityExternalIP:   sets.NewString(publicKey),
			},
		},
	}
	internalEncryptionConfig = &config.Config{
		Contour: &config.Contour{
			VisibilityKeys: map[v1alpha1.IngressVisibility]sets.String{
				v1alpha1.IngressVisibilityClusterLocal: sets.NewString(privateKey),
				v1alpha1.IngressVisibilityExternalIP:   sets.NewString(publicKey),
			},
		},
		Network: &netconfig.Config{
			InternalEncryption: true,
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
		// Contour Control Plane Services
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      publicName,
				Namespace: publicNS,
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: publicSvcIP,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      privateName,
				Namespace: privateNS,
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: privateSvcIP,
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

	tlsService = []runtime.Object{
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      tlsServiceName,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{
					Name:       "https",
					Port:       443,
					TargetPort: intstr.FromInt(8443),
					Protocol:   corev1.ProtocolTCP,
				}},
			},
		},
		// Contour Control Plane Services
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      publicName,
				Namespace: publicNS,
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: publicSvcIP,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      privateName,
				Namespace: privateNS,
			},
			Spec: corev1.ServiceSpec{
				ClusterIP: privateSvcIP,
			},
		},
	}
	tlsEndpoint = []runtime.Object{
		&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      tlsServiceName,
			},
			Subsets: []corev1.EndpointSubset{{
				Addresses: []corev1.EndpointAddress{{
					IP: "10.0.0.2",
				}},
				Ports: []corev1.EndpointPort{{
					Name: "https",
					Port: 8443,
				}},
			}},
		},
	}
	tlsServiceAndEndpoint = append(append([]runtime.Object{}, tlsService...), tlsEndpoint...)

	h2cServiceName    = "doo"
	tlsServiceName    = "tlsService"
	serviceToProtocol = map[string]string{
		h2cServiceName: "h2c",
		tlsServiceName: resources.InternalEncryptionProtocol,
	}
)

type HTTPProxyOption func(*v1.HTTPProxy)

func mustMakeProxies(t *testing.T, i *v1alpha1.Ingress, opts ...HTTPProxyOption) (objs []runtime.Object) {
	return mustMakeProxiesWithConfig(t, i, defaultConfig, opts...)
}

func mustMakeProxiesWithConfig(t *testing.T, i *v1alpha1.Ingress, cfg *config.Config, opts ...HTTPProxyOption) (objs []runtime.Object) {
	t.Helper()
	ctx := (&testConfigStore{config: cfg}).ToContext(context.Background())
	ps := resources.MakeHTTPProxies(ctx, i, serviceToProtocol)
	for _, p := range ps {
		for _, opt := range opts {
			opt(p)
		}
		objs = append(objs, p)
	}
	return
}

func deleteSelector(t *testing.T, generation int) labels.Selector {
	l, err := labels.Parse(fmt.Sprintf("%s=name,%s!=%d",
		resources.ParentKey, resources.GenerationKey, generation))
	if err != nil {
		t.Fatal("labels.Parse() =", err)
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

func mustMakeProbe(t *testing.T, i *v1alpha1.Ingress, opts ...IngressOption) runtime.Object {
	return mustMakeProbeWithConfig(t, i, defaultConfig, opts...)
}

func mustMakeProbeWithConfig(t *testing.T, i *v1alpha1.Ingress, cfg *config.Config, opts ...IngressOption) runtime.Object {
	t.Helper()
	ctx := (&testConfigStore{config: cfg}).ToContext(context.Background())
	chIng := resources.MakeEndpointProbeIngress(ctx, i, nil)
	for _, opt := range opts {
		opt(chIng)
	}
	return chIng
}

func makeItReady(i *v1alpha1.Ingress) {
	i.Status.InitializeConditions()
	i.Status.MarkNetworkConfigured()
	i.Status.MarkLoadBalancerReady(
		[]v1alpha1.LoadBalancerIngressStatus{{
			DomainInternal: publicSvc,
			IP:             publicSvcIP,
		}},
		[]v1alpha1.LoadBalancerIngressStatus{{
			DomainInternal: privateSvc,
			IP:             privateSvcIP,
		}})
}

func withBasicSpec(i *v1alpha1.Ingress) {
	i.Spec = v1alpha1.IngressSpec{
		HTTPOption: v1alpha1.HTTPOptionEnabled,
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
		HTTPOption: v1alpha1.HTTPOptionEnabled,
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

func withHTTPRedirected(i *v1alpha1.Ingress) {
	i.Spec.HTTPOption = v1alpha1.HTTPOptionRedirected
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

func withTLSServiceSpec(i *v1alpha1.Ingress) {
	i.Spec = v1alpha1.IngressSpec{
		Rules: []v1alpha1.IngressRule{{
			Hosts:      []string{"foo.com"},
			Visibility: v1alpha1.IngressVisibilityExternalIP,
			HTTP: &v1alpha1.HTTPIngressRuleValue{
				Paths: []v1alpha1.HTTPIngressPath{{
					Splits: []v1alpha1.IngressBackendSplit{{
						IngressBackend: v1alpha1.IngressBackend{
							ServiceName:      tlsServiceName,
							ServiceNamespace: i.Namespace,
							ServicePort:      intstr.FromInt(443),
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
		i.Annotations = kmeta.UnionMaps(i.Annotations, ann)
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
