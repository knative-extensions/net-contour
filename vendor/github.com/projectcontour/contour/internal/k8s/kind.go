// Copyright © 2019 VMware
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8s

import (
	ingressroutev1 "github.com/projectcontour/contour/apis/contour/v1beta1"
	projectcontour "github.com/projectcontour/contour/apis/projectcontour/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1beta1"
)

// KindOf returns the kind string for the given Kubernetes object.
//
// The API machinery doesn't populate the metav1.TypeMeta field for
// objects, so we have to use a type assertion to detect kinds that
// we care about.
func KindOf(obj interface{}) string {
	switch obj.(type) {
	case *v1.Secret:
		return "Secret"
	case *v1.Service:
		return "Service"
	case *v1beta1.Ingress:
		return "Ingress"
	case *ingressroutev1.IngressRoute:
		return "IngressRoute"
	case *projectcontour.HTTPProxy:
		return "HTTPProxy"
	case *ingressroutev1.TLSCertificateDelegation:
		return "TLSCertificateDelegation"
	case *projectcontour.TLSCertificateDelegation:
		return "TLSCertificateDelegation"
	default:
		return ""
	}
}
