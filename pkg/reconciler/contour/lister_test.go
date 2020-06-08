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
	"net/url"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/serving/pkg/network"
	"knative.dev/serving/pkg/network/status"

	"github.com/google/go-cmp/cmp"
	. "knative.dev/net-contour/pkg/reconciler/testing"
)

func TestListProbeTargets(t *testing.T) {
	tests := []struct {
		name         string
		ing          *v1alpha1.Ingress
		objects      []runtime.Object
		httpProtocol network.HTTPProtocol
		want         []status.ProbeTarget
		wantErr      error
	}{{
		name: "public with single address to probe",
		objects: []runtime.Object{
			publicService,
			privateService,
			publicEndpointsOneAddr,
			privateEndpointsNoAddr,
		},
		ing: ing("name", "ns", withBasicSpec, withContour),
		want: []status.ProbeTarget{{
			PodIPs:  sets.NewString("1.2.3.4"),
			Port:    "80",
			PodPort: "1234",
			URLs: []*url.URL{{
				Scheme: "http",
				Host:   "example.com",
			}},
		}},
	}, {
		name: "public with single address to probe (https redirected)",
		objects: []runtime.Object{
			publicSecureService,
			privateService,
			publicEndpointsOneAddr,
			privateEndpointsNoAddr,
		},
		ing:          ing("name", "ns", withBasicSpec, withContour),
		httpProtocol: network.HTTPRedirected,
		want: []status.ProbeTarget{{
			PodIPs:  sets.NewString("1.2.3.4"),
			Port:    "443",
			PodPort: "1234",
			URLs: []*url.URL{{
				Scheme: "https",
				Host:   "example.com",
			}},
		}},
	}, {
		name: "public with multiple addresses and subsets to probe",
		objects: []runtime.Object{
			publicService,
			privateService,
			publicEndpointsMultiAddrMultiSubset,
			privateEndpointsNoAddr,
		},
		ing: ing("name", "ns", withBasicSpec, withContour),
		want: []status.ProbeTarget{{
			PodIPs:  sets.NewString("2.3.4.5"),
			Port:    "80",
			PodPort: "1234",
			URLs: []*url.URL{{
				Scheme: "http",
				Host:   "example.com",
			}},
		}, {
			PodIPs:  sets.NewString("3.4.5.6", "4.3.2.1"),
			Port:    "80",
			PodPort: "4321",
			URLs: []*url.URL{{
				Scheme: "http",
				Host:   "example.com",
			}},
		}},
	}, {
		name:    "no public service",
		objects: []runtime.Object{},
		ing:     ing("name", "ns", withBasicSpec, withContour),
		wantErr: fmt.Errorf("failed to get Service: service %q not found", publicName),
	}, {
		name:    "no public endpoints",
		objects: []runtime.Object{publicService},
		ing:     ing("name", "ns", withBasicSpec, withContour),
		wantErr: fmt.Errorf("failed to get Endpoints: endpoints %q not found", publicName),
	}, {
		name:    "no port 80 in service",
		objects: []runtime.Object{publicServiceNoPort80, publicEndpointsOneAddr},
		ing:     ing("name", "ns", withBasicSpec, withContour),
		wantErr: fmt.Errorf("failed to lookup port 80 in %s/%s: no port with number 80 found",
			publicNS, publicName),
	}, {
		name:         "no port 443 in service (http disabled)",
		objects:      []runtime.Object{publicSecureServiceNoPort443, publicEndpointsOneAddr},
		ing:          ing("name", "ns", withBasicSpec, withContour),
		httpProtocol: network.HTTPDisabled,
		wantErr: fmt.Errorf("failed to lookup port 443 in %s/%s: no port with number 443 found",
			publicNS, publicName),
	}, {
		name:    "no matching port name in endpoints",
		objects: []runtime.Object{publicService, publicEndpointsWrongPortName},
		ing:     ing("name", "ns", withBasicSpec, withContour),
		wantErr: fmt.Errorf(`failed to lookup port name "asdf" in endpoints subset for %s/%s: no port for name "asdf" found`, publicNS, publicName),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tl := NewListers(test.objects)

			l := &lister{
				ServiceLister:   tl.GetK8sServiceLister(),
				EndpointsLister: tl.GetEndpointsLister(),
			}

			cfg := defaultConfig.DeepCopy()
			if len(test.httpProtocol) != 0 {
				cfg.Network.HTTPProtocol = test.httpProtocol
			}
			ctx := (&testConfigStore{config: cfg}).ToContext(context.Background())

			got, gotErr := l.ListProbeTargets(ctx, test.ing)
			if (gotErr != nil) != (test.wantErr != nil) {
				t.Fatalf("ListProbeTargets() = %v, wanted %v", gotErr, test.wantErr)
			} else if gotErr != nil && test.wantErr != nil && gotErr.Error() != test.wantErr.Error() {
				t.Fatalf("ListProbeTargets() = %v, wanted %v", gotErr, test.wantErr)
			}

			if !cmp.Equal(test.want, got) {
				t.Errorf("ListProbeTargets (-want, +got) = %s", cmp.Diff(test.want, got))
			}
		})
	}
}

var (
	publicService = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: publicNS,
			Name:      publicName,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name: "asdf",
				Port: 80,
			}},
		},
	}
	publicServiceNoPort80 = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: publicNS,
			Name:      publicName,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name: "asdf",
				Port: 81,
			}},
		},
	}
	publicSecureService = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: publicNS,
			Name:      publicName,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name: "asdf",
				Port: 443,
			}},
		},
	}
	publicSecureServiceNoPort443 = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: publicNS,
			Name:      publicName,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name: "asdf",
				Port: 444,
			}},
		},
	}
	privateService = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: privateNS,
			Name:      privateName,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{{
				Name: "fdsa",
				Port: 80,
			}},
		},
	}
	publicEndpointsOneAddr = &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: publicNS,
			Name:      publicName,
		},
		Subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name: "asdf",
				Port: 1234,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP: "1.2.3.4",
			}},
		}},
	}
	publicEndpointsWrongPortName = &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: publicNS,
			Name:      publicName,
		},
		Subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name: "wrong",
				Port: 1234,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP: "1.2.3.4",
			}},
		}},
	}
	publicEndpointsMultiAddrMultiSubset = &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: publicNS,
			Name:      publicName,
		},
		Subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name: "asdf",
				Port: 1234,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP: "2.3.4.5",
			}},
		}, {
			Ports: []corev1.EndpointPort{{
				Name: "asdf",
				Port: 4321,
			}},
			Addresses: []corev1.EndpointAddress{{
				IP: "3.4.5.6",
			}, {
				IP: "4.3.2.1",
			}},
		}},
	}
	privateEndpointsNoAddr = &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: privateNS,
			Name:      privateName,
		},
		Subsets: []corev1.EndpointSubset{{
			Ports: []corev1.EndpointPort{{
				Name: "fdsa",
				Port: 32,
			}},
		}},
	}
)
