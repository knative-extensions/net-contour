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

package testing

import (
	"context"
	"testing"

	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	fakeservingclient "knative.dev/serving/pkg/client/injection/client/fake"

	"k8s.io/apimachinery/pkg/runtime"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	rtesting "knative.dev/pkg/reconciler/testing"
)

// Ctor functions create a k8s controller with given params.
type Ctor func(context.Context, *Listers, configmap.Watcher) controller.Reconciler

// MakeFactory creates a reconciler factory with fake clients and controller created by `ctor`.
func MakeFactory(ctor Ctor) rtesting.Factory {
	return func(t *testing.T, r *rtesting.TableRow) (
		controller.Reconciler, rtesting.ActionRecorderList, rtesting.EventList) {

		ls := NewListers(r.Objects)

		ctx := r.Ctx
		if ctx == nil {
			ctx = context.Background()
		}
		logger := logtesting.TestLogger(t)
		ctx = logging.WithLogger(ctx, logger)

		eventRecorder := record.NewFakeRecorder(10)
		ctx = controller.WithEventRecorder(ctx, eventRecorder)

		ctx, client := fakeservingclient.With(ctx, ls.GetServingObjects()...)
		ctx, kubeclient := fakekubeclient.With(ctx, ls.GetKubeObjects()...)
		ctx, dynamicClient := fakedynamicclient.With(ctx,
			ls.NewScheme(), r.Objects...)

		// This is needed by the Configuration controller tests, which
		// use GenerateName to produce Revisions.
		rtesting.PrependGenerateNameReactor(&client.Fake)
		rtesting.PrependGenerateNameReactor(&kubeclient.Fake)

		dynamicClient.PrependReactor("patch", "*",
			func(action ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, nil
			})
		// Set up our Controller from the fakes.
		c := ctor(ctx, &ls, configmap.NewStaticWatcher())
		// Update the context with the stuff we decorated it with.
		r.Ctx = ctx

		for _, reactor := range r.WithReactors {
			client.PrependReactor("*", "*", reactor)
			kubeclient.PrependReactor("*", "*", reactor)
			dynamicClient.PrependReactor("*", "*", reactor)
		}

		// Validate all Create operations through the serving client.
		client.PrependReactor("create", "*", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			// TODO(n3wscott): context.Background is the best we can do at the moment, but it should be set-able.
			return rtesting.ValidateCreates(context.Background(), action)
		})
		client.PrependReactor("update", "*", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			// TODO(n3wscott): context.Background is the best we can do at the moment, but it should be set-able.
			return rtesting.ValidateUpdates(context.Background(), action)
		})

		actionRecorderList := rtesting.ActionRecorderList{dynamicClient, client, kubeclient}
		eventList := rtesting.EventList{Recorder: eventRecorder}

		return c, actionRecorderList, eventList
	}
}
