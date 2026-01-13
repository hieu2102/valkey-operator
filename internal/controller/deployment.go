/*
Copyright 2025 Valkey Contributors.

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

package controller

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	valkeyiov1alpha1 "valkey.io/valkey-operator/api/v1alpha1"
)

func createClusterDeployment(cluster *valkeyiov1alpha1.ValkeyCluster) *appsv1.Deployment {
	image := DefaultImage
	if cluster.Spec.Image != "" {
		image = cluster.Spec.Image
	}

	containers := []corev1.Container{
		{
			Name:  "valkey-server",
			Image: image,
			Command: []string{
				"valkey-server",
				"/config/valkey.conf",
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          "client",
					ContainerPort: DefaultPort,
				},
				{
					Name:          "cluster-bus",
					ContainerPort: DefaultClusterBusPort,
				},
			},
			LivenessProbe: &corev1.Probe{
				InitialDelaySeconds: 5,
				PeriodSeconds:       5,
				FailureThreshold:    5,
				TimeoutSeconds:      5,
				SuccessThreshold:    1,
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"/bin/bash",
							"-c",
							"/scripts/liveness-check.sh",
						},
					},
				},
			},
			ReadinessProbe: &corev1.Probe{
				InitialDelaySeconds: 5,
				PeriodSeconds:       5,
				FailureThreshold:    5,
				TimeoutSeconds:      2,
				SuccessThreshold:    1,
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"/bin/bash",
							"-c",
							"/scripts/readiness-check.sh",
						},
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "scripts",
					MountPath: "/scripts",
				},
				{
					Name:      "valkey-conf",
					MountPath: "/config",
					ReadOnly:  true,
				},
			},
		},
	}
	if cluster.Spec.Exporter.Enabled {
		exporterImage := DefaultExporterImage
		if cluster.Spec.Exporter.Image != "" {
			exporterImage = cluster.Spec.Exporter.Image
		}

		exporterContainer := corev1.Container{
			Name:  "metrics-exporter",
			Image: exporterImage,
			Args: []string{
				fmt.Sprintf("--redis.addr=localhost:%d", DefaultPort),
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          "metrics",
					ContainerPort: DefaultExporterPort,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			LivenessProbe: &corev1.Probe{
				InitialDelaySeconds: 10,
				PeriodSeconds:       10,
				TimeoutSeconds:      3,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/health",
						Port: intstr.FromInt(DefaultExporterPort),
					},
				},
			},
			ReadinessProbe: &corev1.Probe{
				InitialDelaySeconds: 5,
				PeriodSeconds:       10,
				TimeoutSeconds:      3,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/ready",
						Port: intstr.FromInt(DefaultExporterPort),
					},
				},
			},
			Resources: cluster.Spec.Exporter.Resources,
		}

		containers = append(containers, exporterContainer)

	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cluster.Name + "-",
			Namespace:    cluster.Namespace,
			Labels:       labels(cluster),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func(i int32) *int32 { return &i }(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels(cluster),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels(cluster),
				},
				Spec: corev1.PodSpec{
					Containers: containers,
					Volumes: []corev1.Volume{
						{
							Name: "scripts",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cluster.Name,
									},
									DefaultMode: func(i int32) *int32 { return &i }(0755),
								},
							},
						},
						{
							Name: "valkey-conf",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cluster.Name,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return deployment
}
