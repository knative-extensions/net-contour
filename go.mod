module knative.dev/net-contour

go 1.16

require (
	github.com/bombsimon/logrusr/v2 v2.0.0-00010101000000-000000000000
	github.com/google/go-cmp v0.5.6
	github.com/mikefarah/yq/v3 v3.0.0-20200601230220-721dd57ed41b
	github.com/projectcontour/contour v1.19.1
	go.uber.org/zap v1.19.1
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	knative.dev/hack v0.0.0-20220111151514-59b0cf17578e
	knative.dev/networking v0.0.0-20220112013650-eac673fb5c49
	knative.dev/pkg v0.0.0-20220112181951-2b23ad111bc2
	sigs.k8s.io/yaml v1.3.0
)

// TODO - drop these requires when we bump to contour 1.20
//
// This is necessary so we can build our controller with klog/v2
// which offers us the ability to pipe logs to zap if needed
//
// Upstream contour uses an older controller-runtime that was using
// and older klog so these replace directives (with the patch) update
// their version with one that's compatible with ours
replace (
	github.com/bombsimon/logrusr/v2 => github.com/bombsimon/logrusr/v2 v2.0.1
	github.com/envoyproxy/go-control-plane => github.com/envoyproxy/go-control-plane v0.9.10-0.20210806072310-abdc764d71d2
	github.com/go-logr/logr => github.com/go-logr/logr v1.2.2

	k8s.io/api => k8s.io/api v0.22.5
	k8s.io/apimachinery => k8s.io/apimachinery v0.22.5
	k8s.io/client-go => k8s.io/client-go v0.22.5
	k8s.io/component-base => k8s.io/component-base v0.22.5
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.11.0
)
