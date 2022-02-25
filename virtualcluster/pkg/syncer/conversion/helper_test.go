package conversion

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis/tenancy/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
)

func TestToClusterKey(t *testing.T) {
	for _, tt := range []struct {
		name        string
		vc          *v1alpha1.VirtualCluster
		expectedKey string
	}{
		{
			name: "normal vc",
			vc: &v1alpha1.VirtualCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
					UID:       "d64ea0c0-91f8-46f5-8643-c0cab32ab0cd",
				},
			},
			expectedKey: "ns-fd1b34-name",
		},
	} {
		t.Run(tt.name, func(tc *testing.T) {
			key := ToClusterKey(tt.vc)
			if key != tt.expectedKey {
				tc.Errorf("expected key %s, got %s", tt.expectedKey, key)
			}
		})
	}
}

func TestIsControlPlaneService(t *testing.T) {
	type args struct {
		service *v1.Service
		cluster string
	}
	tests := []struct {
		name           string
		args           args
		featureEnabled bool
		want           bool
	}{
		{
			"TestDefaultKubernetesService",
			args{
				&v1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "test-default", Name: "kubernetes"}},
				"test",
			},
			false,
			true,
		},
		{
			"TestDefaultNginxService",
			args{
				&v1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "test-default", Name: "nginx"}},
				"test",
			},
			false,
			false,
		},
		{
			"TestClusterAPIServiceSVC",
			args{
				&v1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "apiserver-svc"}},
				"test",
			},
			true,
			true,
		},
		{
			"TestDefaultNginx",
			args{
				&v1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "nginx"}},
				"test",
			},
			true,
			false,
		},
	}
	for _, tt := range tests {
		// Flip feature gate on and off
		gates := map[string]bool{featuregate.SuperClusterServiceNetwork: tt.featureEnabled}
		featuregate.DefaultFeatureGate, _ = featuregate.NewFeatureGate(gates)

		t.Run(tt.name, func(t *testing.T) {
			if got := IsControlPlaneService(tt.args.service, tt.args.cluster); got != tt.want {
				t.Errorf("IsControlPlaneService() = %v, want %v", got, tt.want)
			}
		})
	}
}
