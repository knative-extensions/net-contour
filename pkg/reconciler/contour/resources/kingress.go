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

	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/net-contour/pkg/reconciler/contour/config"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/pkg/kmeta"
)

func MakeEndpointProbeIngress(ctx context.Context, ing *v1alpha1.Ingress, previousState []*v1.HTTPProxy) *v1alpha1.Ingress {
	childIng := &v1alpha1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kmeta.ChildName(ing.Name+"--", "ep"),
			Namespace: ing.Namespace,
			Labels:    ing.Labels,
			Annotations: kmeta.UnionMaps(ing.Annotations, map[string]string{
				EndpointsProbeKey: "true",
			}),
			OwnerReferences: []metav1.OwnerReference{*kmeta.NewControllerRef(ing)},
		},
	}

	sns := ServiceNames(ctx, ing)

	// Reverse engineer our previous state from the prior generation's HTTP Proxy resources.
	for _, proxy := range previousState {
		// Establish the visibility based on the class annotation.
		var vis v1alpha1.IngressVisibility
		for v, class := range config.FromContext(ctx).Contour.VisibilityClasses {
			if class == proxy.Annotations["projectcontour.io/ingress.class"] {
				vis = v
			}
		}
		if vis == "" {
			continue
		}

		for _, route := range proxy.Spec.Routes {
			for _, svc := range route.Services {
				si, ok := sns[svc.Name]
				if !ok {
					si = ServiceInfo{
						Port:         intstr.FromInt(svc.Port),
						Visibilities: make([]v1alpha1.IngressVisibility, 0, 1),
					}
				}
				has := false
				for _, v := range si.Visibilities {
					if v == vis {
						has = true
						break
					}
				}
				if !has {
					si.Visibilities = append(si.Visibilities, vis)
				}
				sns[svc.Name] = si
			}
		}
	}

	// Give the services a deterministic ordering.
	order := make(sets.String, len(sns))
	for key := range sns {
		order.Insert(key)
	}

	for _, name := range order.List() {
		si := sns[name]
		for _, vis := range si.Visibilities {
			childIng.Spec.Rules = append(childIng.Spec.Rules, v1alpha1.IngressRule{
				Hosts:      []string{fmt.Sprintf("%s.%s.%s.net-contour.invalid", name, ing.Name, ing.Namespace)},
				Visibility: vis,
				HTTP: &v1alpha1.HTTPIngressRuleValue{
					Paths: []v1alpha1.HTTPIngressPath{{
						Splits: []v1alpha1.IngressBackendSplit{{
							IngressBackend: v1alpha1.IngressBackend{
								ServiceName:      name,
								ServiceNamespace: ing.Namespace,
								ServicePort:      si.Port,
							},
						}},
					}},
				},
			})
		}
	}

	return childIng
}
