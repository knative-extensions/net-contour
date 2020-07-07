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
	knative.dev/networking v0.0.0-20200707203944-725ec013d8a2
	knative.dev/pkg v0.0.0-20200707190344-0a8314b44495
	knative.dev/serving v0.16.1-0.20200707200544-47f8b0ede63a
	knative.dev/test-infra v0.0.0-20200707183444-aed09e56ddc7
)

replace (
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
)
