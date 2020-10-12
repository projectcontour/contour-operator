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

package contour

import (
	"context"
	"fmt"
	"time"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	oputil "github.com/projectcontour/contour-operator/util"
	utilequality "github.com/projectcontour/contour-operator/util/equality"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
)

const (
	jobContainerName = "contour"
	jobNsEnvVar      = "CONTOUR_NAMESPACE"
)

// TODO [danehans]: The real dependency is whether the TLS secrets are present.
// The method should first check for the secrets, then use certgen as a secret
// generating strategy.
// ensureJob ensures that a Job exists for the given contour.
func (r *Reconciler) ensureJob(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	desired, err := DesiredJob(contour, r.Config.ContourImage)
	if err != nil {
		return fmt.Errorf("failed to build job: %w", err)
	}

	current, err := r.currentJob(ctx, contour)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Client.Create(ctx, desired); err != nil {
				return fmt.Errorf("failed to create job %s/%s: %w", desired.Namespace, desired.Name, err)
			}
			r.Log.Info("created job", "namespace", desired.Namespace, "name", desired.Name)
			return nil
		}
		return fmt.Errorf("failed to get job %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	if err := r.recreateJobIfNeeded(ctx, current, desired); err != nil {
		return fmt.Errorf("failed to recreate job %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	return nil
}

// ensureJobDeleted ensures the Job for the provided contour is deleted.
func (r *Reconciler) ensureJobDeleted(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	key := types.NamespacedName{
		Namespace: contour.Spec.Namespace.Name,
		Name:      contour.Name + "-certgen",
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: key.Namespace,
			Name:      key.Name,
		},
	}

	if err := r.Client.Delete(ctx, job); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}
	r.Log.Info("deleted job", "namespace", job.Namespace, "name", job.Name)

	return nil
}

// currentJob returns the current Job resource for the provided contour.
func (r *Reconciler) currentJob(ctx context.Context, contour *operatorv1alpha1.Contour) (*batchv1.Job, error) {
	current := &batchv1.Job{}
	key := types.NamespacedName{
		Namespace: contour.Spec.Namespace.Name,
		Name:      contour.Name + "-certgen",
	}
	err := r.Client.Get(ctx, key, current)
	if err != nil {
		return nil, err
	}
	return current, nil
}

// DesiredJob generates the desired Job for the given contour.
func DesiredJob(contour *operatorv1alpha1.Contour, image string) (*batchv1.Job, error) {
	env := corev1.EnvVar{
		Name: jobNsEnvVar,
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				APIVersion: "v1",
				FieldPath:  "metadata.namespace",
			},
		},
	}

	container := corev1.Container{
		Name:            jobContainerName,
		Image:           image,
		ImagePullPolicy: corev1.PullAlways,
		Command: []string{
			"contour",
			"certgen",
			"--kube",
			"--incluster",
			"--overwrite",
			"--secrets-format=compact",
			fmt.Sprintf("--namespace=$(%s)", jobNsEnvVar),
		},
		Env:                      []corev1.EnvVar{env},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
	}

	spec := corev1.PodSpec{
		Containers:                    []corev1.Container{container},
		DeprecatedServiceAccount:      defaultCertGenRbacName,
		ServiceAccountName:            defaultCertGenRbacName,
		SecurityContext:               oputil.NewUnprivilegedPodSecurity(),
		RestartPolicy:                 corev1.RestartPolicyNever,
		DNSPolicy:                     corev1.DNSClusterFirst,
		SchedulerName:                 "default-scheduler",
		TerminationGracePeriodSeconds: pointer.Int64Ptr(int64(30)),
	}

	// TODO [danehans] certgen needs to be updated to match these labels.
	// See https://github.com/projectcontour/contour/issues/1821 for details.
	labels := map[string]string{
		"app.kubernetes.io/name":       "contour-certgen",
		"app.kubernetes.io/instance":   contour.Name,
		"app.kubernetes.io/component":  "ingress-controller",
		"app.kubernetes.io/part-of":    "project-contour",
		"app.kubernetes.io/managed-by": "contour-operator",
		// associate the job with the provided contour.
		operatorv1alpha1.OwningContourLabel: contour.Name,
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      contour.Name + "-certgen",
			Namespace: contour.Spec.Namespace.Name,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Parallelism:  pointer.Int32Ptr(int32(1)),
			Completions:  pointer.Int32Ptr(int32(1)),
			BackoffLimit: pointer.Int32Ptr(int32(1)),
			// Make job eligible to for immediate deletion (feature gate dependant).
			TTLSecondsAfterFinished: pointer.Int32Ptr(int32(0)),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: contourOwningSelector(contour).MatchLabels,
				},
				Spec: spec,
			},
		},
	}

	return job, nil
}

// recreateJobIfNeeded recreates a Job if current doesn't match desired.
func (r *Reconciler) recreateJobIfNeeded(ctx context.Context, current, desired *batchv1.Job) error {
	updated, changed := utilequality.JobConfigChanged(current, desired)
	if !changed {
		r.Log.Info("current job matches desired state; skipped updating",
			"namespace", current.Namespace, "name", current.Name)
		return nil
	}

	if err := r.Client.Delete(ctx, updated); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	// Retry is needed since the object may still be getting deleted.
	if err := r.retryJobCreate(ctx, updated, time.Second*3); err != nil {
		return err
	}
	r.Log.Info("recreated job", "namespace", updated.Namespace, "name", updated.Name)

	return nil
}

// retryJobCreate tries creating the provided Job, retrying every second
// until timeout is reached.
func (r *Reconciler) retryJobCreate(ctx context.Context, job *batchv1.Job, timeout time.Duration) error {
	err := wait.PollImmediate(1*time.Second, timeout, func() (bool, error) {
		if err := r.Client.Create(ctx, job); err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to create job %s/%s: %w", job.Namespace, job.Name, err)
	}
	return nil
}
