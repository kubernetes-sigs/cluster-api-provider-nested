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
			Name:    "clusters_update_seconds",
			Help:    "Duration of cluster upgrade by reconciler in featuregate.ClusterVersionApplyCurrentState",
			Buckets: []float64{.1, .5, 1, 5, 10, 20, 30, 60, 90, 120, 300, 600, 900},
		},
		[]string{"cluster_version", "resource_version"},
	)
)
