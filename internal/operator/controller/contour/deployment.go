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
	equality "github.com/projectcontour/contour-operator/internal/equality"
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
	// contourDeploymentName is the name of Contour's Deployment resource.
	// [TODO] danehans: Remove and use contour.Name + "-contour" when
	// https://github.com/projectcontour/contour/issues/2122 is fixed.
	contourDeploymentName = "contour"
	// contourContainerName is the name of the Contour container.
	contourContainerName = "contour"
	// contourNsEnvVar is the name of the contour namespace environment variable.
	contourNsEnvVar = "CONTOUR_NAMESPACE"
	// contourPodEnvVar is the name of the contour pod name environment variable.
	contourPodEnvVar = "POD_NAME"
	// contourCertsVolName is the name of the contour certificates volume.
	contourCertsVolName = "contourcert"
	// contourCertsVolMntDir is the directory name of the contour certificates volume.
	contourCertsVolMntDir = "certs"
	// contourCertsSecretName is the name of the secret used as the certificate volume source.
	contourCertsSecretName = contourCertsVolName
	// contourCfgVolName is the name of the contour configuration volume.
	contourCfgVolName = "contour-config"
	// contourCfgVolMntDir is the directory name of the contour configuration volume.
	contourCfgVolMntDir = "config"
	// contourCfgFileName is the name of the contour configuration file.
	contourCfgFileName = "contour.yaml"
	// xdsPort is the network port number of Contour's xDS service.
	xdsPort = 8001
	// metricsPort is the network port number of Contour's metrics service.
	metricsPort = 8000
	// debugPort is the network port number of Contour's debug service.
	debugPort = 6060
)

// ensureDeployment ensures a deployment exists for the given contour.
func (r *reconciler) ensureDeployment(ctx context.Context, contour *operatorv1alpha1.Contour) (*appsv1.Deployment, error) {
	desired := DesiredDeployment(contour, r.config.ContourImage)
	current, err := r.currentDeployment(ctx, contour)
	if err != nil {
		if errors.IsNotFound(err) {
			updated, err := r.createDeployment(ctx, desired)
			if err != nil {
				return nil, fmt.Errorf("failed to create deployment %s/%s: %w", desired.Namespace, desired.Name, err)
			}
			return updated, nil
		}
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	differ := equality.DeploymentSelectorsDiffer(current, desired)
	if differ {
		r.log.Info("selectors differ and are immutable, recreating deployment %s/%s",
			"namespace", current.Namespace, "name", current.Name)
		return nil, r.ensureDeploymentDeleted(ctx, contour)
	}

	updated, err := r.updateDeploymentIfNeeded(ctx, contour, current, desired)
	if err != nil {
		return nil, fmt.Errorf("failed to update deployment %s/%s: %w", desired.Namespace, desired.Name, err)
	}

	return updated, nil
}

// ensureDeploymentDeleted ensures the deployment for the provided contour
// is deleted if Contour owner labels exist.
func (r *reconciler) ensureDeploymentDeleted(ctx context.Context, contour *operatorv1alpha1.Contour) error {
	deploy, err := r.currentDeployment(ctx, contour)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !ownerLabelsExist(deploy, contour) {
		r.log.Info("deployment not labeled; skipping deletion", "namespace", deploy.Namespace, "name", deploy.Name)
	} else {
		if err := r.client.Delete(ctx, deploy); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		r.log.Info("deleted deployment", "namespace", deploy.Namespace, "name", deploy.Name)
	}

	return nil
}

// DesiredDeployment returns the desired deployment for the provided contour using
// image as Contour's container image.
func DesiredDeployment(contour *operatorv1alpha1.Contour, image string) *appsv1.Deployment {
	args := []string{
		"serve",
		"--incluster",
		"--xds-address=0.0.0.0",
		fmt.Sprintf("--xds-port=%d", xdsPort),
		fmt.Sprintf("--contour-cafile=%s", filepath.Join("/", contourCertsVolMntDir, "ca.crt")),
		fmt.Sprintf("--contour-cert-file=%s", filepath.Join("/", contourCertsVolMntDir, "tls.crt")),
		fmt.Sprintf("--contour-key-file=%s", filepath.Join("/", contourCertsVolMntDir, "tls.key")),
		fmt.Sprintf("--config-path=%s", filepath.Join("/", contourCfgVolMntDir, contourCfgFileName)),
	}
	// Pass the insecure/secure flags to Contour if using non-default ports.
	for _, port := range contour.Spec.NetworkPublishing.Envoy.ContainerPorts {
		switch {
		case port.Name == "http" && port.PortNumber != envoyInsecureContainerPort:
			args = append(args, fmt.Sprintf("--envoy-service-http-port=%d", port.PortNumber))
		case port.Name == "https" && port.PortNumber != envoySecureContainerPort:
			args = append(args, fmt.Sprintf("--envoy-service-https-port=%d", port.PortNumber))
		}
	}

	container := corev1.Container{
		Name:            contourContainerName,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"contour"},
		Args:            args,
		Env: []corev1.EnvVar{
			{
				Name: contourNsEnvVar,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.namespace",
					},
				},
			},
			{
				Name: contourPodEnvVar,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.name",
					},
				},
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "xds",
				ContainerPort: xdsPort,
				Protocol:      "TCP",
			},
			{
				Name:          "metrics",
				ContainerPort: metricsPort,
				Protocol:      "TCP",
			},
			{
				Name:          "debug",
				ContainerPort: debugPort,
				Protocol:      "TCP",
			},
		},
		LivenessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Scheme: corev1.URISchemeHTTP,
					Path:   "/healthz",
					Port:   intstr.IntOrString{IntVal: int32(metricsPort)},
				},
			},
			TimeoutSeconds:   int32(1),
			PeriodSeconds:    int32(10),
			SuccessThreshold: int32(1),
			FailureThreshold: int32(3),
		},
		ReadinessProbe: &corev1.Probe{
			Handler: corev1.Handler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.IntOrString{
						IntVal: int32(xdsPort),
					},
				},
			},
			TimeoutSeconds:      int32(1),
			InitialDelaySeconds: int32(15),
			PeriodSeconds:       int32(10),
			SuccessThreshold:    int32(1),
			FailureThreshold:    int32(3),
		},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      contourCertsVolName,
				MountPath: filepath.Join("/", contourCertsVolMntDir),
				ReadOnly:  true,
			},
			{
				Name:      contourCfgVolName,
				MountPath: filepath.Join("/", contourCfgVolMntDir),
				ReadOnly:  true,
			},
		},
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: contour.Spec.Namespace.Name,
			Name:      contourDeploymentName,
			Labels:    makeDeploymentLabels(contour, objutil.TagFromImage(image)),
		},
		Spec: appsv1.DeploymentSpec{
			ProgressDeadlineSeconds: pointer.Int32Ptr(int32(600)),
			Replicas:                &contour.Spec.Replicas,
			RevisionHistoryLimit:    pointer.Int32Ptr(int32(10)),
			// Ensure the deployment adopts only its own pods.
			Selector: contourDeploymentPodSelector(),
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       pointerTo(intstr.FromString("50%")),
					MaxUnavailable: pointerTo(intstr.FromString("25%")),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					// TODO [danehans]: Remove the prometheus annotations when Contour is updated to
					// show how the Prometheus Operator is used to scrape Contour/Envoy metrics.
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   fmt.Sprintf("%d", metricsPort),
					},
					Labels: contourDeploymentPodSelector().MatchLabels,
				},
				Spec: corev1.PodSpec{
					// TODO [danehans]: Readdress anti-affinity when https://github.com/projectcontour/contour/issues/2997
					// is resolved.
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: int32(100),
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: "kubernetes.io/hostname",
										LabelSelector: &metav1.LabelSelector{
											MatchLabels: contourDeploymentPodSelector().MatchLabels,
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{container},
					Volumes: []corev1.Volume{
						{
							Name: contourCertsVolName,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									DefaultMode: pointer.Int32Ptr(int32(420)),
									SecretName:  contourCertsSecretName,
								},
							},
						},
						{
							Name: contourCfgVolName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										// [TODO] danehans: Update to contour.Name when
										// projectcontour/contour/issues/2122 is fixed.
										Name: contourCfgMapName,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  contourCfgFileName,
											Path: contourCfgFileName,
										},
									},
									DefaultMode: pointer.Int32Ptr(int32(420)),
								},
							},
						},
					},
					DNSPolicy:                     corev1.DNSClusterFirst,
					DeprecatedServiceAccount:      contourRbacName,
					ServiceAccountName:            contourRbacName,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SchedulerName:                 "default-scheduler",
					SecurityContext:               objutil.NewUnprivilegedPodSecurity(),
					TerminationGracePeriodSeconds: pointer.Int64Ptr(int64(30)),
				},
			},
		},
	}

	return deploy
}

// currentDeployment returns the current Deployment resource for the provided contour.
func (r *reconciler) currentDeployment(ctx context.Context, contour *operatorv1alpha1.Contour) (*appsv1.Deployment, error) {
	deploy := &appsv1.Deployment{}
	key := types.NamespacedName{
		Namespace: contour.Spec.Namespace.Name,
		Name:      contourDeploymentName,
	}

	if err := r.client.Get(ctx, key, deploy); err != nil {
		return nil, err
	}

	return deploy, nil
}

// createDeployment creates a Deployment resource for the provided deploy.
func (r *reconciler) createDeployment(ctx context.Context, deploy *appsv1.Deployment) (*appsv1.Deployment, error) {
	if err := r.client.Create(ctx, deploy); err != nil {
		return nil, fmt.Errorf("failed to create deployment %s/%s: %w", deploy.Namespace, deploy.Name, err)
	}
	r.log.Info("created deployment", "namespace", deploy.Namespace, "name", deploy.Name)

	return deploy, nil
}

// updateDeploymentIfNeeded updates a Deployment if current does not match desired,
// using contour to verify the existence of owner labels.
func (r *reconciler) updateDeploymentIfNeeded(ctx context.Context, contour *operatorv1alpha1.Contour, current, desired *appsv1.Deployment) (*appsv1.Deployment, error) {
	if !ownerLabelsExist(current, contour) {
		r.log.Info("deployment missing owner labels; skipped updating", "namespace", current.Namespace,
			"name", current.Name)
		return current, nil
	}
	deploy, updated := equality.DeploymentConfigChanged(current, desired)
	if updated {
		if err := r.client.Update(ctx, deploy); err != nil {
			return nil, fmt.Errorf("failed to update deployment %s/%s: %w", deploy.Namespace, deploy.Name, err)
		}
		r.log.Info("updated deployment", "namespace", deploy.Namespace, "name", deploy.Name)
		return deploy, nil
	}
	r.log.Info("deployment unchanged; skipped updating",
		"namespace", current.Namespace, "name", current.Name)

	return current, nil
}

// makeDeploymentLabels returns labels for a Contour deployment, using
// "app.kubernetes.io/version: image" and contour fields for other label values.
func makeDeploymentLabels(contour *operatorv1alpha1.Contour, image string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":     "contour",
		"app.kubernetes.io/instance": contour.Name,
		// The image tag is used as the version.
		"app.kubernetes.io/version":    image,
		"app.kubernetes.io/component":  "ingress-controller",
		"app.kubernetes.io/managed-by": "contour-operator",
		// Associate the deployment with the provided contour.
		operatorv1alpha1.OwningContourNameLabel: contour.Name,
		operatorv1alpha1.OwningContourNsLabel:   contour.Namespace,
	}

}
