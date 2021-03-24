module knative.dev/net-contour

go 1.15

require (
	github.com/google/go-cmp v0.5.5
	github.com/mikefarah/yq/v3 v3.0.0-20200601230220-721dd57ed41b
	github.com/projectcontour/contour v1.13.0
	go.uber.org/zap v1.16.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	knative.dev/hack v0.0.0-20210317214554-58edbdc42966
	knative.dev/networking v0.0.0-20210323181619-8fc278deb519
	knative.dev/pkg v0.0.0-20210323202917-b558677ab034
	sigs.k8s.io/yaml v1.2.0
)
