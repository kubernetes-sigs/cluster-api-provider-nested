/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
