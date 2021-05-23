## Quick Start 

This tutorial introduces how to create a nested controlplane. CAPN should work with any standard 
Kubernetes cluster out of box, but for demo purposes, in this tutorial, we will use 
a `kind` cluster as the management cluster as well as the nested workload cluster.

### Prerequisites

Please install the latest version of [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) 
and [kubectl](https://kubernetes.io/docs/tasks/tools/)

### Clone CAPN

```shell
git clone https://github.com/kubernetes-sigs/cluster-api-provider-nested
cd cluster-api-provider-nested
```

### Create `kind` cluster

```shell
kind create cluster --name=capn
```

### Install `cert-manager`

Cert Manager is a soft dependency for the Cluster API components to enable mutating 
and validating webhooks to be auto deployed. For more detailed instructions 
go [Cert Manager Installion](https://cert-manager.io/docs/installation/kubernetes/#installing-with-regular-manifests).

```shell
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
```

### Clone CAPI and Deploy Dev release

As a cluster API~(CAPI) provider, CAPN requires core components of CAPI to be setup. 
We need to deploy the unreleased version of CAPI for `v1alpha4` API support.

```shell
git clone git@github.com:kubernetes-sigs/cluster-api.git
cd cluster-api
make release-manifests
# change feature flags on core
sed -i'' -e 's@- --feature-gates=.*@- --feature-gates=MachinePool=false,ClusterResourceSet=true@' out/core-components.yaml
kubectl apply -f out/core-components.yaml
cd ..
```

### Create Docker Images, Manifests and Load Images

```shell
PULL_POLICY=Never TAG=dev make docker-build release-manifests
kind load docker-image gcr.io/cluster-api-nested-controller-amd64:dev --name=capn
kind load docker-image gcr.io/nested-controlplane-controller-amd64:dev --name=capn
```

### Deploy CAPN

Next, we will deploy the CAPN related CRDs and controllers.

```shell
kubectl apply -f out/cluster-api-provider-nested-components.yaml 
```

### Apply Sample Cluster

```shell
kubectl apply -f config/samples/
```

### Get `KUBECONFIG`

We will use the `clusterctl` command-line tool to generate the `KUBECONFIG`, which 
will be used to access the nested controlplane later.

```shell
cd cluster-api
make clusterctl
./bin/clusterctl get kubeconfig cluster-sample > ../kubeconfig
cd ..
```

### Port Forward

To access the nested controlplane, in a separate shell, you will need 
to `port-forward` the apiserver service.

```shell
kubectl port-forward svc/cluster-sample-apiserver 6443:6443
```

### Connect to Cluster

To use the `KUBECONFIG` created by `clusterctl` without modification, we first 
need to setup a host record for the apiserver service name, to do this we can 
define a custom hosts file by setting the `HOSTALIASES` env and append the 
IP-address-to-URL mapping to the hosts file.

```shell
echo '127.0.0.1 cluster-sample-apiserver' >> ~/.hosts
export HOSTALIASES=~/.hosts
```

### Connect to the Cluster! :tada:

```shell
kubectl --kubeconfig kubeconfig get all -A
```

### Clean Up

```shell
kind delete cluster --name=capn
```
