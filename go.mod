module knative.dev/net-contour

go 1.14

require (
	github.com/google/go-cmp v0.4.0
	github.com/projectcontour/contour v1.4.1-0.20200507033955-65d52b253570
	gopkg.in/yaml.v2 v2.2.8
	istio.io/client-go v0.0.0-20200505182340-146ba01d5357 // indirect
	k8s.io/api v0.18.2
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/pkg v0.0.0-20200511160847-a468cb85692f
	knative.dev/serving v0.14.1-0.20200429130642-78994f29e07e
	knative.dev/test-infra v0.0.0-20200509000045-c7114387eed5
)

replace (
	k8s.io/api => k8s.io/api v0.16.4
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.4
	k8s.io/client-go => k8s.io/client-go v0.16.4
	k8s.io/code-generator => k8s.io/code-generator v0.16.5-beta.1
	knative.dev/serving => knative.dev/serving v0.14.1-0.20200511192146-700e26c4f365
)
