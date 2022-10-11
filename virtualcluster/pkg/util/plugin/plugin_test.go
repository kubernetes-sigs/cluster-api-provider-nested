/*
Copyright 2022 The Kubernetes Authors.

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

package plugin

import (
	"reflect"
	"testing"
)

func none(id string) func(ctx *InitContext) (interface{}, error) {
	return func(ctx *InitContext) (interface{}, error) { return id, nil }
}

func TestResourceRegister_List(t *testing.T) {
	tests := map[string]struct {
		resources []*Registration
		want      []string
	}{
		"empty": {
			resources: []*Registration{},
			want:      []string{},
		},
		"sorted": {
			resources: []*Registration{
				{ID: "00_test", InitFn: none("test")},
				{ID: "01_test", InitFn: none("another test")},
			},
			want: []string{
				"00_test",
				"01_test",
			},
		},
		"reversed": {
			resources: []*Registration{
				{ID: "01_test", InitFn: none("another test")},
				{ID: "00_test", InitFn: none("test")},
			},
			want: []string{
				"00_test",
				"01_test",
			},
		},
		"three elements": {
			resources: []*Registration{
				{ID: "01_test", InitFn: none("another test")},
				{ID: "00_test", InitFn: none("test")},
				{ID: "02_test", InitFn: none("other test"), Disable: true},
			},
			want: []string{
				"00_test",
				"01_test",
				"02_test",
			},
		},
	}
	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			var reg ResourceRegister
			for _, r := range tt.resources {
				reg.Register(r)
			}
			gotPlugins := reg.List()
			got := make([]string, 0, len(gotPlugins))
			for _, gotPlugin := range gotPlugins {
				got = append(got, gotPlugin.ID)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("List() = %v, want %v", got, tt.want)
			}
		})
	}
}
