/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package controllers contains the controller for the Control Plane
// api group.
package controllers

const (
	statefulsetOwnerKeyNEtcd = ".metadata.netcd.controller"
	statefulsetOwnerKeyNKas  = ".metadata.nkas.controller"
	statefulsetOwnerKeyNKcm  = ".metadata.nkcm.controller"
	// KASManifestConfigmapName is the key name of the apiserver manifest in the configmap.
	KASManifestConfigmapName = "nkas-manifest"
	// KCMManifestConfigmapName is the key name of the controller-manager manifest in the configmap.
	KCMManifestConfigmapName = "nkcm-manifest"
	// EtcdManifestConfigmapName is the key name of the etcd manifest in the configmap.
	EtcdManifestConfigmapName = "netcd-manifest"
	loopbackAddress           = "127.0.0.1"
)
