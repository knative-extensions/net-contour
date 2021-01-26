module knative.dev/net-contour

go 1.14

require (
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/google/go-cmp v0.5.4
	github.com/googleapis/gnostic v0.4.0 // indirect
	github.com/mikefarah/yq/v3 v3.0.0-20200601230220-721dd57ed41b
	github.com/projectcontour/contour v1.10.0
	go.uber.org/zap v1.16.0
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29 // indirect
	knative.dev/hack v0.0.0-20210120165453-8d623a0af457
	knative.dev/networking v0.0.0-20210125050654-94433ab7f620
	knative.dev/pkg v0.0.0-20210125222030-6040b3af4803
	sigs.k8s.io/structured-merge-diff/v3 v3.0.1-0.20200706213357-43c19bbb7fba // indirect
)

replace (
	k8s.io/api => k8s.io/api v0.18.12
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.12
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.12
	k8s.io/apiserver => k8s.io/apiserver v0.18.8
	k8s.io/client-go => k8s.io/client-go v0.18.12
	k8s.io/code-generator => k8s.io/code-generator v0.18.12
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20200410145947-61e04a5be9a6
)
