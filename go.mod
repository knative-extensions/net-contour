module knative.dev/net-contour

go 1.15

require (
	github.com/google/go-cmp v0.5.6
	github.com/mikefarah/yq/v3 v3.0.0-20200601230220-721dd57ed41b
	github.com/projectcontour/contour v1.14.0
	go.uber.org/zap v1.17.0
	k8s.io/api v0.20.7
	k8s.io/apimachinery v0.20.7
	k8s.io/client-go v0.20.7
	knative.dev/hack v0.0.0-20210610231243-3d4b264d9472
	knative.dev/networking v0.0.0-20210610142944-8c7fb22941cf
	knative.dev/pkg v0.0.0-20210611022643-8d21ce6c6355
	sigs.k8s.io/yaml v1.2.0
)
