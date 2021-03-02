// Copyright Project Contour Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slice

import (
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	gatewayv1alpha1 "sigs.k8s.io/gateway-api/apis/v1alpha1"
)

// RemoveString returns a newly created []string that contains all items from slice that
// are not equal to s.
func RemoveString(slice []string, s string) []string {
	var newSlice []string
	for _, item := range slice {
		if item != s {
			newSlice = append(newSlice, item)
		}
	}
	return newSlice
}

// RemoveGatewayListener returns a newly created []gatewayv1alpha1.Listener
// that contains all items from slice that are not equal to l.
func RemoveGatewayListener(slice []gatewayv1alpha1.Listener, l gatewayv1alpha1.Listener) []gatewayv1alpha1.Listener {
	var newSlice []gatewayv1alpha1.Listener
	for _, item := range slice {
		if !apiequality.Semantic.DeepEqual(item, l) {
			newSlice = append(newSlice, item)
		}
	}
	return newSlice
}

// ContainsString checks if a given slice of strings contains the provided string.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ContainsInt32 checks if a given int32 slice contains the provided int32.
func ContainsInt32(slice []int32, i int32) bool {
	for _, item := range slice {
		if item == i {
			return true
		}
	}
	return false
}
