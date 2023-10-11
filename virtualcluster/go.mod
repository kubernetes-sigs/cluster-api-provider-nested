module sigs.k8s.io/cluster-api-provider-nested/virtualcluster

go 1.16

require (
	github.com/aliyun/alibaba-cloud-sdk-go v1.60.324
	github.com/emicklei/go-restful v2.9.6+incompatible
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	go.uber.org/zap v1.17.0
	golang.org/x/net v0.17.0
	k8s.io/api v0.21.9
	k8s.io/apiextensions-apiserver v0.21.9
	k8s.io/apimachinery v0.21.9
	k8s.io/apiserver v0.21.9
	k8s.io/cli-runtime v0.21.9
	k8s.io/client-go v0.21.9
	k8s.io/code-generator v0.21.9
	k8s.io/component-base v0.21.9
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/cluster-api v0.4.0-beta.0
	sigs.k8s.io/controller-runtime v0.9.0
)

replace (
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
	github.com/moby/spdystream => github.com/moby/spdystream v0.2.0
	k8s.io/api => k8s.io/api v0.21.9
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.9
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.9
	k8s.io/apiserver => k8s.io/apiserver v0.21.9
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.21.9
	k8s.io/client-go => k8s.io/client-go v0.21.9
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.21.9
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.21.9
	k8s.io/code-generator => k8s.io/code-generator v0.21.9
	k8s.io/component-base => k8s.io/component-base v0.21.9
	k8s.io/component-helpers => k8s.io/component-helpers v0.21.9
	k8s.io/controller-manager => k8s.io/controller-manager v0.21.9
	k8s.io/cri-api => k8s.io/cri-api v0.21.9
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.21.9
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.21.9
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.21.9
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.21.9
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.21.9
	k8s.io/kubectl => k8s.io/kubectl v0.21.9
	k8s.io/kubelet => k8s.io/kubelet v0.21.9
	k8s.io/kubernetes => k8s.io/kubernetes v1.21.9
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.21.9
	k8s.io/metrics => k8s.io/metrics v0.21.9
	k8s.io/mount-utils => k8s.io/mount-utils v0.21.9
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.21.9
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.21.9
)
