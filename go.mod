module knative.dev/net-contour

go 1.15

require (
	github.com/google/go-cmp v0.5.5
	github.com/mikefarah/yq/v3 v3.0.0-20200601230220-721dd57ed41b
	github.com/projectcontour/contour v1.14.0
	go.uber.org/zap v1.16.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	knative.dev/hack v0.0.0-20210423193138-b5f6e2587f6d
	knative.dev/networking v0.0.0-20210423055338-2b84569a04be
	knative.dev/pkg v0.0.0-20210423162638-78b8140ed19c
	sigs.k8s.io/yaml v1.2.0
)
