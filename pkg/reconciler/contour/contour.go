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

	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	contourclientset "knative.dev/net-contour/pkg/client/clientset/versioned"
	contourlisters "knative.dev/net-contour/pkg/client/listers/projectcontour/v1"
	ingressclientset "knative.dev/networking/pkg/client/clientset/versioned"
	ingressreconciler "knative.dev/networking/pkg/client/injection/reconciler/networking/v1alpha1/ingress"
	networkingv1alpha1 "knative.dev/networking/pkg/client/listers/networking/v1alpha1"

	"knative.dev/net-contour/pkg/reconciler/contour/config"
	"knative.dev/net-contour/pkg/reconciler/contour/resources"
	"knative.dev/net-contour/pkg/reconciler/contour/resources/names"
	"knative.dev/networking/pkg/apis/networking"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/pkg/network"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/tracker"
	"knative.dev/serving/pkg/network/status"
)

const (
	// ContourIngressClassName value for specifying knative's Contour
	// Ingress reconciler.
	ContourIngressClassName = "contour.ingress.networking.knative.dev"
)

// Reconciler implements controller.Reconciler for Ingress resources.
type Reconciler struct {
	ingressClient ingressclientset.Interface
	contourClient contourclientset.Interface

	// Listers index properties about resources
	contourLister contourlisters.HTTPProxyLister
	ingressLister networkingv1alpha1.IngressLister
	serviceLister corev1listers.ServiceLister

	statusManager status.Manager
	tracker       tracker.Interface
}

var _ ingressreconciler.Interface = (*Reconciler)(nil)

// ReconcileKind reconciles ingress resource.
func (r *Reconciler) ReconcileKind(ctx context.Context, ing *v1alpha1.Ingress) reconciler.Event {

	// Track whether there is an endpoint probe kingress to clean up.
	haveEndpointProbe := false

	if _, ok := ing.Annotations[resources.EndpointsProbeKey]; ok {
		// We only create an Endpoint probe kingress for top-level net-contour
		// kingress. Stop recursing when we see our annotation and proceed to
		// HTTP Proxy and probing.
	} else
	// See if we have any HTTPProxy resources for this generation.
	// We only create HTTPProxy resources once we have successfully probed
	// a generation's endpoints.
	if elts, err := r.contourLister.HTTPProxies(ing.Namespace).List(labels.Set(map[string]string{
		resources.ParentKey:     ing.Name,
		resources.GenerationKey: fmt.Sprintf("%d", ing.Generation),
	}).AsSelector()); err != nil {
		return err
	} else if len(elts) == 0 {
		// There are no HTTPProxy resources with the current generation.
		// Reconcile an endpoint probe child kingress to ensure the Contour
		// gateways have the endpoints for our generation's services.
		//
		// Fetch the HTTP Proxy resources from the PRIOR generation to include
		// in the Endpoint Probe.  The Endpoint probe is used to warm new Envoy
		// "clusters" (Endpoints), but also to keep the prior HTTP Proxy's "clusters"
		// in existence until the new generation has been rolled out as fully ready.
		selector, err := labels.Parse(fmt.Sprintf("%s=%s,%s!=%d",
			resources.ParentKey, ing.Name,
			resources.GenerationKey, ing.Generation))
		if err != nil {
			return err
		}
		elts, err := r.contourLister.HTTPProxies(ing.Namespace).List(selector)
		if err != nil {
			return err
		}

		desiredChIng := resources.MakeEndpointProbeIngress(ctx, ing, elts)
		actualChIng, err := r.ingressLister.Ingresses(desiredChIng.Namespace).Get(desiredChIng.Name)
		if apierrs.IsNotFound(err) { // Create it.
			actualChIng, err = r.ingressClient.NetworkingV1alpha1().Ingresses(desiredChIng.Namespace).Create(desiredChIng)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if !equality.Semantic.DeepEqual(actualChIng.Spec, desiredChIng.Spec) { // Reconcile it.
			actualChIng = actualChIng.DeepCopy()
			actualChIng.Spec = desiredChIng.Spec
			actualChIng, err = r.ingressClient.NetworkingV1alpha1().Ingresses(actualChIng.Namespace).Update(actualChIng)
			if err != nil {
				return err
			}
		}

		if !actualChIng.IsReady() {
			ing.Status.MarkIngressNotReady("EndpointsNotReady", "Waiting for Envoys to receive Endpoints data.")
			return nil
		}

		// The endpoints ingress is ready, we are good to go!
		haveEndpointProbe = true
	} else {
		_, err := r.ingressLister.Ingresses(ing.Namespace).Get(names.EndpointProbeIngress(ing))
		haveEndpointProbe = !apierrs.IsNotFound(err)
	}

	info := resources.ServiceNames(ctx, ing)
	serviceNames := make(sets.String, len(info))
	serviceToProtocol := make(map[string]string, len(info))
	for name := range info {
		serviceNames.Insert(name)
	}

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
	}

	for _, proxy := range resources.MakeHTTPProxies(ctx, ing, serviceToProtocol) {
		selector := labels.Set(map[string]string{
			resources.ParentKey:     proxy.Labels[resources.ParentKey],
			resources.DomainHashKey: proxy.Labels[resources.DomainHashKey],
		}).AsSelector()
		elts, err := r.contourLister.HTTPProxies(ing.Namespace).List(selector)
		if err != nil {
			return err
		}
		if len(elts) == 0 {
			if _, err := r.contourClient.ProjectcontourV1().HTTPProxies(proxy.Namespace).Create(proxy); err != nil {
				return err
			}
			continue
		}
		update := elts[0].DeepCopy()
		update.Annotations = proxy.Annotations
		update.Labels = proxy.Labels
		update.Spec = proxy.Spec
		if equality.Semantic.DeepEqual(elts[0], update) {
			// Avoid updates that don't change anything.
			continue
		}
		if _, err = r.contourClient.ProjectcontourV1().HTTPProxies(proxy.Namespace).Update(update); err != nil {
			return err
		}
	}

	// Before deleting old programming, check our cached to see whether there is anything to clean up.
	if selector, err := labels.Parse(fmt.Sprintf("%s=%s,%s!=%d",
		resources.ParentKey, ing.Name,
		resources.GenerationKey, ing.Generation)); err != nil {
		return err
	} else if elts, err := r.contourLister.HTTPProxies(ing.Namespace).List(selector); err != nil {
		return err
	} else if len(elts) != 0 {
		if err := r.contourClient.ProjectcontourV1().HTTPProxies(ing.Namespace).DeleteCollection(
			&metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: selector.String()}); err != nil {
			return err
		}
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

		if haveEndpointProbe {
			// Delete the endpoints probe once we have reached a steady state.
			if err := r.ingressClient.NetworkingV1alpha1().Ingresses(ing.Namespace).Delete(
				names.EndpointProbeIngress(ing), &metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
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
