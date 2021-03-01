module knative.dev/net-contour

go 1.15

require (
	github.com/google/go-cmp v0.5.4
	github.com/mikefarah/yq/v3 v3.0.0-20200601230220-721dd57ed41b
	github.com/projectcontour/contour v1.13.0
	go.uber.org/zap v1.16.0
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	knative.dev/hack v0.0.0-20210203173706-8368e1f6eacf
	knative.dev/networking v0.0.0-20210301023148-54c0eb153147
	knative.dev/pkg v0.0.0-20210226182947-9039dc189ced
	sigs.k8s.io/yaml v1.2.0
)
