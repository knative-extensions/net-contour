module knative.dev/net-contour

go 1.14

require (
	github.com/google/go-cmp v0.4.0
	github.com/mikefarah/yq/v3 v3.0.0-20200501003153-6fc3566acd3a
	github.com/projectcontour/contour v1.4.1-0.20200507033955-65d52b253570
	gopkg.in/yaml.v2 v2.2.8
	istio.io/client-go v0.0.0-20200505182340-146ba01d5357 // indirect
	k8s.io/api v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/pkg v0.0.0-20200529164702-389d28f9b67a
	knative.dev/serving v0.15.1-0.20200529200802-e0b4b30c6461
	knative.dev/test-infra v0.0.0-20200531185603-9c9dad87b366
)

replace (
	k8s.io/api => k8s.io/api v0.16.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.4
	k8s.io/client-go => k8s.io/client-go v0.16.4
	k8s.io/code-generator => k8s.io/code-generator v0.16.4
)

replace knative.dev/serving => github.com/dprotaso/serving v0.1.1-0.20200601212011-8a3d9dbc39be
