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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1 "github.com/projectcontour/contour/apis/projectcontour/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeHTTPProxies implements HTTPProxyInterface
type FakeHTTPProxies struct {
	Fake *FakeProjectcontourV1
	ns   string
}

var httpproxiesResource = v1.SchemeGroupVersion.WithResource("httpproxies")

var httpproxiesKind = v1.SchemeGroupVersion.WithKind("HTTPProxy")

// Get takes name of the hTTPProxy, and returns the corresponding hTTPProxy object, and an error if there is any.
func (c *FakeHTTPProxies) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.HTTPProxy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(httpproxiesResource, c.ns, name), &v1.HTTPProxy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.HTTPProxy), err
}

// List takes label and field selectors, and returns the list of HTTPProxies that match those selectors.
func (c *FakeHTTPProxies) List(ctx context.Context, opts metav1.ListOptions) (result *v1.HTTPProxyList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(httpproxiesResource, httpproxiesKind, c.ns, opts), &v1.HTTPProxyList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.HTTPProxyList{ListMeta: obj.(*v1.HTTPProxyList).ListMeta}
	for _, item := range obj.(*v1.HTTPProxyList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested hTTPProxies.
func (c *FakeHTTPProxies) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(httpproxiesResource, c.ns, opts))

}

// Create takes the representation of a hTTPProxy and creates it.  Returns the server's representation of the hTTPProxy, and an error, if there is any.
func (c *FakeHTTPProxies) Create(ctx context.Context, hTTPProxy *v1.HTTPProxy, opts metav1.CreateOptions) (result *v1.HTTPProxy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(httpproxiesResource, c.ns, hTTPProxy), &v1.HTTPProxy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.HTTPProxy), err
}

// Update takes the representation of a hTTPProxy and updates it. Returns the server's representation of the hTTPProxy, and an error, if there is any.
func (c *FakeHTTPProxies) Update(ctx context.Context, hTTPProxy *v1.HTTPProxy, opts metav1.UpdateOptions) (result *v1.HTTPProxy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(httpproxiesResource, c.ns, hTTPProxy), &v1.HTTPProxy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.HTTPProxy), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeHTTPProxies) UpdateStatus(ctx context.Context, hTTPProxy *v1.HTTPProxy, opts metav1.UpdateOptions) (*v1.HTTPProxy, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(httpproxiesResource, "status", c.ns, hTTPProxy), &v1.HTTPProxy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.HTTPProxy), err
}

// Delete takes name of the hTTPProxy and deletes it. Returns an error if one occurs.
func (c *FakeHTTPProxies) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(httpproxiesResource, c.ns, name, opts), &v1.HTTPProxy{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeHTTPProxies) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(httpproxiesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1.HTTPProxyList{})
	return err
}

// Patch applies the patch and returns the patched hTTPProxy.
func (c *FakeHTTPProxies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.HTTPProxy, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(httpproxiesResource, c.ns, name, pt, data, subresources...), &v1.HTTPProxy{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.HTTPProxy), err
}
