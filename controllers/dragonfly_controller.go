/*
Copyright 2023.

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

package controllers

import (
	"context"
	"fmt"
	"os"
	"path"
	// "strings"
	monitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/k8sutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	dragonflyv1alpha1 "github.com/tamcore/dragonfly-operator/api/v1alpha1"
)

const dragonflyFinalizer = "dragonfly.pborn.eu/finalizer"

// Definitions to manage status conditions
const (
	// typeAvailableDragonfly represents the status of the Deployment reconciliation
	typeAvailableDragonfly = "Available"
	// typeDegradedDragonfly represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	typeDegradedDragonfly = "Degraded"
)

// DragonflyReconciler reconciles a Dragonfly object
type DragonflyReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// The following markers are used to generate the rules permissions (RBAC) on config/rbac using controller-gen
// when the command <make manifests> is executed.
// To know more about markers see: https://book.kubebuilder.io/reference/markers.html

//+kubebuilder:rbac:groups=dragonfly.pborn.eu,resources=dragonflies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dragonfly.pborn.eu,resources=dragonflies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dragonfly.pborn.eu,resources=dragonflies/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulset,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

// It is essential for the controller's reconciliation loop to be idempotent. By following the Operator
// pattern you will create Controllers which provide a reconcile function
// responsible for synchronizing resources until the desired state is reached on the cluster.
// Breaking this recommendation goes against the design principles of controller-runtime.
// and may lead to unforeseen consequences such as resources becoming stuck and requiring manual intervention.
// For further info:
// - About Operator Pattern: https://kubernetes.io/docs/concepts/extend-kubernetes/operator/
// - About Controllers: https://kubernetes.io/docs/concepts/architecture/controller/
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.1/pkg/reconcile
func (r *DragonflyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Dragonfly instance
	// The purpose is check if the Custom Resource for the Kind Dragonfly
	// is applied on the cluster if not we return nil to stop the reconciliation
	dragonfly := &dragonflyv1alpha1.Dragonfly{}
	err := r.Get(ctx, req.NamespacedName, dragonfly)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("dragonfly resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get dragonfly")
		return ctrl.Result{}, err
	}

	// Let's just set the status as Unknown when no status are available
	if dragonfly.Status.Conditions == nil || len(dragonfly.Status.Conditions) == 0 {
		meta.SetStatusCondition(&dragonfly.Status.Conditions, metav1.Condition{Type: typeAvailableDragonfly, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err = r.Status().Update(ctx, dragonfly); err != nil {
			log.Error(err, "Failed to update Dragonfly status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the dragonfly Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, dragonfly); err != nil {
			log.Error(err, "Failed to re-fetch dragonfly")
			return ctrl.Result{}, err
		}
	}

	// Let's add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(dragonfly, dragonflyFinalizer) {
		log.Info("Adding Finalizer for Dragonfly")
		if ok := controllerutil.AddFinalizer(dragonfly, dragonflyFinalizer); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

		if err = r.Update(ctx, dragonfly); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Check if the Dragonfly instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isDragonflyMarkedToBeDeleted := dragonfly.GetDeletionTimestamp() != nil
	if isDragonflyMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(dragonfly, dragonflyFinalizer) {
			log.Info("Performing Finalizer Operations for Dragonfly before delete CR")

			// Let's add here an status "Downgrade" to define that this resource begin its process to be terminated.
			meta.SetStatusCondition(&dragonfly.Status.Conditions, metav1.Condition{Type: typeDegradedDragonfly,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", dragonfly.Name)})

			if err := r.Status().Update(ctx, dragonfly); err != nil {
				log.Error(err, "Failed to update Dragonfly status")
				return ctrl.Result{}, err
			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			r.doFinalizerOperationsForDragonfly(dragonfly)

			// TODO(user): If you add operations to the doFinalizerOperationsForDragonfly method
			// then you need to ensure that all worked fine before deleting and updating the Downgrade status
			// otherwise, you should requeue here.

			// Re-fetch the dragonfly Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			if err := r.Get(ctx, req.NamespacedName, dragonfly); err != nil {
				log.Error(err, "Failed to re-fetch dragonfly")
				return ctrl.Result{}, err
			}

			meta.SetStatusCondition(&dragonfly.Status.Conditions, metav1.Condition{Type: typeDegradedDragonfly,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", dragonfly.Name)})

			if err := r.Status().Update(ctx, dragonfly); err != nil {
				log.Error(err, "Failed to update Dragonfly status")
				return ctrl.Result{}, err
			}

			log.Info("Removing Finalizer for Dragonfly after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(dragonfly, dragonflyFinalizer); !ok {
				log.Error(err, "Failed to remove finalizer for Dragonfly")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, dragonfly); err != nil {
				log.Error(err, "Failed to remove finalizer for Dragonfly")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if dragonfly.Spec.RedisPort == "" {
		dragonfly.Spec.RedisPort = "6379"
	}

	if true {
		foundService := &v1.Service{}
		service, serviceErr := r.serviceForDragonfly(dragonfly)
		err = r.Get(ctx, types.NamespacedName{Name: dragonfly.Name, Namespace: dragonfly.Namespace}, foundService)
		if err != nil && apierrors.IsNotFound(err) {
			if serviceErr != nil {
				log.Error(err, "Failed to define new Service resource for Dragonfly")

				// The following implementation will update the status
				meta.SetStatusCondition(&dragonfly.Status.Conditions, metav1.Condition{Type: typeAvailableDragonfly,
					Status: metav1.ConditionFalse, Reason: "Reconciling",
					Message: fmt.Sprintf("Failed to create Service for the custom resource (%s): (%s)", dragonfly.Name, err)})

				if err := r.Status().Update(ctx, dragonfly); err != nil {
					log.Error(err, "Failed to update Dragonfly status")
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, err
			}

			log.Info("Creating a new Service",
				"Service.Namespace", service.Namespace, "Service.Name", service.Name)
			if err = r.Create(ctx, service); err != nil {
				log.Error(err, "Failed to create new Service",
					"Service.Namespace", service.Namespace, "Service.Name", service.Name)
				return ctrl.Result{}, err
			}

			// Service created successfully
			// We will requeue the reconciliation so that we can ensure the state
			// and move forward for the next operations
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		} else if err != nil {
			log.Error(err, "Failed to get Service")
			// Let's return the error for the reconciliation be re-trigged again
			return ctrl.Result{}, err
		}

		if !reflect.DeepEqual(service.Spec, foundService.Spec) {
			foundService.Spec = service.Spec
			log.Info("Reconciling Service %s/%s\n", service.Namespace, service.Name)

			err = r.Update(context.TODO(), foundService)
			if err != nil {
				log.Error(err, "Failed to recincile deployment")
				return ctrl.Result{}, err
			}
		}

	}

	if dragonfly.Spec.PodMonitor {
		foundPodMonitor := &monitoring.PodMonitor{TypeMeta: metav1.TypeMeta{
			Kind:       "PodMonitor",
			APIVersion: "monitoring.coreos.com/v1",
		}}
		podMonitor, podMonitorErr := r.podMonitorForDragonfly(dragonfly)
		err = r.Get(ctx, types.NamespacedName{Name: dragonfly.Name, Namespace: dragonfly.Namespace}, foundPodMonitor)
		if err != nil && apierrors.IsNotFound(err) {
			if podMonitorErr != nil {
				log.Error(err, "Failed to define new PodMonitor resource for Dragonfly")

				// The following implementation will update the status
				meta.SetStatusCondition(&dragonfly.Status.Conditions, metav1.Condition{Type: typeAvailableDragonfly,
					Status: metav1.ConditionFalse, Reason: "Reconciling",
					Message: fmt.Sprintf("Failed to create PodMonitor for the custom resource (%s): (%s)", dragonfly.Name, err)})

				if err := r.Status().Update(ctx, dragonfly); err != nil {
					log.Error(err, "Failed to update Dragonfly status")
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, err
			}

			log.Info("Creating a new PodMonitor",
				"PodMonitor.Namespace", dragonfly.Namespace, "PodMonitor.Name", dragonfly.Name)
			if err = r.Create(ctx, podMonitor); err != nil {
				log.Error(err, "Failed to create new PodMonitor",
					"PodMonitor.Namespace", dragonfly.Namespace, "PodMonitor.Name", dragonfly.Name)
				return ctrl.Result{}, err
			}

			// PodMonitor created successfully
			// We will requeue the reconciliation so that we can ensure the state
			// and move forward for the next operations
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		} else if err != nil {
			log.Error(err, "Failed to get PodMonitor")
			// Let's return the error for the reconciliation be re-trigged again
			return ctrl.Result{}, err
		}

		if !reflect.DeepEqual(podMonitor.Spec, foundPodMonitor.Spec) {
			foundPodMonitor.Spec = podMonitor.Spec
			log.Info("Reconciling PodMonitor %s/%s\n", dragonfly.Namespace, dragonfly.Name)

			err = r.Update(context.TODO(), foundPodMonitor)
			if err != nil {
				log.Error(err, "Failed to recincile deployment")
				return ctrl.Result{}, err
			}
		}
	}

	if dragonfly.Spec.StatefulMode {
		found := &appsv1.StatefulSet{}
		deploy, deployErr := r.statefulsetForDragonfly(dragonfly)
		err = r.Get(ctx, types.NamespacedName{Name: dragonfly.Name, Namespace: dragonfly.Namespace}, found)
		if err != nil && apierrors.IsNotFound(err) {
			if deployErr != nil {
				log.Error(err, "Failed to define new StatefulSet resource for Dragonfly")

				// The following implementation will update the status
				meta.SetStatusCondition(&dragonfly.Status.Conditions, metav1.Condition{Type: typeAvailableDragonfly,
					Status: metav1.ConditionFalse, Reason: "Reconciling",
					Message: fmt.Sprintf("Failed to create StatefulSet for the custom resource (%s): (%s)", dragonfly.Name, err)})

				if err := r.Status().Update(ctx, dragonfly); err != nil {
					log.Error(err, "Failed to update Dragonfly status")
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, err
			}

			log.Info("Creating a new StatefulSet",
				"StatefulSet.Namespace", deploy.Namespace, "StatefulSet.Name", deploy.Name)
			if err = r.Create(ctx, deploy); err != nil {
				log.Error(err, "Failed to create new StatefulSet",
					"StatefulSet.Namespace", deploy.Namespace, "StatefulSet.Name", deploy.Name)
				return ctrl.Result{}, err
			}

			// StatefulSet created successfully
			// We will requeue the reconciliation so that we can ensure the state
			// and move forward for the next operations
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		} else if err != nil {
			log.Error(err, "Failed to get StatefulSet")
			// Let's return the error for the reconciliation be re-trigged again
			return ctrl.Result{}, err
		}

		if !reflect.DeepEqual(deploy.Spec, found.Spec) {
			found.Spec = deploy.Spec
			log.Info("Reconciling StatefulSet %s/%s\n", deploy.Namespace, deploy.Name)

			err = r.Update(context.TODO(), found)
			if err != nil {
				log.Error(err, "Failed to recincile deployment")
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{Requeue: true}, nil
	}

	found := &appsv1.Deployment{}
	deploy, deployErr := r.deploymentForDragonfly(dragonfly)
	err = r.Get(ctx, types.NamespacedName{Name: dragonfly.Name, Namespace: dragonfly.Namespace}, found)
	if err != nil && apierrors.IsNotFound(err) {
		if deployErr != nil {
			log.Error(err, "Failed to define new Deployment resource for Dragonfly")

			// The following implementation will update the status
			meta.SetStatusCondition(&dragonfly.Status.Conditions, metav1.Condition{Type: typeAvailableDragonfly,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Deployment for the custom resource (%s): (%s)", dragonfly.Name, err)})

			if err := r.Status().Update(ctx, dragonfly); err != nil {
				log.Error(err, "Failed to update Dragonfly status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("Creating a new Deployment",
			"Deployment.Namespace", deploy.Namespace, "Deployment.Name", deploy.Name)
		if err = r.Create(ctx, deploy); err != nil {
			log.Error(err, "Failed to create new Deployment",
				"Deployment.Namespace", deploy.Namespace, "Deployment.Name", deploy.Name)
			return ctrl.Result{}, err
		}

		// Deployment created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	if !reflect.DeepEqual(deploy.Spec, found.Spec) {
		found.Spec = deploy.Spec
		log.Info("Reconciling Deployment %s/%s\n", deploy.Namespace, deploy.Name)

		err = r.Update(context.TODO(), found)
		if err != nil {
			log.Error(err, "Failed to recincile deployment")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{Requeue: true}, nil
}

// finalizeDragonfly will perform the required operations before delete the CR.
func (r *DragonflyReconciler) doFinalizerOperationsForDragonfly(cr *dragonflyv1alpha1.Dragonfly) {
	// TODO(user): Add the cleanup steps that the operator
	// needs to do before the CR can be deleted. Examples
	// of finalizers include performing backups and deleting
	// resources that are not owned by this CR, like a PVC.

	// Note: It is not recommended to use finalizers with the purpose of delete resources which are
	// created and managed in the reconciliation. These ones, such as the Deployment created on this reconcile,
	// are defined as depended of the custom resource. See that we use the method ctrl.SetControllerReference.
	// to set the ownerRef which means that the Deployment will be deleted by the Kubernetes API.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/

	// The following implementation will raise an event
	r.Recorder.Event(cr, "Warning", "Deleting",
		fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
			cr.Name,
			cr.Namespace))
}

func (r *DragonflyReconciler) dragonflyPodSpec(dragonfly *dragonflyv1alpha1.Dragonfly) *corev1.PodSpec {

	// Get the Operand image
	image, _ := imageForDragonfly(dragonfly.Spec.Image.Repository, dragonfly.Spec.Image.Tag)

	var ports []corev1.ContainerPort
	ports = append(ports, corev1.ContainerPort{
		Name:          "dragonfly",
		Protocol:      "TCP",
		ContainerPort: intstr.Parse(dragonfly.Spec.RedisPort).IntVal,
	})

	args := []string{
		"--logtostdout",
	}

	if len(dragonfly.Spec.ExtraArgs) > 0 {
		args = append(args, dragonfly.Spec.ExtraArgs...)
	}
	if dragonfly.Spec.RedisPort != "" {
		args = append(args, fmt.Sprintf("--port=%s", dragonfly.Spec.RedisPort))
	}
	if dragonfly.Spec.MemcachePort != "" {
		args = append(args, fmt.Sprintf("--memcache_port=%s", dragonfly.Spec.MemcachePort))
		ports = append(ports, corev1.ContainerPort{
			Name:          "memcache",
			Protocol:      "TCP",
			ContainerPort: intstr.Parse(dragonfly.Spec.MemcachePort).IntVal,
		})
	}

	var envs []corev1.EnvVar
	envs = append(envs, dragonfly.Spec.ExtraEnvs...)

	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount
	volumeMounts = append(volumeMounts, dragonfly.Spec.VolumeMounts...)
	volumeMounts = append(volumeMounts,
		corev1.VolumeMount{
			Name:      "data",
			MountPath: "/data",
		},
	)

	volumes = append(volumes, dragonfly.Spec.Volumes...)

	if !dragonfly.Spec.StatefulMode {
		volumes = append(volumes, corev1.Volume{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	for _, s := range dragonfly.Spec.Secrets {
		volumes = append(volumes, corev1.Volume{
			Name: fmt.Sprintf("secret-%s", s),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: s,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      fmt.Sprintf("secret-%s", s),
			ReadOnly:  true,
			MountPath: path.Join("/etc/dragonfly/secret/", s),
		})
	}

	for _, c := range dragonfly.Spec.ConfigMaps {
		volumes = append(volumes, corev1.Volume{
			Name: fmt.Sprintf("configmap-%s", c),
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: c,
					},
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      fmt.Sprintf("configmap-%s", c),
			ReadOnly:  true,
			MountPath: path.Join("/etc/dragonfly/configs/", c),
		})
	}

	dragonflyContainer := []corev1.Container{{
		Name:            "dragonfly",
		Command:         dragonfly.Spec.CommandOverride,
		Image:           image,
		Args:            args,
		Env:             envs,
		ImagePullPolicy: dragonfly.Spec.Image.PullPolicy,
		Ports:           ports,
		Resources:       dragonfly.Spec.Resources,
		SecurityContext: dragonfly.Spec.SecurityContext,
		VolumeMounts:    volumeMounts,
	}}

	containers, _ := k8sutil.MergePatchContainers(dragonflyContainer, dragonfly.Spec.Containers)

	podSpec := &corev1.PodSpec{
		Containers:         containers,
		NodeSelector:       dragonfly.Spec.NodeSelector,
		Affinity:           dragonfly.Spec.Affinity,
		HostNetwork:        dragonfly.Spec.HostNetwork,
		ImagePullSecrets:   dragonfly.Spec.ImagePullSecrets,
		InitContainers:     dragonfly.Spec.InitContainers,
		SecurityContext:    dragonfly.Spec.PodSecurityContext,
		ServiceAccountName: dragonfly.Spec.ServiceAccountName,
		Tolerations:        dragonfly.Spec.Tolerations,
		Volumes:            volumes,
	}

	return podSpec
}

// deploymentForDragonfly returns a Dragonfly Deployment object
func (r *DragonflyReconciler) deploymentForDragonfly(dragonfly *dragonflyv1alpha1.Dragonfly) (*appsv1.Deployment, error) {
	ls := labelsForDragonfly(dragonfly.Name)

	podSpec := r.dragonflyPodSpec(dragonfly)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dragonfly.Name,
			Namespace: dragonfly.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &dragonfly.Spec.ReplicaCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: *podSpec,
			},
		},
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(dragonfly, dep, r.Scheme); err != nil {
		return nil, err
	}
	return dep, nil
}

func (r *DragonflyReconciler) statefulsetForDragonfly(dragonfly *dragonflyv1alpha1.Dragonfly) (*appsv1.StatefulSet, error) {
	ls := labelsForDragonfly(dragonfly.Name)

	podSpec := r.dragonflyPodSpec(dragonfly)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dragonfly.Name,
			Namespace: dragonfly.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &dragonfly.Spec.ReplicaCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: *podSpec,
			},
		},
	}

	// take care of the STS volume claim
	stsClaim := v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "data",
		},
		Spec: dragonfly.Spec.StatefulStorage,
	}
	if stsClaim.Spec.AccessModes == nil {
		stsClaim.Spec.AccessModes = []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}
	}
	sts.Spec.VolumeClaimTemplates = append(sts.Spec.VolumeClaimTemplates, stsClaim)

	if err := ctrl.SetControllerReference(dragonfly, sts, r.Scheme); err != nil {
		return nil, err
	}
	return sts, nil
}

func (r *DragonflyReconciler) podMonitorForDragonfly(dragonfly *dragonflyv1alpha1.Dragonfly) (*monitoring.PodMonitor, error) {
	ls := labelsForDragonfly(dragonfly.Name)

	svc := &monitoring.PodMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodMonitor",
			APIVersion: "monitoring.coreos.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dragonfly.Name,
			Namespace: dragonfly.Namespace,
		},
		Spec: monitoring.PodMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: ls,
			},
			PodMetricsEndpoints: []monitoring.PodMetricsEndpoint{
				{
					Path:   "/metrics",
					Scheme: "http",
					Port:   "dragonfly",
				},
			},
		},
	}
	// TODO: Add HTTPS and eventually TLS validation

	if err := ctrl.SetControllerReference(dragonfly, svc, r.Scheme); err != nil {
		return nil, err
	}
	return svc, nil
}

func (r *DragonflyReconciler) serviceForDragonfly(dragonfly *dragonflyv1alpha1.Dragonfly) (*v1.Service, error) {
	ls := labelsForDragonfly(dragonfly.Name)

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dragonfly.Name,
			Namespace: dragonfly.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeClusterIP,
			Selector: ls,
			Ports: []v1.ServicePort{
				{
					Name:       "dragonfly",
					Protocol:   "TCP",
					Port:       intstr.Parse(dragonfly.Spec.RedisPort).IntVal,
					TargetPort: intstr.Parse(dragonfly.Spec.RedisPort),
				},
			},
		},
	}

	if dragonfly.Spec.MemcachePort != "" {
		svc.Spec.Ports = append(svc.Spec.Ports, v1.ServicePort{
			Name:       "memcache",
			Protocol:   "TCP",
			Port:       intstr.Parse(dragonfly.Spec.MemcachePort).IntVal,
			TargetPort: intstr.Parse(dragonfly.Spec.MemcachePort),
		})
	}

	if err := ctrl.SetControllerReference(dragonfly, svc, r.Scheme); err != nil {
		return nil, err
	}
	return svc, nil
}

// labelsForDragonfly returns the labels for selecting the resources
// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
func labelsForDragonfly(name string) map[string]string {
	// var imageTag string
	// image, err := imageForDragonfly()
	// if err == nil {
	// 	imageTag = strings.Split(image, ":")[1]
	// }
	return map[string]string{"app.kubernetes.io/name": "Dragonfly",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/part-of":    "dragonfly-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

func getVarEnvFallback(string, envVar string) string {
	if string == "" {
		if value, ok := os.LookupEnv(envVar); ok {
			return value
		}
	}
	return string
}

// imageForDragonfly gets the Operand image which is managed by this controller
// from the DRAGONFLY_IMAGE environment variable defined in the config/manager/manager.yaml
func imageForDragonfly(imageRepository string, imageTag string) (string, error) {
	image := fmt.Sprintf("%s:%s",
		getVarEnvFallback(imageRepository, "DRAGONFLY_IMAGE_REPOSITORY"),
		getVarEnvFallback(imageTag, "DRAGONFLY_IMAGE_TAG"),
	)

	return image, nil
}

// SetupWithManager sets up the controller with the Manager.
// Note that the Deployment will be also watched in order to ensure its
// desirable state on the cluster
func (r *DragonflyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dragonflyv1alpha1.Dragonfly{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
