module knative.dev/net-contour

go 1.16

require (
	github.com/google/go-cmp v0.5.6
	github.com/mikefarah/yq/v3 v3.0.0-20200601230220-721dd57ed41b
	github.com/projectcontour/contour v1.18.1
	go.uber.org/zap v1.19.0
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	knative.dev/hack v0.0.0-20210806075220-815cd312d65c
	knative.dev/networking v0.0.0-20210908132645-c94e114d7fed
	knative.dev/pkg v0.0.0-20210908202858-9a4b6128207c
	sigs.k8s.io/yaml v1.2.0
)
