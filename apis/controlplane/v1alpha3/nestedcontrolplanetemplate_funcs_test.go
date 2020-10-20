/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	"testing"
)

func TestKubernetesVersion_SupportedVersionChange(t *testing.T) {
	kv := KubernetesVersion{
		Major: "1",
		Minor: "18",
		Patch: "1",
	}

	type fields struct {
		Major string
		Minor string
		Patch string
	}
	type args struct {
		old KubernetesVersion
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"TestValidKubernetesVersionSame", fields{Major: "1", Minor: "18", Patch: "1"}, args{kv}, false},
		{"TestValidKubernetesVersionChange", fields{Major: "1", Minor: "18", Patch: "2"}, args{kv}, false},
		{"TestValidKubernetesVersionPatchChangeWithLeadingZero", fields{Major: "1", Minor: "18", Patch: "09"}, args{kv}, false},
		{"TestInvalidKubernetesVersionMajorChange", fields{Major: "2", Minor: "18", Patch: "1"}, args{kv}, true},
		{"TestInvalidKubernetesVersionMinorChange", fields{Major: "1", Minor: "19", Patch: "1"}, args{kv}, true},
		{"TestInvalidKubernetesVersionPatchChange", fields{Major: "1", Minor: "18", Patch: "0"}, args{kv}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			new := &KubernetesVersion{
				Major: tt.fields.Major,
				Minor: tt.fields.Minor,
				Patch: tt.fields.Patch,
			}
			if err := new.SupportedVersionChange(tt.args.old); (err != nil) != tt.wantErr {
				t.Errorf("KubernetesVersion.SupportedVersionChange() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
