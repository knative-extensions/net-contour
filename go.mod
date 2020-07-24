module knative.dev/net-contour

go 1.14

require (
	github.com/google/go-cmp v0.5.1
	github.com/mikefarah/yq/v3 v3.0.0-20200601230220-721dd57ed41b
	github.com/projectcontour/contour v1.4.1-0.20200507033955-65d52b253570
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.18.2
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/networking v0.0.0-20200723155758-cc457d7322d6
	knative.dev/pkg v0.0.0-20200724055557-c36f46cc8c80
	knative.dev/serving v0.16.1-0.20200724095657-0c89df260ca5
	knative.dev/test-infra v0.0.0-20200723182457-517b66ba19c1
)

replace (
	// TODO(mattmoor): DO NOT SUBMIT
	github.com/projectcontour/contour => github.com/stevesloka/contour v0.13.0-beta.2.0.20200622143240-715aab4e4b1a
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
)
