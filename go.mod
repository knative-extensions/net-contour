module knative.dev/net-contour

go 1.14

require (
	github.com/google/go-cmp v0.4.0
	github.com/mikefarah/yq/v3 v3.0.0-20200601230220-721dd57ed41b
	github.com/projectcontour/contour v1.4.1-0.20200507033955-65d52b253570
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.18.1
	k8s.io/apimachinery v0.18.1
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	knative.dev/networking v0.0.0-20200611155523-bd13675bcf70
	knative.dev/pkg v0.0.0-20200616011124-086ff4395641
	knative.dev/serving v0.15.1-0.20200616123024-a1a08f451a22
	knative.dev/test-infra v0.0.0-20200615231324-3a016f44102c
)

replace (
	k8s.io/api => k8s.io/api v0.17.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.17.6
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
)
