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
	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

// JobConfigChanged checks if the current and expected Job match and if not,
// returns true and the expected job.
func JobConfigChanged(current, expected *batchv1.Job) (*batchv1.Job, bool) {
	changed := false
	updated := current.DeepCopy()

	if !apiequality.Semantic.DeepEqual(current.Labels, expected.Labels) {
		updated = expected
		changed = true
	}

	if !apiequality.Semantic.DeepEqual(current.Spec.Parallelism, expected.Spec.Parallelism) {
		updated = expected
		changed = true
	}

	if !apiequality.Semantic.DeepEqual(current.Spec.BackoffLimit, expected.Spec.BackoffLimit) {
		updated = expected
		changed = true
	}

	// The completions field is immutable, so no need to compare. Ignore job-generated
	// labels and only check the presence of the contour owning label.
	if current.Spec.Template.Labels != nil {
		if _, ok := current.Spec.Template.Labels[operatorv1alpha1.OwningContourLabel]; !ok {
			updated = expected
			changed = true
		}
	}

	if !apiequality.Semantic.DeepEqual(current.Spec.Template.Spec, expected.Spec.Template.Spec) {
		updated = expected
		changed = true
	}

	if !changed {
		return nil, false
	}

	return updated, true
}
