// Copyright © 2019 VMware
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"

	"github.com/envoyproxy/go-control-plane/pkg/cache"
	clientset "github.com/projectcontour/contour/apis/generated/clientset/versioned"
	"github.com/sirupsen/logrus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/client-go/kubernetes"
	coordinationv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

func init() {
	// even thought we don't use it directly, some of our dependencies use klog
	// so we must initialize it here to ensure that klog is set to log to stderr
	// and not to a file.
	// yes, this is gross, the klog authors are monsters.
	klog.InitFlags(nil)
}

func main() {
	log := logrus.StandardLogger()
	app := kingpin.New("contour", "Contour Kubernetes ingress controller.")

	bootstrap, bootstrapCtx := registerBootstrap(app)

	certgenApp, certgenConfig := registerCertGen(app)

	cli := app.Command("cli", "A CLI client for the Contour Kubernetes ingress controller.")
	var client Client
	cli.Flag("contour", "Contour host:port.").Default("127.0.0.1:8001").StringVar(&client.ContourAddr)
	cli.Flag("cafile", "CA bundle file for connecting to a TLS-secured Contour.").Envar("CLI_CAFILE").StringVar(&client.CAFile)
	cli.Flag("cert-file", "Client certificate file for connecting to a TLS-secured Contour.").Envar("CLI_CERT_FILE").StringVar(&client.ClientCert)
	cli.Flag("key-file", "Client key file for connecting to a TLS-secured Contour.").Envar("CLI_KEY_FILE").StringVar(&client.ClientKey)

	var resources []string
	cds := cli.Command("cds", "Watch services.")
	cds.Arg("resources", "CDS resource filter").StringsVar(&resources)
	eds := cli.Command("eds", "Watch endpoints.")
	eds.Arg("resources", "EDS resource filter").StringsVar(&resources)
	lds := cli.Command("lds", "Watch listeners.")
	lds.Arg("resources", "LDS resource filter").StringsVar(&resources)
	rds := cli.Command("rds", "Watch routes.")
	rds.Arg("resources", "RDS resource filter").StringsVar(&resources)
	sds := cli.Command("sds", "Watch secrets.")
	sds.Arg("resources", "SDS resource filter").StringsVar(&resources)

	serve, serveCtx := registerServe(app)

	args := os.Args[1:]
	switch kingpin.MustParse(app.Parse(args)) {
	case bootstrap.FullCommand():
		doBootstrap(bootstrapCtx)
	case certgenApp.FullCommand():
		doCertgen(certgenConfig)
	case cds.FullCommand():
		stream := client.ClusterStream()
		watchstream(stream, cache.ClusterType, resources)
	case eds.FullCommand():
		stream := client.EndpointStream()
		watchstream(stream, cache.EndpointType, resources)
	case lds.FullCommand():
		stream := client.ListenerStream()
		watchstream(stream, cache.ListenerType, resources)
	case rds.FullCommand():
		stream := client.RouteStream()
		watchstream(stream, cache.RouteType, resources)
	case sds.FullCommand():
		stream := client.RouteStream()
		watchstream(stream, cache.SecretType, resources)
	case serve.FullCommand():
		// parse args a second time so cli flags are applied
		// on top of any values sourced from -c's config file.
		_, err := app.Parse(args)
		check(err)
		log.Infof("args: %v", args)
		check(doServe(log, serveCtx))
	default:
		app.Usage(args)
		os.Exit(2)
	}
}

type kubernetesClients struct {
	core         *kubernetes.Clientset
	contour      *clientset.Clientset
	coordination *coordinationv1.CoordinationV1Client
}

func newKubernetesClients(kubeconfig string, inCluster bool) (kubernetesClients, error) {
	var err error
	var config *rest.Config
	var clients kubernetesClients

	if kubeconfig != "" && !inCluster {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return clients, err
	}

	clients.core, err = kubernetes.NewForConfig(config)
	if err != nil {
		return clients, err
	}

	clients.contour, err = clientset.NewForConfig(config)
	if err != nil {
		return clients, err
	}

	clients.coordination, err = coordinationv1.NewForConfig(config)
	if err != nil {
		return clients, err
	}

	return clients, nil
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
