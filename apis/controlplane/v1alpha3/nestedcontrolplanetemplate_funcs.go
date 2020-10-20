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
	"fmt"
	"strconv"
)

// SupportedVersionChange will validate the Kubernetes Version change against
// the previous versions. You cannot change Kubernetes Major or Minor versions
// if you change Patch versions you can only increase patch.
//
// Kubernetes v1.18.1:
//   Major: 1
//   Minor: 18
//   Patch: 1
func (new *KubernetesVersion) SupportedVersionChange(old KubernetesVersion) error {
	if new.Major != old.Major {
		return fmt.Errorf("can't change Kubernetes major version")
	}
	if new.Minor != old.Minor {
		return fmt.Errorf("can't change Kubernetes Minor version")
	}

	newPatch, err := strconv.Atoi(new.Patch)
	if err != nil {
		return fmt.Errorf("can't parse new Kubernetes patch version")
	}
	oldPatch, err := strconv.Atoi(old.Patch)
	if err != nil {
		return fmt.Errorf("can't parse old Kubernetes patch version")
	}

	if newPatch < oldPatch {
		return fmt.Errorf("can't downgrade Kubernetes Patch version")
	}
	return nil
}
