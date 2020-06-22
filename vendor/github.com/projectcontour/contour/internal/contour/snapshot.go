package contour

import (
	"strconv"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v2"
)

// SnapshotHandler implements xds snapshot cache
type SnapshotHandler struct {
	CacheHandler        *CacheHandler
	EndpointsTranslator *EndpointsTranslator

	// snapshotVersion holds the current version of the snapshot
	snapshotVersion int

	// snapshotCache holds
	SnapshotCache cache.SnapshotCache
}

func (s *SnapshotHandler) UpdateSnapshot() {
	// Increment the snapshot version
	s.snapshotVersion++

	// Create xds snapshot
	snapshot := cache.NewSnapshot(strconv.Itoa(s.snapshotVersion),
		s.EndpointsTranslator.Contents(),
		s.CacheHandler.ClusterCache.Contents(),
		s.CacheHandler.RouteCache.Contents(),
		s.CacheHandler.ListenerCache.Contents(),
		nil,
		s.CacheHandler.SecretCache.Contents())
	s.SnapshotCache.SetSnapshot("contour", snapshot)
}
