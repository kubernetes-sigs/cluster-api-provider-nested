## Quick Start 

This tutorial introduces how to create a nested controlplane. CAPN should work with any standard 
Kubernetes cluster out of box, but for demo purposes, in this tutorial, we will use 
a `kind` cluster as the management cluster as well as the nested workload cluster.

### Prerequisites

Please install the latest version of [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) 
and [kubectl](https://kubernetes.io/docs/tasks/tools/)

### Create `kind` cluster

```console
kind create cluster --name=capn
```

### Clone CAPI and build `clusterctl` tool

As a Cluster API (CAPI) provider, CAPN needs the `clusterctl` binary to install and manage clusters.

```console
git clone git@github.com:kubernetes-sigs/cluster-api.git
cd cluster-api
make clusterctl
```

### Init control plane, infrastructure etc

If you aren't familar with CAPI & `clusterctl` this command will deploy the core components, as well as the Nested components for infra providers and for control plane providers. 

```console
./bin/clusterctl init --core cluster-api:v0.4.0  --control-plane nested:v0.1.0  --infrastructure nested:v0.1.0
```

you should see something like:
```
Fetching providers
Installing cert-manager Version="v1.1.0"
Waiting for cert-manager to be available...
Installing Provider="cluster-api" Version="v0.4.0" TargetNamespace="capi-system"
Installing Provider="bootstrap-kubeadm" Version="v0.4.0" TargetNamespace="capi-kubeadm-bootstrap-system"
Installing Provider="control-plane-nested" Version="v0.1.0" TargetNamespace="capn-nested-control-plane-system"
Installing Provider="infrastructure-nested" Version="v0.1.0" TargetNamespace="capn-system"

Your management cluster has been initialized successfully!

You can now create your first workload cluster by running the following:

  clusterctl generate cluster [name] --kubernetes-version [version] | kubectl apply -f -
```

and wait for all pods to be `Running` before proceed to next step:
```console
$ kubectl get pods --all-namespaces
NAMESPACE                          NAME                                                            READY   STATUS    RESTARTS   AGE
capi-kubeadm-bootstrap-system      capi-kubeadm-bootstrap-controller-manager-c59c94d6f-l8f4v       1/1     Running   0          4m45s
capi-system                        capi-controller-manager-6c555b545d-rtw8k                        1/1     Running   0          4m46s
capn-nested-control-plane-system   capn-nested-control-plane-controller-manager-698c444c6d-nhddn   2/2     Running   0          4m45s
capn-system                        capn-controller-manager-7f9757b67f-cp8d9                        2/2     Running   0          4m43s
cert-manager                       cert-manager-5597cff495-gwgr5                                   1/1     Running   0          5m7s
cert-manager                       cert-manager-cainjector-bd5f9c764-vvbjf                         1/1     Running   0          5m7s
cert-manager                       cert-manager-webhook-5f57f59fbc-ccg5k                           1/1     Running   0          5m7s
kube-system                        coredns-74ff55c5b-9pspc                                         1/1     Running   0          6m16s
kube-system                        coredns-74ff55c5b-nqnk9                                         1/1     Running   0          6m16s
kube-system                        etcd-capn-control-plane                                         1/1     Running   0          6m29s
kube-system                        kindnet-9g9z4                                                   1/1     Running   0          6m16s
kube-system                        kube-apiserver-capn-control-plane                               1/1     Running   0          6m29s
kube-system                        kube-controller-manager-capn-control-plane                      1/1     Running   0          6m29s
kube-system                        kube-proxy-jl46r                                                1/1     Running   0          6m16s
kube-system                        kube-scheduler-capn-control-plane                               1/1     Running   0          6m29s
local-path-storage                 local-path-provisioner-78776bfc44-qcx49                         1/1     Running   0          6m16s
```

### Set clustername (in our example, we set clustername to `cluster-sample`)

```console
export CLUSTER_NAME=cluster-sample
```

### Generate custom resource (`Cluster`, `NestedCluster` etc) and apply to our cluster

```console
./bin/clusterctl generate cluster ${CLUSTER_NAME} --infrastructure=nested:v0.1.0 | kubectl apply -f -
```

### Get `KUBECONFIG`

We will use the `clusterctl` command-line tool to generate the `KUBECONFIG`, which 
will be used to access the nested controlplane later.

```console
./bin/clusterctl get kubeconfig ${CLUSTER_NAME} > ../kubeconfig
```

This error means that the status of the cluster is not ready.
```
Error: "cluster-sample-kubeconfig" not found in namespace "default": secrets "cluster-sample-kubeconfig" not found
```

Run following command and make sure the `Ready` is true before retry above command.
```console
$ kubectl get nestedcluster -w
NAME             READY   AGE
cluster-sample   true    26h
```

### Make sure `cluster-sample` related pods are running before proceed.

```console
$ kubectl get pods
NAME                                  READY   STATUS    RESTARTS   AGE
cluster-sample-apiserver-0            1/1     Running   0          25h
cluster-sample-controller-manager-0   1/1     Running   0          25h
cluster-sample-etcd-0                 1/1     Running   0          25h
```

### Port Forward

To access the nested controlplane, in a separate shell, you will need 
to `port-forward` the apiserver service.

```console
$ kubectl port-forward svc/cluster-sample-apiserver 6443:6443
Forwarding from 127.0.0.1:6443 -> 6443
Forwarding from [::1]:6443 -> 6443
```

### Connect to Cluster

To use the `KUBECONFIG` created by `clusterctl` without modification, we first 
need to setup a host record for the apiserver service name, to do this need add
following line to `/etc/hosts`.

```
127.0.0.1 cluster-sample-apiserver
```

### Connect to the Cluster! :tada:

```console
$ kubectl --kubeconfig kubeconfig get all -A
NAMESPACE   NAME                 TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)   AGE
default     service/kubernetes   ClusterIP   10.32.0.1    <none>        443/TCP   25h

```

### Clean Up

```console
kind delete cluster --name=capn
```
