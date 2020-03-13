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
	"fmt"

	projectcontour "github.com/projectcontour/contour/apis/projectcontour/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type Converter interface {
	Convert(obj interface{}) (interface{}, error)
	CanConvert(obj interface{}) bool
}

// UnstructuredConverter handles conversions between unstructured.Unstructured and Contour types
type UnstructuredConverter struct {
	// scheme holds an initializer for converting Unstructured to a type
	scheme *runtime.Scheme
}

// NewUnstructuredConverter returns a new UnstructuredConverter initialized
func NewUnstructuredConverter() (*UnstructuredConverter, error) {
	uc := &UnstructuredConverter{
		scheme: runtime.NewScheme(),
	}

	// Setup converter to understand custom CRD types
	projectcontour.AddKnownTypes(uc.scheme)

	return uc, nil
}

func (c *UnstructuredConverter) CanConvert(obj interface{}) bool {
	_, ok := obj.(*unstructured.Unstructured)
	return ok
}

// Convert converts an unstructured.Unstructured to typed struct
func (c *UnstructuredConverter) Convert(obj interface{}) (interface{}, error) {
	unstructured, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("unsupported object type: %T", obj)
	}
	switch unstructured.GetKind() {
	case "HTTPProxy":
		proxy := &projectcontour.HTTPProxy{}
		err := c.scheme.Convert(obj, proxy, nil)
		return proxy, err
	default:
		return nil, fmt.Errorf("unsupported object type: %T", obj)
	}
}
