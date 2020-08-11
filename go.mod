module knative.dev/net-contour

go 1.14

require (
	github.com/google/go-cmp v0.5.1
	github.com/mikefarah/yq/v3 v3.0.0-20200601230220-721dd57ed41b
	github.com/projectcontour/contour v1.4.1-0.20200507033955-65d52b253570
	go.uber.org/zap v1.15.0
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.18.7-rc.0
	k8s.io/apimachinery v0.18.7-rc.0
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/networking v0.0.0-20200811174006-3b1fd29485ee
	knative.dev/pkg v0.0.0-20200811182305-91561f0a4117
	knative.dev/test-infra v0.0.0-20200811195106-afcd1747545f
)

replace (
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
)
