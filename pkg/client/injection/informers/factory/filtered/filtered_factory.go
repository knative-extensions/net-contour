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

// Code generated by injection-gen. DO NOT EDIT.

package filteredFactory

import (
	context "context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	externalversions "knative.dev/net-contour/pkg/client/informers/externalversions"
	client "knative.dev/net-contour/pkg/client/injection/client"
	controller "knative.dev/pkg/controller"
	injection "knative.dev/pkg/injection"
	logging "knative.dev/pkg/logging"
)

func init() {
	injection.Default.RegisterInformerFactory(withInformerFactory)
}

// Key is used as the key for associating information with a context.Context.
type Key struct {
	Selector string
}

type LabelKey struct{}

func WithSelectors(ctx context.Context, selector ...string) context.Context {
	return context.WithValue(ctx, LabelKey{}, selector)
}

func withInformerFactory(ctx context.Context) context.Context {
	c := client.Get(ctx)
	untyped := ctx.Value(LabelKey{})
	if untyped == nil {
		logging.FromContext(ctx).Panic(
			"Unable to fetch labelkey from context.")
	}
	labelSelectors := untyped.([]string)
	for _, selector := range labelSelectors {
		selectorVal := selector
		opts := []externalversions.SharedInformerOption{}
		if injection.HasNamespaceScope(ctx) {
			opts = append(opts, externalversions.WithNamespace(injection.GetNamespaceScope(ctx)))
		}
		opts = append(opts, externalversions.WithTweakListOptions(func(l *v1.ListOptions) {
			l.LabelSelector = selectorVal
		}))
		ctx = context.WithValue(ctx, Key{Selector: selectorVal},
			externalversions.NewSharedInformerFactoryWithOptions(c, controller.GetResyncPeriod(ctx), opts...))
	}
	return ctx
}

// Get extracts the InformerFactory from the context.
func Get(ctx context.Context, selector string) externalversions.SharedInformerFactory {
	untyped := ctx.Value(Key{Selector: selector})
	if untyped == nil {
		logging.FromContext(ctx).Panicf(
			"Unable to fetch knative.dev/net-contour/pkg/client/informers/externalversions.SharedInformerFactory with selector %s from context.", selector)
	}
	return untyped.(externalversions.SharedInformerFactory)
}
