module sigs.k8s.io/cluster-api-provider-nested

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.14.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.21.9
	k8s.io/apimachinery v0.21.9
	k8s.io/client-go v0.21.9
	k8s.io/component-base v0.21.2 // indirect
	k8s.io/klog/v2 v2.10.0
	sigs.k8s.io/cluster-api v0.4.0
	sigs.k8s.io/controller-runtime v0.9.3
	sigs.k8s.io/kubebuilder-declarative-pattern v0.0.0-20210630174303-f77bb4933dfb
)
