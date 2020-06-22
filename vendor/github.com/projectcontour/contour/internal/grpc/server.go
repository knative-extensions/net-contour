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

// Package grpc provides a gRPC implementation of the Envoy v2 xDS API.
package grpc

import (
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
)

// NewAPI returns a *grpc.Server which responds to the Envoy v2 xDS gRPC API.
func NewAPI(registry *prometheus.Registry, opts ...grpc.ServerOption) *grpc.Server {
	s := &grpcServer{
		metrics: grpc_prometheus.NewServerMetrics(),
	}
	registry.MustRegister(s.metrics)
	opts = append(opts, grpc.StreamInterceptor(s.metrics.StreamServerInterceptor()),
		grpc.UnaryInterceptor(s.metrics.UnaryServerInterceptor()))
	g := grpc.NewServer(opts...)
	s.metrics.InitializeMetrics(g)
	return g
}

// grpcServer implements the LDS, RDS, CDS, and EDS, gRPC endpoints.
type grpcServer struct {
	metrics *grpc_prometheus.ServerMetrics
}
