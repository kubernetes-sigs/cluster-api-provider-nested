package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	clustersUpgradedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "clusters_upgraded",
			Help: "Amount of clusters upgraded by reconciler in featuregate.ClusterVersionPartialUpgrade",
		},
		[]string{"cluster_version", "resource_version"},
	)
	clustersUpgradeFailedCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "clusters_upgrade_failed",
			Help: "Amount of clusters failed to upgrade by reconciler in featuregate.ClusterVersionPartialUpgrade",
		},
		[]string{"cluster_version", "resource_version"},
	)
	clustersUpgradeSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "clusters_upgrade_seconds",
			Help:    "Duration of cluster upgrade by reconciler in featuregate.ClusterVersionPartialUpgrade",
			Buckets: []float64{.1, .5, 1, 5, 10, 20, 30, 60, 90, 120, 300, 600, 900},
		},
		[]string{"cluster_version", "resource_version"},
	)
)
