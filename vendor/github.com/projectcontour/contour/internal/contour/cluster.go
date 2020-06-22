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

package contour

import (
	"sort"
	"sync"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/projectcontour/contour/internal/dag"
	"github.com/projectcontour/contour/internal/envoy"
	"github.com/projectcontour/contour/internal/protobuf"
	"github.com/projectcontour/contour/internal/sorter"
)

// ClusterCache manages the contents of the gRPC CDS cache.
type ClusterCache struct {
	mu     sync.Mutex
	values map[string]*v2.Cluster
}

// Update replaces the contents of the cache with the supplied map.
func (c *ClusterCache) Update(v map[string]*v2.Cluster) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.values = v
}

// Contents returns a copy of the cache's contents.
func (c *ClusterCache) Contents() []types.Resource {
	c.mu.Lock()
	defer c.mu.Unlock()
	var values []*v2.Cluster
	for _, v := range c.values {
		values = append(values, v)
	}
	sort.Stable(sorter.For(values))
	return protobuf.AsMessages(values)
}

type clusterVisitor struct {
	clusters map[string]*v2.Cluster
}

// visitCluster produces a map of *v2.Clusters.
func visitClusters(root dag.Vertex) map[string]*v2.Cluster {
	cv := clusterVisitor{
		clusters: make(map[string]*v2.Cluster),
	}
	cv.visit(root)
	return cv.clusters
}

func (v *clusterVisitor) visit(vertex dag.Vertex) {
	if cluster, ok := vertex.(*dag.Cluster); ok {
		name := envoy.Clustername(cluster)
		if _, ok := v.clusters[name]; !ok {
			c := envoy.Cluster(cluster)
			v.clusters[c.Name] = c
		}
	}

	// recurse into children of v
	vertex.Visit(v.visit)
}
