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
	"path/filepath"

	operatorv1alpha1 "github.com/projectcontour/contour-operator/api/v1alpha1"
	"github.com/projectcontour/contour-operator/internal/equality"
	objutil "github.com/projectcontour/contour-operator/internal/object"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

const (
	// envoyDaemonSetName is the name of Envoy's DaemonSet resource.
	// [TODO] danehans: Remove and use contour.Name + "-envoy" when
	// https://github.com/projectcontour/contour/issues/2122 is fixed.
	envoyDaemonSetName = "envoy"
	// EnvoyContainerName is the name of the Envoy container.
	EnvoyContainerName = "envoy"
	// ShutdownContainerName is the name of the Shutdown Manager container.
	ShutdownContainerName = "shutdown-manager"
	// envoyInitContainerName is the name of the Envoy init container.
	envoyInitContainerName = "envoy-initconfig"
	// envoyNsEnvVar is the name of the contour namespace environment variable.
	envoyNsEnvVar = "CONTOUR_NAMESPACE"
	// envoyPodEnvVar is the name of the Envoy pod name environment variable.
	envoyPodEnvVar = "ENVOY_POD_NAME"
	// envoyCertsVolName is the name of the contour certificates volume.
	envoyCertsVolName = "envoycert"
	// envoyCertsVolMntDir is the directory name of the Envoy certificates volume.
	envoyCertsVolMntDir = "certs"
	// envoyCertsSecretName is the name of the secret used as the certificate volume source.
	envoyCertsSecretName = envoyCertsVolName
	// envoyCfgVolName is the name of the Envoy configuration volume.
	envoyCfgVolName = "envoy-config"
	// envoyCfgVolMntDir is the directory name of the Envoy configuration volume.
	envoyCfgVolMntDir = "config"
	// envoyCfgFileName is the name of the Envoy configuration file.
	envoyCfgFileName = "envoy.json"
	// xdsResourceVersion is the version of the Envoy xdS resource types.
	xdsResourceVersion = "v3"
)

// ensureDaemonSet ensures a DaemonSet exists for the given contour.
func (r *reconciler) ensureDaemonSet(ctx context.Context, contour *operatorv1alpha1.Contour) (*appsv1.DaemonSet, error) {
	desired := DesiredDaemonSet(contour, r.config.ContourImage, r.config.EnvoyImage)
	current, err := r.currentDaemonSet(ctx, contour)
	if err != nil {
		if errors.IsNotFound(err) {
			updated, err := r.createDaemonSet(ctx, desired)
			if err != nil {
				return nil, fmt.Errorf("failed to create daemonset %s/%s: %w", desired.Namespace, desired.Name, err)
			}
			return updated, nil
		}
		return nil, fmt.Errorf("failed to get daemonset %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	differ := equality.DaemonSetSelectorsDiffer(current, desired)
	if differ {
		r.log.Info("selectors differ and are immutable, recreating daemonset %s/%s",
			"namespace", current.Namespace, "name", current.Name)
		return nil, r.ensureDaemonSetDeleted(ctx, contour)
	}

	updated, err := r.updateDaemonSetIfNeeded(ctx, contour, current, desired)
	if err != nil {
		return nil, fmt.Errorf("failed to update daemonset for contour %s/%s: %w", contour.Namespace, contour.Name, err)
	}

	return updated, nil
}

// ensureDaemonSetDeleted ensures the DaemonSet for the provided contour is deleted
// if Contour owner labels exist.
func (r *reconciler) ensureDaemonSetDeleted(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	ds, err := r.currentDaemonSet(ctx, contour)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !ownerLabelsExist(ds, contour) {
		r.log.Info("daemonset not labeled; skipping deletion", "namespace", ds.Namespace, "name", ds.Name)
	} else {
		if err := r.client.Delete(ctx, ds); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		r.log.Info("deleted daemonset", "namespace", ds.Namespace, "name", ds.Name)
	}

	return nil
}

// DesiredDaemonSet returns the desired DaemonSet for the provided contour using
// contourImage as the shutdown-manager/envoy-initconfig container images and
// envoyImage as Envoy's container image.
func DesiredDaemonSet(contour *operatorv1alpha1.Contour, contourImage, envoyImage string) *appsv1.DaemonSet {
	labels := map[string]string{
		"app.kubernetes.io/name":     "contour",
		"app.kubernetes.io/instance": contour.Name,
		// The contourImage tag is used as the version.
		"app.kubernetes.io/version":    objutil.TagFromImage(contourImage),
		"app.kubernetes.io/component":  "ingress-controller",
		"app.kubernetes.io/managed-by": "contour-operator",
		// Associate the daemonset with the provided contour.
		operatorv1alpha1.OwningContourNsLabel:   contour.Namespace,
		operatorv1alpha1.OwningContourNameLabel: contour.Name,
	}

	containers := []corev1.Container{
		{
			Name:            ShutdownContainerName,
			Image:           contourImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command: []string{
				"/bin/contour",
			},
			Args: []string{
				"envoy",
				"shutdown-manager",
			},
			LivenessProbe: &corev1.Probe{
				FailureThreshold: int32(3),
				Handler: corev1.Handler{
					HTTPGet: &corev1.HTTPGetAction{
						Scheme: corev1.URISchemeHTTP,
						Path:   "/healthz",
						Port:   intstr.IntOrString{IntVal: int32(8090)},
					},
				},
				InitialDelaySeconds: int32(3),
				PeriodSeconds:       int32(10),
				SuccessThreshold:    int32(1),
				TimeoutSeconds:      int32(1),
			},
			Lifecycle: &corev1.Lifecycle{
				PreStop: &corev1.Handler{
					Exec: &corev1.ExecAction{
						Command: []string{"/bin/contour", "envoy", "shutdown"},
					},
				},
			},
			TerminationMessagePolicy: corev1.TerminationMessageReadFile,
			TerminationMessagePath:   "/dev/termination-log",
		},
		{
			Name:            EnvoyContainerName,
			Image:           envoyImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command: []string{
				"envoy",
			},
			Args: []string{
				"-c",
				filepath.Join("/", envoyCfgVolMntDir, envoyCfgFileName),
				fmt.Sprintf("--service-cluster $(%s)", envoyNsEnvVar),
				fmt.Sprintf("--service-node $(%s)", envoyPodEnvVar),
				"--log-level info",
			},
			Env: []corev1.EnvVar{
				{
					Name: envoyNsEnvVar,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "metadata.namespace",
						},
					},
				},
				{
					Name: envoyPodEnvVar,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "metadata.name",
						},
					},
				},
			},
			ReadinessProbe: &corev1.Probe{
				FailureThreshold: int32(3),
				Handler: corev1.Handler{
					HTTPGet: &corev1.HTTPGetAction{
						Scheme: corev1.URISchemeHTTP,
						Path:   "/ready",
						Port:   intstr.IntOrString{IntVal: int32(8002)},
					},
				},
				InitialDelaySeconds: int32(3),
				PeriodSeconds:       int32(4),
				SuccessThreshold:    int32(1),
				TimeoutSeconds:      int32(1),
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          "http",
					ContainerPort: int32(httpPort),
					// Required for kind/bare-metal deployments but unneeded otherwise.
					// TODO [danehans]: Remove when https://github.com/projectcontour/contour-operator/issues/70 merges.
					HostPort: int32(httpPort),
					Protocol: "TCP",
				},
				{
					Name:          "https",
					ContainerPort: int32(httpsPort),
					// Required for kind/bare-metal deployments but unneeded otherwise.
					// TODO [danehans]: Remove when https://github.com/projectcontour/contour-operator/issues/70 merges.
					HostPort: int32(httpsPort),
					Protocol: "TCP",
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      envoyCertsVolName,
					MountPath: filepath.Join("/", envoyCertsVolMntDir),
					ReadOnly:  true,
				},
				{
					Name:      envoyCfgVolName,
					MountPath: filepath.Join("/", envoyCfgVolMntDir),
					ReadOnly:  true,
				},
			},
			Lifecycle: &corev1.Lifecycle{
				PreStop: &corev1.Handler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/shutdown",
						Port:   intstr.FromInt(8090),
						Scheme: "HTTP",
					},
				},
			},
			TerminationMessagePolicy: corev1.TerminationMessageReadFile,
			TerminationMessagePath:   "/dev/termination-log",
		},
	}

	initContainers := []corev1.Container{
		{
			Name:            envoyInitContainerName,
			Image:           contourImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command: []string{
				"contour",
			},
			Args: []string{
				"bootstrap",
				filepath.Join("/", envoyCfgVolMntDir, envoyCfgFileName),
				"--xds-address=contour",
				fmt.Sprintf("--xds-port=%d", xdsPort),
				fmt.Sprintf("--xds-resource-version=%s", xdsResourceVersion),
				fmt.Sprintf("--resources-dir=%s", filepath.Join("/", envoyCfgVolMntDir, "resources")),
				fmt.Sprintf("--envoy-cafile=%s", filepath.Join("/", envoyCertsVolMntDir, "ca.crt")),
				fmt.Sprintf("--envoy-cert-file=%s", filepath.Join("/", envoyCertsVolMntDir, "tls.crt")),
				fmt.Sprintf("--envoy-key-file=%s", filepath.Join("/", envoyCertsVolMntDir, "tls.key")),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      envoyCertsVolName,
					MountPath: filepath.Join("/", envoyCertsVolMntDir),
					ReadOnly:  true,
				},
				{
					Name:      envoyCfgVolName,
					MountPath: filepath.Join("/", envoyCfgVolMntDir),
					ReadOnly:  false,
				},
			},
			Env: []corev1.EnvVar{
				{
					Name: envoyNsEnvVar,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							APIVersion: "v1",
							FieldPath:  "metadata.namespace",
						},
					},
				},
			},
			TerminationMessagePolicy: corev1.TerminationMessageReadFile,
			TerminationMessagePath:   "/dev/termination-log",
		},
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: contour.Spec.Namespace.Name,
			Name:      envoyDaemonSetName,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			RevisionHistoryLimit: pointer.Int32Ptr(int32(10)),
			// Ensure the deamonset adopts only its own pods.
			Selector: envoyDaemonSetPodSelector(),
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: pointerTo(intstr.FromString("10%")),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					// TODO [danehans]: Remove the prometheus annotations when Contour is updated to
					// show how the Prometheus Operator is used to scrape Contour/Envoy metrics.
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   "8002",
						"prometheus.io/path":   "/stats/prometheus",
					},
					Labels: envoyDaemonSetPodSelector().MatchLabels,
				},
				Spec: corev1.PodSpec{
					Containers:     containers,
					InitContainers: initContainers,
					Volumes: []corev1.Volume{
						{
							Name: envoyCertsVolName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									DefaultMode: pointer.Int32Ptr(int32(420)),
									SecretName:  envoyCertsSecretName,
								},
							},
						},
						{
							Name: envoyCfgVolName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					ServiceAccountName:            envoyRbacName,
					DeprecatedServiceAccount:      EnvoyContainerName,
					AutomountServiceAccountToken:  pointer.BoolPtr(false),
					TerminationGracePeriodSeconds: pointer.Int64Ptr(int64(300)),
					SecurityContext:               &corev1.PodSecurityContext{},
					DNSPolicy:                     corev1.DNSClusterFirst,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SchedulerName:                 "default-scheduler",
				},
			},
		},
	}

	return ds
}

// currentDaemonSet returns the current DaemonSet resource for the provided contour.
func (r *reconciler) currentDaemonSet(ctx context.Context, contour *operatorv1alpha1.Contour) (*appsv1.DaemonSet, error) {
	ds := &appsv1.DaemonSet{}
	key := types.NamespacedName{
		Namespace: contour.Spec.Namespace.Name,
		Name:      envoyDaemonSetName,
	}

	if err := r.client.Get(ctx, key, ds); err != nil {
		return nil, err
	}

	return ds, nil
}

// createDaemonSet creates a DaemonSet resource for the provided ds.
func (r *reconciler) createDaemonSet(ctx context.Context, ds *appsv1.DaemonSet) (*appsv1.DaemonSet, error) {
	if err := r.client.Create(ctx, ds); err != nil {
		return nil, fmt.Errorf("failed to create daemonset %s/%s: %w", ds.Namespace, ds.Name, err)
	}
	r.log.Info("created daemonset", "namespace", ds.Namespace, "name", ds.Name)

	return ds, nil
}

// updateDaemonSetIfNeeded updates a DaemonSet if current does not match desired,
// using contour to verify the existence of owner labels.
func (r *reconciler) updateDaemonSetIfNeeded(ctx context.Context, contour *operatorv1alpha1.Contour, current, desired *appsv1.DaemonSet) (*appsv1.DaemonSet, error) {
	if !ownerLabelsExist(current, contour) {
		r.log.Info("daemonset missing owner labels; skipped updating", "namespace", current.Namespace,
			"name", current.Name)
		return current, nil
	}
	ds, updated := equality.DaemonsetConfigChanged(current, desired)
	if updated {
		if err := r.client.Update(ctx, ds); err != nil {
			return nil, fmt.Errorf("failed to update daemonset %s/%s: %w", ds.Namespace, ds.Name, err)
		}
		r.log.Info("updated daemonset", "namespace", ds.Namespace, "name", ds.Name)
		return ds, nil
	}
	r.log.Info("daemonset unchanged; skipped updating",
		"namespace", current.Namespace, "name", current.Name)

	return current, nil
}
