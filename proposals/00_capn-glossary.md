# Table of Contents

This document inherits terms from [Cluster API
Glossary](https://cluster-api.sigs.k8s.io/reference/glossary.html) to support
definitions for the Cluster API Provider Nested implementation.

[C](#c) | [E](#e) | [N](#n) | [S](#s)

# C
---

<!-- Eventually this should migrate to the CAPI Glossary -->

### CAPN

Cluster API Provider Nested

### Component Controller

Operators that create the NestedControlPlane components, including the NestedEtcd controller, NestedAPIServer controller and NestedControllerManager controller.

### Cluster Admin

Responsible for creating the underlying super cluster, deploying component controllers, and in-charge of creating Nested clusters.

# E
---

### End User

Represents a nested cluster user. These users have limited access to the super cluster.

# N
---

### NestedControlPlane(NCP)

The control plane that are hosted on the super cluster.

### NestedEtcd(NEtcd)

The etcd that belongs to the control plane of the nested cluster.

### NestedAPIServer(NKAS)

The kube-apiserver which belongs to the control plane of the nested cluster.

### NestedControllerManager(NKCM)

The kube-control-manager which belongs to the control plane of the nested cluster.

### NCP

The abbreviation of the NestedControlPlane.

### NEtcd

The abbreviation of the NestedEtcd.

### NKAS

The abbreviation of the NestedAPIServer.

### NKCM

The abbreviation of the NestedControllerManager.

# S
---

### Super Cluster

The underlying cluster that manages the physical nodes, all pods created through the NestedControlPlanes will run on this cluster.
