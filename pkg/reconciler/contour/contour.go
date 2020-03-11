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
	"encoding/json"
	"fmt"

	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	ingressreconciler "knative.dev/serving/pkg/client/injection/reconciler/networking/v1alpha1/ingress"

	"knative.dev/net-contour/pkg/reconciler/contour/config"
	"knative.dev/net-contour/pkg/reconciler/contour/resources"
	"knative.dev/pkg/network"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/tracker"
	"knative.dev/serving/pkg/apis/networking"
	"knative.dev/serving/pkg/apis/networking/v1alpha1"
	"knative.dev/serving/pkg/network/status"
)

const (
	// ContourIngressClassName value for specifying knative's Contour
	// Ingress reconciler.
	ContourIngressClassName = "contour.ingress.networking.knative.dev"
)

// Reconciler implements controller.Reconciler for Ingress resources.
type Reconciler struct {
	contourClient   dynamic.Interface
	serviceLister   corev1listers.ServiceLister
	endpointsLister corev1listers.EndpointsLister
	statusManager   status.Manager
	tracker         tracker.Interface
}

var _ ingressreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind reconciles ingress resource.
func (r *Reconciler) ReconcileKind(ctx context.Context, ing *v1alpha1.Ingress) reconciler.Event {
	if ann := ing.Annotations[networking.IngressClassAnnotationKey]; ann != ContourIngressClassName {
		return nil
	}
	ing.Status.InitializeConditions()

	if err := r.reconcileProxies(ctx, ing); err != nil {
		return err
	}

	ing.Status.ObservedGeneration = ing.Generation
	return nil
}

func (r *Reconciler) reconcileProxies(ctx context.Context, ing *v1alpha1.Ingress) error {
	serviceNames := resources.ServiceNames(ctx, ing)
	serviceToProtocol := make(map[string]string, len(serviceNames))

	// Establish the protocol for each Service, and ensure that their Endpoints are
	// populated with Ready addresses before we reprogram Contour.
	for _, name := range serviceNames.List() {
		if err := r.tracker.TrackReference(tracker.Reference{
			APIVersion: "v1",
			Kind:       "Service",
			Namespace:  ing.Namespace,
			Name:       name,
		}, ing); err != nil {
			return err
		}
		svc, err := r.serviceLister.Services(ing.Namespace).Get(name)
		if err != nil {
			return err
		}
		for _, port := range svc.Spec.Ports {
			if port.Name == networking.ServicePortNameH2C {
				serviceToProtocol[name] = "h2c"
				break
			}
		}

		if err := r.tracker.TrackReference(tracker.Reference{
			APIVersion: "v1",
			Kind:       "Endpoints",
			Namespace:  ing.Namespace,
			Name:       name,
		}, ing); err != nil {
			return err
		}
		ep, err := r.endpointsLister.Endpoints(ing.Namespace).Get(name)
		if err != nil {
			return err
		}
		for _, subset := range ep.Subsets {
			if len(subset.Addresses) == 0 {
				ing.Status.MarkIngressNotReady("EndpointsNotReady",
					fmt.Sprintf("Waiting for Endpoints %q to have ready addresses.", name))
				return nil
			}
		}
	}

	for _, proxy := range resources.MakeHTTPProxies(ctx, ing, serviceToProtocol) {
		ls := metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
				resources.ParentKey, proxy.Labels[resources.ParentKey],
				resources.DomainHashKey, proxy.Labels[resources.DomainHashKey]),
		}
		elts, err := r.contourClient.Resource(v1.HTTPProxyGVR).Namespace(ing.Namespace).List(ls)
		if err != nil {
			return err
		}
		if len(elts.Items) == 0 {
			u, err := convertObjtoUnstructured(proxy)
			if err != nil {
				return err
			}

			if _, err := r.contourClient.Resource(v1.HTTPProxyGVR).Namespace(proxy.Namespace).Create(u, metav1.CreateOptions{}); err != nil {
				return err
			}
			continue
		}
		us := elts.Items[0].DeepCopy()
		update := us.DeepCopy()
		update.SetLabels(proxy.Labels)
		update.SetAnnotations(proxy.Annotations)
		// Update spec
		b, err := json.Marshal(proxy)
		if err != nil {
			return err
		}
		u := &unstructured.Unstructured{}
		if err := json.Unmarshal(b, u); err != nil {
			return err
		}
		update.Object["spec"] = u.Object["spec"]
		if equality.Semantic.DeepEqual(us, update) {
			// Avoid updates that don't change anything.
			continue
		}
		if _, err = r.contourClient.Resource(v1.HTTPProxyGVR).Namespace(proxy.Namespace).Update(update, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	if err := r.contourClient.Resource(v1.HTTPProxyGVR).Namespace(ing.Namespace).DeleteCollection(
		&metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s,%s!=%d",
				resources.ParentKey, ing.Name,
				resources.GenerationKey, ing.Generation),
		}); err != nil {
		return err
	}
	ing.Status.MarkNetworkConfigured()

	ready, err := r.statusManager.IsReady(ctx, ing)
	if err != nil {
		return fmt.Errorf("failed to probe Ingress %s/%s: %w", ing.GetNamespace(), ing.GetName(), err)
	}
	if ready {
		ing.Status.MarkLoadBalancerReady(
			[]v1alpha1.LoadBalancerIngressStatus{},
			lbStatus(ctx, v1alpha1.IngressVisibilityExternalIP),
			lbStatus(ctx, v1alpha1.IngressVisibilityClusterLocal))
	} else {
		ing.Status.MarkLoadBalancerNotReady()
	}
	return nil
}

func lbStatus(ctx context.Context, vis v1alpha1.IngressVisibility) (lbs []v1alpha1.LoadBalancerIngressStatus) {
	if keys, ok := config.FromContext(ctx).Contour.VisibilityKeys[vis]; ok {
		for _, key := range keys.List() {
			namespace, name, _ := cache.SplitMetaNamespaceKey(key)
			lbs = append(lbs, v1alpha1.LoadBalancerIngressStatus{
				DomainInternal: network.GetServiceHostname(name, namespace),
			})
		}
	}
	return
}

func convertObjtoUnstructured(p metav1.Object) (*unstructured.Unstructured, error) {
	proxyObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(p)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	u.SetUnstructuredContent(proxyObj)
	return u, nil
}
