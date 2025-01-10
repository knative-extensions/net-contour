//go:build e2e
// +build e2e

/*
Copyright 2021 The Knative Authors

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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/networking/pkg/apis/networking"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/networking/test"
	"knative.dev/networking/test/conformance/ingress"
	"knative.dev/pkg/reconciler"
)

func TestRewriteHost_HeadlessService(t *testing.T) {
	t.Parallel()
	ctx, clients := context.Background(), test.Setup(t)

	name, port, _ := ingress.CreateRuntimeService(ctx, t, clients, networking.ServicePortNameHTTP1)

	privateServiceName := test.ObjectNameForTest(t)
	privateHostName := privateServiceName + "." + test.ServingNamespace + ".svc." + test.NetworkingFlags.ClusterSuffix

	// Create a simple Ingress over the Service.
	ing, _, _ := ingress.CreateIngressReady(ctx, t, clients, v1alpha1.IngressSpec{
		Rules: []v1alpha1.IngressRule{{
			Visibility: v1alpha1.IngressVisibilityClusterLocal,
			Hosts:      []string{privateHostName},
			HTTP: &v1alpha1.HTTPIngressRuleValue{
				Paths: []v1alpha1.HTTPIngressPath{{
					Splits: []v1alpha1.IngressBackendSplit{{
						IngressBackend: v1alpha1.IngressBackend{
							ServiceName:      name,
							ServiceNamespace: test.ServingNamespace,
							ServicePort:      intstr.FromInt(port),
						},
					}},
				}},
			},
		}},
	})

	// Slap an ExternalName service in front of the kingress
	ip := ing.Status.PrivateLoadBalancer.Ingress[0].IP
	createHeadlessService(ctx, t, clients, privateHostName, ip)

	hosts := []string{
		"vanity.ismy.name",
		"vanity.isalsomy.number",
	}

	// Using fixed hostnames can lead to conflicts when -count=N>1
	// so pseudo-randomize the hostnames to avoid conflicts.
	for i, host := range hosts {
		hosts[i] = name + "." + host
	}

	// Now create a RewriteHost ingress to point a custom Host at the Service
	_, client, _ := ingress.CreateIngressReady(ctx, t, clients, v1alpha1.IngressSpec{
		Rules: []v1alpha1.IngressRule{{
			Hosts:      hosts,
			Visibility: v1alpha1.IngressVisibilityExternalIP,
			HTTP: &v1alpha1.HTTPIngressRuleValue{
				Paths: []v1alpha1.HTTPIngressPath{{
					RewriteHost: privateHostName,
					Splits: []v1alpha1.IngressBackendSplit{{
						IngressBackend: v1alpha1.IngressBackend{
							ServiceName:      privateServiceName,
							ServiceNamespace: test.ServingNamespace,
							ServicePort:      intstr.FromInt(80),
						},
					}},
				}},
			},
		}},
	})

	for _, host := range hosts {
		ingress.RuntimeRequest(ctx, t, client, "http://"+host)
	}
}

func createHeadlessService(ctx context.Context, t *testing.T, clients *test.Clients, target, ip string) {
	t.Helper()

	targetName := strings.SplitN(target, ".", 3)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetName[0],
			Namespace: targetName[1],
		},
		Spec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeClusterIP,
			SessionAffinity: corev1.ServiceAffinityNone,
			Ports: []corev1.ServicePort{{
				Name:       networking.ServicePortNameH2C,
				Port:       int32(80),
				TargetPort: intstr.FromInt(80),
			}},
		},
	}

	ep := &corev1.Endpoints{
		Subsets: []corev1.EndpointSubset{{
			Addresses: []corev1.EndpointAddress{{
				IP: ip,
			}},
			Ports: []corev1.EndpointPort{{
				Name: networking.ServicePortNameH2C,
				Port: 80,
			}},
		}},
	}
	ep.ObjectMeta = svc.ObjectMeta

	createService(ctx, t, clients, svc)
	createEndpoints(ctx, t, clients, ep)
}

func createEndpoints(ctx context.Context, t *testing.T, clients *test.Clients, ep *corev1.Endpoints) {
	t.Helper()

	epName := ktypes.NamespacedName{Name: ep.Name, Namespace: ep.Namespace}

	if err := reconciler.RetryTestErrors(func(attempts int) error {
		if attempts > 0 {
			t.Logf("Attempt %d creating endpoint %q", attempts, epName)
		}
		_, err := clients.KubeClient.CoreV1().Endpoints(ep.Namespace).Create(ctx, ep, metav1.CreateOptions{})
		if err != nil {
			t.Logf("Attempt %d creating endpoint failed with: %v", attempts, err)
		}
		return err
	}); err != nil {
		t.Fatalf("Error creating endpoint %q: %v", epName, err)
	}

	t.Cleanup(func() {
		clients.KubeClient.CoreV1().Endpoints(ep.Namespace).Delete(ctx, ep.Name, metav1.DeleteOptions{})
	})
}

func createService(ctx context.Context, t *testing.T, clients *test.Clients, svc *corev1.Service) {
	t.Helper()

	svcName := ktypes.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}

	if err := reconciler.RetryTestErrors(func(attempts int) error {
		if attempts > 0 {
			t.Logf("Attempt %d creating service %q", attempts, svcName)
		}
		_, err := clients.KubeClient.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
		if err != nil {
			t.Logf("Attempt %d creating service failed with: %v", attempts, err)
		}
		return err
	}); err != nil {
		t.Fatalf("Error creating Service %q: %v", svcName, err)
	}

	t.Cleanup(func() {
		clients.KubeClient.CoreV1().Services(svc.Namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
	})
}
