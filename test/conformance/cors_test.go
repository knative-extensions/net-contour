//go:build e2e
// +build e2e

/*
Copyright 2024 The Knative Authors

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

package conformance

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/system"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/net-contour/pkg/reconciler/contour/config"
	"knative.dev/networking/pkg/apis/networking"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/networking/test"
	"knative.dev/networking/test/conformance/ingress"
)

func TestCORS(t *testing.T) {
	ctx, clients := context.Background(), test.Setup(t)
	systemNamespace := system.Namespace()

	// Save the current config to restore it at the end of the test
	oldConfig, err := clients.KubeClient.CoreV1().ConfigMaps(systemNamespace).Get(ctx, config.ContourConfigName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get configmap %s/%s", systemNamespace, config.ContourConfigName)
	}

	cases := []struct {
		name          string
		configMapData map[string]string
		want          *v1.CORSPolicy
	}{
		{
			name: "cors policy set to valid values",
			configMapData: map[string]string{
				"cors-policy": `allowOrigin:
  - "*"
allowMethods:
  - GET
  - POST
  - OPTIONS
`,
			},
			want: &v1.CORSPolicy{
				AllowOrigin:  []string{"*"},
				AllowMethods: []v1.CORSHeaderValue{"GET", "POST", "OPTIONS"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mergePatch := map[string]interface{}{
				"data": c.configMapData,
			}
			patch, err := json.Marshal(mergePatch)
			if err != nil {
				t.Fatalf("Failed to marshal json with configuration for configmap %s/%s: %v", systemNamespace, config.ContourConfigName, err)
			}
			if _, err := clients.KubeClient.CoreV1().ConfigMaps(systemNamespace).Patch(ctx, config.ContourConfigName, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
				t.Fatalf("Failed to patch configmap %s/%s: %v", systemNamespace, config.ContourConfigName, err)
			}

			// Wait for changes to take effect
			t.Logf("Waiting for changes in ConfigMap to take effect")
			time.Sleep(15 * time.Second)

			// Create ingress
			name, port, _ := ingress.CreateRuntimeService(ctx, t, clients, networking.ServicePortNameHTTP1)
			hosts := []string{name + ".example.com"}

			ing, _, _ := ingress.CreateIngressReady(ctx, t, clients, v1alpha1.IngressSpec{
				Rules: []v1alpha1.IngressRule{{
					Hosts:      hosts,
					Visibility: v1alpha1.IngressVisibilityExternalIP,
					HTTP: &v1alpha1.HTTPIngressRuleValue{
						Paths: []v1alpha1.HTTPIngressPath{{
							Splits: []v1alpha1.IngressBackendSplit{{
								IngressBackend: v1alpha1.IngressBackend{
									ServiceName:      name,
									ServiceNamespace: test.ServingNamespace,
									ServicePort:      intstr.FromInt32(int32(port)),
								},
							}},
						}},
					},
				}},
			})

			// Wait ingress to be ready and get its status
			for {
				ing, err = clients.NetworkingClient.Ingresses.Get(ctx, ing.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Failed to get ingress %s", ing.Name)
				}
				if ing.IsReady() {
					break
				}
			}

			// Check if HTTPProxy was created properly
			unstructuredList, err := clients.Dynamic.Resource(schema.GroupVersionResource{
				Group:    "projectcontour.io",
				Version:  "v1",
				Resource: "httpproxies",
			}).Namespace(test.ServingNamespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				t.Fatalf("Failed to get list of HTTPProxy resources in namespace %s", test.ServingNamespace)
			}

			var externalHTTPProxy *v1.HTTPProxy
			for _, unstructured := range unstructuredList.Items {
				raw, err := unstructured.MarshalJSON()
				if err != nil {
					t.Fatal("Could not marshal unstructured resource with HTTPProxy", err)
				}
				httpProxy := &v1.HTTPProxy{}
				if err := json.Unmarshal(raw, &httpProxy); err != nil {
					t.Fatal("Could not unmarshal json to HTTPProxy object", err)
				}
				if httpProxy.Spec.VirtualHost.Fqdn != hosts[0] && httpProxy.Status.CurrentStatus != "valid" {
					t.Fatalf("Could not find HTTPProxy containing test name string %s", name)
				}
				if httpProxy.Status.LoadBalancer.Ingress != nil {
					externalHTTPProxy = httpProxy
					break
				}
			}
			if externalHTTPProxy == nil {
				t.Fatal("Could not find HTTPProxy related to external routes")
			}

			got := externalHTTPProxy.Spec.VirtualHost.CORSPolicy
			if !cmp.Equal(c.want, got) {
				t.Error("HTTPProxy CORSPolicy (-want, +got) =", cmp.Diff(c.want, got))
			}

			// Restore configmap
			cm, err := clients.KubeClient.CoreV1().ConfigMaps(systemNamespace).Get(ctx, config.ContourConfigName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get configmap %s/%s", systemNamespace, config.ContourConfigName)
			}

			cm.Data = oldConfig.Data

			if _, err := clients.KubeClient.CoreV1().ConfigMaps(systemNamespace).Update(ctx, cm, metav1.UpdateOptions{}); err != nil {
				t.Fatalf("Failed to restore configmap %s/%s: %v", systemNamespace, config.ContourConfigName, err)
			}
		})
	}
}
