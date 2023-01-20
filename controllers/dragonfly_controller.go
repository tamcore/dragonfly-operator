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
	"path"
	// "strings"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus-operator/prometheus-operator/pkg/k8sutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	// v1 "k8s.io/api/core/v1"
	"github.com/caitlinelfring/go-env-default"
	"github.com/tcnksm/go-latest"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dragonflyv1alpha1 "github.com/tamcore/dragonfly-operator/api/v1alpha1"
)

const dragonflyFinalizer = "dragonfly.pborn.eu/finalizer"

// Definitions to manage status conditions
const (
	// typeAvailableDragonfly represents the status of the reconciliation
	typeAvailableDragonfly = "Available"
	// typeDegradedDragonfly represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	typeDegradedDragonfly = "Degraded"

	dragonflySecretDir    = "/etc/dragonfly/secret"
	dragonflyConfigMapDir = "/etc/dragonfly/config"

	dragonflyTlsDir = "/etc/dragonfly/tls"

	dragonflyVolumeClaimDefaultSize = "1Gi"
)

var (
	dragonflyDefaultRepository = env.GetDefault("DRAGONFLY_IMAGE_REPOSITORY", "ghcr.io/dragonflydb/dragonfly")
	dragonflyDefaultTag        = env.GetDefault("DRAGONFLY_IMAGE_TAG", latestGithubRelease("dragonflydb", "dragonfly"))
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
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=podmonitors,verbs=get;list;watch;create;update;patch;delete

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
		log.Info(fmt.Sprintf("Adding Finalizer for Dragonfly: %s in %s", dragonfly.Name, dragonfly.Namespace))
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

			log.Info("Removing Finalizer for Dragonfly (", dragonfly.Name, " in ", dragonfly.Namespace, ") after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(dragonfly, dragonflyFinalizer); !ok {
				log.Error(err, "Failed to remove finalizer for Dragonfly (", dragonfly.Name, " in ", dragonfly.Namespace, ")")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, dragonfly); err != nil {
				log.Error(err, "Failed to remove finalizer for Dragonfly (", dragonfly.Name, " in ", dragonfly.Namespace, ")")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if dragonfly.Spec.RedisPort == "" {
		dragonfly.Spec.RedisPort = "6379"
	}

	serviceSpec, _ := r.serviceForDragonfly(dragonfly)
	err = r.ReconcileObject(ctx, dragonfly, serviceSpec)
	if err != nil {
		log.Error(err, "unable to ensure Service")
		return ctrl.Result{Requeue: true}, err
	} else {
		log.Info("Ensured Service...")
	}

	if dragonfly.Spec.PodMonitor {
		podMonitorSpec, _ := r.podMonitorForDragonfly(dragonfly)
		err = r.ReconcileObject(ctx, dragonfly, podMonitorSpec)
		if err != nil {
			log.Error(err, "unable to ensure PodMonitor")
			return ctrl.Result{Requeue: true}, err
		} else {
			log.Info("Ensured PodMonitor...")
		}
	}

	if dragonfly.Spec.StatefulMode {
		statefulSetSpec, _ := r.statefulsetForDragonfly(dragonfly)
		err = r.ReconcileObject(ctx, dragonfly, statefulSetSpec)
		if err != nil {
			log.Error(err, "unable to ensure StatefulSet")
			return ctrl.Result{Requeue: true}, err
		} else {
			log.Info("Ensured StatefulSet...")
		}
	}

	if !dragonfly.Spec.StatefulMode {
		deploymentSpec, _ := r.deploymentForDragonfly(dragonfly)
		err = r.ReconcileObject(ctx, dragonfly, deploymentSpec)
		if err != nil {
			log.Error(err, "unable to ensure Deployment")
			return ctrl.Result{Requeue: true}, err
		} else {
			log.Info("Ensured Deployment...")
		}
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *DragonflyReconciler) ReconcileObject(ctx context.Context, dragonfly *dragonflyv1alpha1.Dragonfly, obj runtime.Object) error {
	// Convert the object to a Kubernetes object
	k8sObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}

	// Create a new Unstructured object
	desiredObj := &unstructured.Unstructured{}
	desiredObj.Object = k8sObj

	// Extract the GroupVersionKind from the input object
	gvks, _, err := r.Scheme.ObjectKinds(obj)
	if err != nil {
		return err
	}
	desiredObj.SetGroupVersionKind(gvks[0])

	// Quickly construct an object to search for with CreateOrUpdate and mutate it's result later
	inclusterObj := &unstructured.Unstructured{}
	inclusterObj.SetName(desiredObj.GetName())
	inclusterObj.SetNamespace(desiredObj.GetNamespace())
	inclusterObj.SetGroupVersionKind(gvks[0])

	//Create or Update the object in the cluster
	_, err = ctrl.CreateOrUpdate(ctx, r.Client, inclusterObj, func() error {
		// Set the spec field on the new object
		inclusterObj.Object["spec"] = desiredObj.Object["spec"]

		// Set the owner reference on the object
		return ctrl.SetControllerReference(dragonfly, inclusterObj, r.Scheme)
	})
	return err
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
	image, _ := imageForDragonfly(dragonfly.Spec.Image.Repository, dragonfly.Spec.Image.Tag)

	var ports []corev1.ContainerPort
	ports = append(ports, corev1.ContainerPort{
		Name:          "dragonfly",
		Protocol:      "TCP",
		ContainerPort: intstr.Parse(dragonfly.Spec.RedisPort).IntVal,
	})

	args := []string{
		"--logtostderr",
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

	if dragonfly.Spec.TlsConfig.ExistingSecret.Size() > 0 {
		volumes = append(volumes, corev1.Volume{
			Name: "tls",
			VolumeSource: corev1.VolumeSource{
				Secret: dragonfly.Spec.TlsConfig.ExistingSecret,
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "tls",
			ReadOnly:  true,
			MountPath: dragonflyTlsDir,
		})
		tlsArgs := []string{
			"--tls",
			fmt.Sprintf("--tls_cert_file=%s/tls.crt", dragonflyTlsDir),
			fmt.Sprintf("--tls_key_file=%s/tls.key", dragonflyTlsDir),
		}
		args = append(args, tlsArgs...)
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
			MountPath: path.Join(dragonflySecretDir, s),
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
			MountPath: path.Join(dragonflyConfigMapDir, c),
		})
	}

	probehandler := corev1.ProbeHandler{
		TCPSocket: &corev1.TCPSocketAction{
			Port: intstr.FromString("dragonfly"),
		},
	}

	readinessProbe := &corev1.Probe{
		TimeoutSeconds:      5,
		InitialDelaySeconds: 5,
		ProbeHandler:        probehandler,
	}

	livenessProbe := &corev1.Probe{
		InitialDelaySeconds: 30,
		TimeoutSeconds:      1,
		FailureThreshold:    6,
		ProbeHandler:        probehandler,
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
		ReadinessProbe:  readinessProbe,
		LivenessProbe:   livenessProbe,
	}}

	containers, _ := k8sutil.MergePatchContainers(dragonflyContainer, dragonfly.Spec.Containers)

	podSpec := &corev1.PodSpec{
		Affinity:                     dragonfly.Spec.Affinity,
		AutomountServiceAccountToken: pointer.Bool(false),
		Containers:                   containers,
		HostNetwork:                  dragonfly.Spec.HostNetwork,
		ImagePullSecrets:             dragonfly.Spec.ImagePullSecrets,
		InitContainers:               dragonfly.Spec.InitContainers,
		NodeSelector:                 dragonfly.Spec.NodeSelector,
		SecurityContext:              dragonfly.Spec.PodSecurityContext,
		ServiceAccountName:           dragonfly.Spec.ServiceAccountName,
		Tolerations:                  dragonfly.Spec.Tolerations,
		Volumes:                      volumes,
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
	stsClaim := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: "data",
		},
		Spec: dragonfly.Spec.StatefulStorage,
	}
	if stsClaim.Spec.AccessModes == nil {
		stsClaim.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}
	if stsClaim.Spec.Resources.Requests.Storage().IsZero() {
		defaultSize := resource.MustParse(dragonflyVolumeClaimDefaultSize)
		stsClaim.Spec.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: defaultSize,
			},
		}
	}
	sts.Spec.VolumeClaimTemplates = append(sts.Spec.VolumeClaimTemplates, stsClaim)

	if err := ctrl.SetControllerReference(dragonfly, sts, r.Scheme); err != nil {
		return nil, err
	}
	return sts, nil
}

func (r *DragonflyReconciler) podMonitorForDragonfly(dragonfly *dragonflyv1alpha1.Dragonfly) (*monitoringv1.PodMonitor, error) {
	ls := labelsForDragonfly(dragonfly.Name)

	PodMetricsEndpointScheme := "http"
	PodMetricsEndpointTLSConfig := monitoringv1.PodMetricsEndpointTLSConfig{}
	if dragonfly.Spec.TlsConfig.ExistingSecret != nil {
		PodMetricsEndpointScheme = "https"
		PodMetricsEndpointTLSConfig.SafeTLSConfig = monitoringv1.SafeTLSConfig{
			InsecureSkipVerify: true,
		}
	}

	svc := &monitoringv1.PodMonitor{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodMonitor",
			APIVersion: "monitoringv1.coreos.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dragonfly.Name,
			Namespace: dragonfly.Namespace,
		},
		Spec: monitoringv1.PodMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: ls,
			},
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
				{
					Path:      "/metrics",
					Scheme:    PodMetricsEndpointScheme,
					Port:      "dragonfly",
					TLSConfig: &PodMetricsEndpointTLSConfig,
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(dragonfly, svc, r.Scheme); err != nil {
		return nil, err
	}
	return svc, nil
}

func (r *DragonflyReconciler) serviceForDragonfly(dragonfly *dragonflyv1alpha1.Dragonfly) (*corev1.Service, error) {
	ls := labelsForDragonfly(dragonfly.Name)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dragonfly.Name,
			Namespace: dragonfly.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: ls,
			Ports: []corev1.ServicePort{
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
		svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
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

func getStringDefault(preferred string, fallback string) string {
	if preferred == "" {
		return fallback
	}
	return preferred
}

func imageForDragonfly(imageRepository string, imageTag string) (string, error) {
	image := fmt.Sprintf("%s:%s",
		getStringDefault(imageRepository, dragonflyDefaultRepository),
		getStringDefault(imageTag, dragonflyDefaultTag),
	)

	return image, nil
}

func latestGithubRelease(repoOwner string, repository string) string {
	githubTag := &latest.GithubTag{
		Owner:      repoOwner,
		Repository: repository,
	}
	res, _ := latest.Check(githubTag, "v0.0.0")
	return "v" + res.Current
}

// SetupWithManager sets up the controller with the Manager.
// Note that the Deployment will be also watched in order to ensure its
// desirable state on the cluster
func (r *DragonflyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dragonflyv1alpha1.Dragonfly{}).
		Owns(&appsv1.Deployment{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&monitoringv1.PodMonitor{}).
		Complete(r)
}
