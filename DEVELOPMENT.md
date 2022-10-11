# Development

This doc explains how to setup a development environment so you can get started
[contributing](https://github.com/knative/docs/blob/main/community/CONTRIBUTING.md)
to Knative `net-contour`. Also take a look at:

- [The pull request workflow](https://github.com/knative/docs/blob/main/community/CONTRIBUTING.md#pull-requests)

## Getting started

1. Create [a GitHub account](https://github.com/join)
1. Setup
   [GitHub access via SSH](https://help.github.com/articles/connecting-to-github-with-ssh/)
1. Install [requirements](#requirements)
1. Set up your [shell environment](#environment-setup)
1. [Create and checkout a repo fork](#checkout-your-fork)

Before submitting a PR, see also [CONTRIBUTING.md](./CONTRIBUTING.md).

### Requirements

You must install these tools:

1. [`go`](https://golang.org/doc/install): The language Knative `net-contour` is
   built in
1. [`git`](https://help.github.com/articles/set-up-git/): For source control
1. [`ko`](https://github.com/google/ko): For development.
1. [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/): For
   managing development environments.

If you want to run the end-to-end (e2e) tests locally using
[kind](https://kind.sigs.k8s.io/), you may need the following additional tools
(only tested on Linux; creating a working LoadBalancer on non-Linux environments
is TBD, but the
[Setting up an Ingress Controller](https://kind.sigs.k8s.io/docs/user/ingress/)
guide _may_ work).

1. [Docker Engine](https://docs.docker.com/engine/install/) to run Docker
   containers
1. `gcc` -- your distribution should have supplied this
1. [`kind`](https://kind.sigs.k8s.io/#installation-and-usage) to run Kubernetes
   clusters locally on Docker
1. [`kubetest2`](https://github.com/kubernetes-sigs/kubetest2) to set up
   Kubernetes clusters from the test scripts
1. [`kntest`](https://github.com/knative/test-infra/tree/main/kntest) for
   additional utilities wrapping `kubetest2`
1. [`gcloud`](https://cloud.google.com/sdk/docs/install) and an active registry,
   for pushing the test images to Google Cloud Registry.

### Environment setup

To get started you'll need to set these environment variables (we recommend
adding them to your `.bashrc`):

1. `GOPATH`: If you don't have one, simply pick a directory and add
   `export GOPATH=...`
1. `$GOPATH/bin` on `PATH`: This is so that tooling installed via `go get` will
   work properly.

`.bashrc` example:

```shell
export GOPATH="$HOME/go"
export PATH="${PATH}:${GOPATH}/bin"
```

### Checkout your fork

The Go tools require that you clone the repository to the
`src/knative.dev/net-contour` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own
   [fork of this repo](https://help.github.com/articles/fork-a-repo/)
1. Clone it to your machine:

```shell
mkdir -p ${GOPATH}/src/knative.dev
cd ${GOPATH}/src/knative.dev
git clone git@github.com:${YOUR_GITHUB_USERNAME}/net-contour.git
cd net-contour
git remote add upstream https://knative.dev/net-contour.git
git remote set-url --push upstream no_push
```

_Adding the `upstream` remote sets you up nicely for regularly
[syncing your fork](https://help.github.com/articles/syncing-a-fork/)._

Once you reach this point you are ready to do a full build and deploy as
described below.

### Installing Contour

Before deploying the `net-contour` controller you will need
[a properly configured installation of Contour](https://projectcontour.io/getting-started/).
We recommend installing the latest upstream release.

### Installing and Iterating on `net-contour`

Once you have a knative/serving installation, and an appropriately configured
Contour installation, you can install the `net-contour` controller via:

```bash
ko apply -f config/
```

### Running the e2e tests manually against an existing cluster

If you have a cluster with net-contour already installed and the proper images
uploaded (from an earlier e2e run, or from building by hand), you can run the
tests directly from `go test`:

```bash
go test -tags=e2e -count=1 -timeout=20m ./test/conformance --skip-tests host-rewrite --ingressClass=contour.ingress.networking.knative.dev
```

The `host-rewrite` test uses `ExternalName` services, which are blocked by
Contour for security reasons.

### Configuring Knative Serving to use Contour by default

You can configure Serving to use Contour by default with the following command:

```bash
kubectl patch configmap/config-network \
  --namespace knative-serving \
  --type merge \
  --patch '{"data":{"ingress.class":"contour.ingress.networking.knative.dev"}}'
```

### Local Development using `kind`

If you want to develop locally ensure that you follow the
[Knative getting started](https://knative.dev/docs/getting-started/) guide or
[Manually setup serving](https://github.com/knative/serving/blob/main/DEVELOPMENT.md)
first.

#### NOTES:

If you use the konk script to setup your cluster, your cluster will be named
`knative`. However, most of the scripts expect it to be the default `kind` name.
Set the kind cluster name env `export KIND_CLUSTER_NAME=knative` to point to
`knative` cluster. KO requires a registry which if you are developing locally
you could use `export KO_DOCKER_REPO=kind.local` to use the local one on kind.
Please see official
[KO documentation](https://github.com/google/ko#local-publishing-options) for
more information.

### Running end-to-end tests on your local machine using Kind

**NOTE** These instructions work on Linux; for Windows and MacOS, additional
network routing is needed to make LoadBalancer Services reachable from the
desktop. Alternatively, on Windows and MacOS, you can choose to copy the tests
into a bash/golang container in the cluster and run them there.

**NOTE** The `dispatch/path`, `host-rewrite`, and `upgrade` tests seem to fail
(as of Jan 28) when run locally on Kind. You'll want to add these to the
`--skip-tests` flag in `hack/e2e-tests.sh`.

Run the tests from the root directory; the script will create a _new_ Kind
cluster and build and install `net-contour` on it, along with building and
pushing test images.

```bash
./test/e2e-tests.sh --cloud-provider kind
```

**WHILE THE TESTS ARE RUNNING**, install MetalLB in the cluster:

You'll want to read the
[Kind Documentation on LoadBalancers](https://kind.sigs.k8s.io/docs/user/loadbalancer/)
and find your Docker IP address range once the cluster is started. You can find
this via:

```bash
docker network inspect -f '{{.IPAM.Config}}' kind
```

You'll then need to
[create a configmap as described in the kind documentation](https://kind.sigs.k8s.io/docs/user/loadbalancer/#setup-address-pool-used-by-loadbalancers),
and save it in a `metallb-config.yaml` file. Finally, you can apply the metallb
manifests and your configmap:

```bash
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/master/manifests/namespace.yaml -f https://raw.githubusercontent.com/metallb/metallb/master/manifests/metallb.yaml  -f metallb-config.yaml
```

You should do this once the cluster is created, while the tests say
"`Waiting until service envoy in namespace contour-external has an external address (IP/hostname)...`".
