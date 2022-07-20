package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
)

var (
	clustersUpdatedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "clusters_updated",
			Help: "Amount of clusters upgraded by reconciler in featuregate.ClusterVersionApplyCurrentState",
		},
		[]string{"cluster_version", "resource_version"},
	)
	clustersUpdateSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "clusters_update_seconds",
			Help: "Duration of cluster upgrade by reconciler in featuregate.ClusterVersionApplyCurrentState",
		},
		[]string{"cluster_version", "resource_version"},
	)
)

func init() {
	// Expose featuregate.ClusterVersionApplyCurrentState metrics only if it enabled
	if featuregate.DefaultFeatureGate.Enabled(featuregate.ClusterVersionApplyCurrentState) {
		metrics.Registry.MustRegister(clustersUpdatedCounter, clustersUpdateSeconds)
	}
}
