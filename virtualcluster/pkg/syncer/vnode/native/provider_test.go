package native

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vnodeprovider "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/vnode/provider"
)

func newNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"a": "test-0",
				"b": "test-1",
			},
		},
	}
}

func Test_provider_GetNodeLabels(t *testing.T) {
	type fields struct {
		labelsToSync map[string]struct{}
	}
	type args struct {
		node *corev1.Node
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   map[string]string
	}{
		{
			name:   "TestWithNoLabels",
			fields: fields{},
			args:   args{newNode()},
			want: map[string]string{
				"tenancy.x-k8s.io/virtualnode": "true",
			},
		},
		{
			name: "TestWithOneLabel",
			fields: fields{
				labelsToSync: map[string]struct{}{
					"a": {},
				},
			},
			args: args{newNode()},
			want: map[string]string{
				"tenancy.x-k8s.io/virtualnode": "true",
				"a":                            "test-0",
			},
		},
		{
			name: "TestWithManyLabels",
			fields: fields{
				labelsToSync: map[string]struct{}{
					"a": {},
					"b": {},
					"c": {},
				},
			},
			args: args{newNode()},
			want: map[string]string{
				"tenancy.x-k8s.io/virtualnode": "true",
				"a":                            "test-0",
				"b":                            "test-1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewNativeVirtualNodeProvider(8080, tt.fields.labelsToSync)
			got := vnodeprovider.GetNodeLabels(p, tt.args.node)
			if len(tt.want) != 0 && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("vnodeprovider.GetNodeLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}
