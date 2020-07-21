module knative.dev/net-contour

go 1.14

require (
	github.com/google/go-cmp v0.5.0
	github.com/mikefarah/yq/v3 v3.0.0-20200601230220-721dd57ed41b
	github.com/projectcontour/contour v1.4.1-0.20200507033955-65d52b253570
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.18.1
	k8s.io/apimachinery v0.18.5
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/networking v0.0.0-20200720170535-ca74f50d1c0e
	knative.dev/pkg v0.0.0-20200721091635-3b7ca76a63e4
	knative.dev/serving v0.16.1-0.20200721135235-a2f470ac5430
	knative.dev/test-infra v0.0.0-20200720224135-d2706240545c
)

replace (
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
)
