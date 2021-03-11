module sigs.k8s.io/cluster-api-provider-nested

go 1.15

require (
	github.com/go-logr/logr v0.3.0
	github.com/onsi/ginkgo v1.14.1
	github.com/onsi/gomega v1.10.2
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v0.19.2
	k8s.io/klog/v2 v2.2.0
	sigs.k8s.io/cluster-api v0.3.11-0.20201112165251-91b70900dbaf
	sigs.k8s.io/controller-runtime v0.7.0
	sigs.k8s.io/kubebuilder-declarative-pattern v0.0.0-20210120001158-a905f3c7cf41
)
