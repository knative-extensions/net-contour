module knative.dev/net-contour

go 1.15

require (
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/google/go-cmp v0.5.4
	github.com/mikefarah/yq/v3 v3.0.0-20200601230220-721dd57ed41b
	github.com/projectcontour/contour v1.12.0
	go.uber.org/zap v1.16.0
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	knative.dev/hack v0.0.0-20210203173706-8368e1f6eacf
	knative.dev/networking v0.0.0-20210204170711-b61da138ba21
	knative.dev/pkg v0.0.0-20210204171111-887806985c09
)
