/*


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

package equality

import (
	batchv1 "k8s.io/api/batch/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

// JobConfigChanged checks if the current and expected Job match and if not,
// returns true and the expected job.
func JobConfigChanged(current, expected *batchv1.Job) (bool, *batchv1.Job) {
	changed := false
	updated := current.DeepCopy()

	if !apiequality.Semantic.DeepEqual(current.Name, expected.Name) {
		updated = expected
		changed = true
	}

	if !apiequality.Semantic.DeepEqual(current.Namespace, expected.Namespace) {
		updated = expected
		changed = true
	}

	if !apiequality.Semantic.DeepEqual(current.Labels, expected.Labels) {
		updated = expected
		changed = true
	}

	if !apiequality.Semantic.DeepEqual(current.Spec, expected.Spec) {
		updated = expected
		changed = true
	}

	if !changed {
		return false, nil
	}

	return true, updated
}
