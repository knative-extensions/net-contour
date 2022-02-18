module knative.dev/net-contour

go 1.16

require (
	github.com/google/go-cmp v0.5.6
	github.com/projectcontour/contour v1.19.1
	go.uber.org/zap v1.19.1
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	knative.dev/hack v0.0.0-20220216040439-0456e8bf6547
	knative.dev/networking v0.0.0-20220217013411-48ac02ff5975
	knative.dev/pkg v0.0.0-20220217155112-d48172451966
	sigs.k8s.io/yaml v1.3.0
)

replace github.com/envoyproxy/go-control-plane => github.com/envoyproxy/go-control-plane v0.9.10-0.20210806072310-abdc764d71d2
