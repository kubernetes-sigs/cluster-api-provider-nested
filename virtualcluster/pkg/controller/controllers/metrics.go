package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
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
