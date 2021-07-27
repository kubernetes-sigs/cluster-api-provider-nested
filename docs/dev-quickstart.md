<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [Development Quick Start](#development-quick-start)
  - [Prerequisites](#prerequisites)
  - [Create `kind` cluster](#create-kind-cluster)
  - [Install `cert-manager`](#install-cert-manager)
  - [Clone CAPI and Deploy Dev release](#clone-capi-and-deploy-dev-release)
  - [Clone CAPN](#clone-capn)
  - [Create Docker Images, Manifests and Load Images](#create-docker-images-manifests-and-load-images)
  - [Deploy CAPN](#deploy-capn)
  - [Apply Sample Tenant Cluster](#apply-sample-tenant-cluster)
  - [Get `KUBECONFIG`](#get-kubeconfig)
  - [Port Forward](#port-forward)
  - [Connect to Cluster](#connect-to-cluster)
  - [Connect to the Cluster! :tada:](#connect-to-the-cluster-tada)
  - [Clean Up](#clean-up)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Development Quick Start 

This tutorial introduces how to create a nested controlplane from source code for development. CAPN should work with any standard Kubernetes cluster out of box, but for demo purposes, in this tutorial, we will use a `kind` cluster as the management cluster as well as the nested workload cluster.

### Prerequisites

Please install the latest version of [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) and [kubectl](https://kubernetes.io/docs/tasks/tools/)

### Create `kind` cluster

```console
# For Kind
kind create cluster --name=capn
```

```console
# For Minikube
minikube start
```

### Install `cert-manager`

Cert Manager is a soft dependency for the Cluster API components to enable mutating and validating webhooks to be auto deployed. For more detailed instructions go [Cert Manager Installion](https://cert-manager.io/docs/installation/kubernetes/#installing-with-regular-manifests).

```console
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
```

After Cert Manager got installed, you will see there is a new namespace `cert-manager` has been created, and some pods for cert manager are running under this namespace.

```console
# kubectl  get ns
NAME                 STATUS   AGE
cert-manager         Active   27s
default              Active   71s
kube-node-lease      Active   73s
kube-public          Active   73s
kube-system          Active   73s
local-path-storage   Active   68s
```

```console
# kubectl  get po -n cert-manager
NAME                                       READY   STATUS    RESTARTS   AGE
cert-manager-7dd5854bb4-8jp4b              1/1     Running   0          32s
cert-manager-cainjector-64c949654c-bhdzz   1/1     Running   0          32s
cert-manager-webhook-6b57b9b886-5cbnp      1/1     Running   0          32s
```

### Clone CAPI and Deploy Dev release

As a cluster API~(CAPI) provider, CAPN requires core components of CAPI to be setup. We need to deploy the unreleased version of CAPI for `v1alpha4` API support.

```console
git clone git@github.com:kubernetes-sigs/cluster-api.git
cd cluster-api
make release-manifests
# change feature flags on core
sed -i'' -e 's@- --feature-gates=.*@- --feature-gates=MachinePool=false,ClusterResourceSet=true@' out/core-components.yaml
kubectl apply -f out/core-components.yaml
cd ..
```

This will help deploy the Cluster API Core Controller under namesapce `capi-system`.

```console
# kubectl  get ns
NAME                 STATUS   AGE
capi-system          Active   52s
cert-manager         Active   2m47s
default              Active   3m31s
kube-node-lease      Active   3m33s
kube-public          Active   3m33s
kube-system          Active   3m33s
local-path-storage   Active   3m28s
```

```console
# kubectl  get po -n capi-system
NAME                                       READY   STATUS    RESTARTS   AGE
capi-controller-manager-5b74fcc774-wpxn7   1/1     Running   0          64s
```
### Clone CAPN

```console
git clone https://github.com/kubernetes-sigs/cluster-api-provider-nested
cd cluster-api-provider-nested
```

### Create Docker Images, Manifests and Load Images

```console
PULL_POLICY=Never TAG=dev make docker-build release-manifests
```

```console
# For Kind
kind load docker-image gcr.io/cluster-api-nested-controller-amd64:dev --name=capn
kind load docker-image gcr.io/nested-controlplane-controller-amd64:dev --name=capn
```

```console
# For Minikube
minikube image load gcr.io/nested-controlplane-controller-amd64:dev
minikube image load gcr.io/cluster-api-nested-controller-amd64:dev
```

### Deploy CAPN

Next, we will deploy the CAPN related CRDs and controllers.

```console
kubectl apply -f out/cluster-api-provider-nested-components.yaml 
```

This will help deploy two controllers:
- Cluster API Nested Controller under namesapce `capn-system`.
- Cluster API Nested Control Plane under namespace `capn-nested-control-plane-system`.

```console
# kubectl  get ns
NAME                               STATUS   AGE
capi-system                        Active   17m
capn-nested-control-plane-system   Active   5s
capn-system                        Active   5s
cert-manager                       Active   19m
default                            Active   19m
kube-node-lease                    Active   19m
kube-public                        Active   19m
kube-system                        Active   19m
local-path-storage                 Active   19m
```

```console
# kubectl  get po -n capn-nested-control-plane-system
NAME                                                           READY   STATUS    RESTARTS   AGE
capn-nested-control-plane-controller-manager-8865cdc4f-787h5   2/2     Running   0          36s
```

```console
# kubectl get po -n capn-system
NAME                                       READY   STATUS    RESTARTS   AGE
capn-controller-manager-6fb7bdd57d-7v77s   2/2     Running   0          50s
```
### Apply Sample Tenant Cluster

```console
kubectl apply -f config/samples/
```

After the cluster was created, you will be able to see the cluster was provisioned, and all the pods for tenant cluster including apiserver, controller manager and etcd will be running.

```console
# kubectl  get cluster
NAME             PHASE
cluster-sample   Provisioned
```

```console
# kubectl  get pods
NAME                                  READY   STATUS    RESTARTS   AGE
cluster-sample-apiserver-0            1/1     Running   0          160m
cluster-sample-controller-manager-0   1/1     Running   1          160m
cluster-sample-etcd-0                 1/1     Running   0          6h4m
```

If you found you cluster keeps provisioning and all of the Tenant Cluster components including APIServer, Controller and Etcd keeps crashed due to docker image pull rate limit as follows:

```console
# kubectl  get cluster
NAME             PHASE
cluster-sample   Provisioning
```

```console
# kubectl  get po
NAME                                  READY   STATUS             RESTARTS   AGE
cluster-sample-apiserver-0            0/1     ImagePullBackOff   0          6m39s
cluster-sample-controller-manager-0   0/1     ImagePullBackOff   0          6m56s
cluster-sample-etcd-0                 0/1     ImagePullBackOff   0          6m45s
```
```console
# kubectl  describe pod cluster-sample-apiserver-0
Name:         cluster-sample-apiserver-0
Namespace:    default
Priority:     0
Node:         capn-control-plane/172.18.0.2
Start Time:   Mon, 19 Jul 2021 23:53:45 -0700
Labels:       component-name=nestedapiserver-sample
              controller-revision-hash=cluster-sample-apiserver-57bbbd9b49
              statefulset.kubernetes.io/pod-name=cluster-sample-apiserver-0
...
Events:
  Type     Reason     Age                    From                         Message
  ----     ------     ----                   ----                         -------
  Normal   Scheduled  6m47s                  default-scheduler            Successfully assigned default/cluster-sample-apiserver-0 to capn-control-plane
  Normal   Pulling    5m8s (x4 over 6m47s)   kubelet, capn-control-plane  Pulling image "virtualcluster/apiserver-v1.16.2"
  Warning  Failed     5m5s (x4 over 6m44s)   kubelet, capn-control-plane  Failed to pull image "virtualcluster/apiserver-v1.16.2": rpc error: code = Unknown desc = failed to pull and unpack image "docker.io/virtualcluster/apiserver-v1.16.2:latest": failed to copy: httpReadSeeker: failed open: unexpected status code https://registry-1.docker.io/v2/virtualcluster/apiserver-v1.16.2/manifests/sha256:81fc8bb510b07535525413b725aed05765b56961c1f4ed28b92ba30acd65f6fb: 429 Too Many Requests - Server message: toomanyrequests: You have reached your pull rate limit. You may increase the limit by authenticating and upgrading: https://www.docker.com/increase-rate-limit
  Warning  Failed     5m5s (x4 over 6m44s)   kubelet, capn-control-plane  Error: ErrImagePull
  Warning  Failed     4m53s (x6 over 6m44s)  kubelet, capn-control-plane  Error: ImagePullBackOff
  Normal   BackOff    106s (x19 over 6m44s)  kubelet, capn-control-plane  Back-off pulling image "virtualcluster/apiserver-v1.16.2"
```

Please follow the following guidnace to workaround:

```console
# Kind
kind load docker-image docker.io/virtualcluster/apiserver-v1.16.2:latest --name=capn
kind load docker-image docker.io/virtualcluster/controller-manager-v1.16.2:latest --name=capn
kind load docker-image docker.io/virtualcluster/etcd-v3.4.0:latest --name=capn
```

```console
# Minikube
minikube image load docker.io/virtualcluster/apiserver-v1.16.2:latest
minikube image load docker.io/virtualcluster/controller-manager-v1.16.2:latest
minikube image load docker.io/virtualcluster/etcd-v3.4.0:latest
```

Get all of the StatefulSet for Tenant Cluster and update the `imagePullPolicy` to `Never`.

```console
# kubectl  get sts
NAME                                READY   AGE
cluster-sample-apiserver            0/1     15m
cluster-sample-controller-manager   0/1     15m
cluster-sample-etcd                 0/1     15m
```

Delete all pods for above StatefulSet resources:

```console
# kubectl  delete po cluster-sample-apiserver-0 cluster-sample-controller-manager-0 cluster-sample-etcd-0 --force --grace-period=0
```

After above steps finsihed, you will be able to see the cluster was provisioned, and all the pods for tenant cluster including apiserver, controller manager and etcd will be running.

### Get `KUBECONFIG`

We will use the `clusterctl` command-line tool to generate the `KUBECONFIG`, which will be used to access the nested controlplane later.

```console
cd cluster-api
make clusterctl
./bin/clusterctl get kubeconfig cluster-sample > ../kubeconfig
cd ..
```

### Port Forward

To access the nested controlplane, in a separate shell, you will need to `port-forward` the apiserver service.

```console
kubectl port-forward svc/cluster-sample-apiserver 6443:6443
```

### Connect to Cluster

To use the `KUBECONFIG` created by `clusterctl` without modification, we first need to setup a host record for the apiserver service name, to do this need add following line to `/etc/hosts`.

```
127.0.0.1 cluster-sample-apiserver
```

### Connect to the Cluster! :tada:

```shell
kubectl --kubeconfig kubeconfig get all -A
```

### Clean Up

```shell
# For Kind
kind delete cluster --name=capn
```

```shell
# For Minikube
Minikube delete
```
