/*
Copyright 2025.

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
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	moodlev1alpha1 "bsu.by/moodle-lms-operator/api/v1alpha1"
)

// MoodleTenantReconciler reconciles a MoodleTenant object
type MoodleTenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=moodle.bsu.by,resources=moodletenants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=moodle.bsu.by,resources=moodletenants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=moodle.bsu.by,resources=moodletenants/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete

const moodleTenantFinalizer = "moodle.bsu.by/finalizer"

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MoodleTenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the MoodleTenant instance
	moodleTenant := &moodlev1alpha1.MoodleTenant{}
	err := r.Get(ctx, req.NamespacedName, moodleTenant)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("MoodleTenant resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get MoodleTenant")
		return ctrl.Result{}, err
	}

	// Examine DeletionTimestamp to determine if object is under deletion
	if moodleTenant.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so register our finalizer
		if !containsString(moodleTenant.GetFinalizers(), moodleTenantFinalizer) {
			moodleTenant.SetFinalizers(append(moodleTenant.GetFinalizers(), moodleTenantFinalizer))
			if err := r.Update(ctx, moodleTenant); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if containsString(moodleTenant.GetFinalizers(), moodleTenantFinalizer) {
			// Our finalizer is present, so lets handle any external dependency
			if err := r.finalizeMoodleTenant(ctx, moodleTenant); err != nil {
				return ctrl.Result{}, err
			}

			// Remove our finalizer from the list and update it
			moodleTenant.SetFinalizers(removeString(moodleTenant.GetFinalizers(), moodleTenantFinalizer))
			if err := r.Update(ctx, moodleTenant); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	// Get the tenant namespace name
	tenantNamespace := fmt.Sprintf("tenant-%s", moodleTenant.Name)

	// Define a new Namespace object
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: tenantNamespace,
		},
	}

	// Check if this Namespace already exists
	foundNamespace := &corev1.Namespace{}
	err = r.Get(ctx, types.NamespacedName{Name: namespace.Name}, foundNamespace)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new Namespace", "Namespace.Name", namespace.Name)
		err = r.Create(ctx, namespace)
		if err != nil {
			logger.Error(err, "Failed to create new Namespace", "Namespace.Name", namespace.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get Namespace")
		return ctrl.Result{}, err
	}

	if err := r.reconcileSecret(ctx, moodleTenant, tenantNamespace); err != nil {
		return ctrl.Result{}, err
	}

	// Namespace exists, now reconcile all resources
	if err := r.reconcileDeployment(ctx, moodleTenant, tenantNamespace); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcilePVC(ctx, moodleTenant, tenantNamespace); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileService(ctx, moodleTenant, tenantNamespace); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileIngress(ctx, moodleTenant, tenantNamespace); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileNetworkPolicy(ctx, moodleTenant, tenantNamespace); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileHPA(ctx, moodleTenant, tenantNamespace); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileCronJob(ctx, moodleTenant, tenantNamespace); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcilePDB(ctx, moodleTenant, tenantNamespace); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Successfully reconciled MoodleTenant", "Name", moodleTenant.Name)

	return ctrl.Result{}, nil
}

// finalizeMoodleTenant handles cleanup before the MoodleTenant is deleted
func (r *MoodleTenantReconciler) finalizeMoodleTenant(ctx context.Context, mt *moodlev1alpha1.MoodleTenant) error {
	logger := log.FromContext(ctx)
	logger.Info("Finalizing MoodleTenant", "Name", mt.Name)

	// Delete the tenant namespace
	tenantNamespace := "tenant-" + mt.Name
	namespace := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: tenantNamespace}, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Namespace already deleted", "Namespace", tenantNamespace)
			return nil
		}
		return err
	}

	logger.Info("Deleting namespace", "Namespace", tenantNamespace)
	if err := r.Delete(ctx, namespace); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	logger.Info("Namespace deleted successfully", "Namespace", tenantNamespace)
	return nil
}

// reconcileDeployment creates or updates the Moodle Deployment
func (r *MoodleTenantReconciler) reconcileDeployment(ctx context.Context, mt *moodlev1alpha1.MoodleTenant, namespace string) error {
	logger := log.FromContext(ctx)

	deployment := r.deploymentForMoodle(mt, namespace)

	// Check if the Deployment already exists
	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		err = r.Create(ctx, deployment)
		if err != nil {
			logger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return err
		}
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to get Deployment")
		return err
	}

	// Deployment exists, could implement update logic here
	logger.Info("Deployment already exists", "Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)
	return nil
}

// reconcilePVC creates or updates the PersistentVolumeClaim
func (r *MoodleTenantReconciler) reconcilePVC(ctx context.Context, mt *moodlev1alpha1.MoodleTenant, namespace string) error {
	logger := log.FromContext(ctx)

	pvc := r.pvcForMoodle(mt, namespace)

	// Check if the PVC already exists
	found := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new PVC", "PVC.Namespace", pvc.Namespace, "PVC.Name", pvc.Name)
		err = r.Create(ctx, pvc)
		if err != nil {
			logger.Error(err, "Failed to create new PVC", "PVC.Namespace", pvc.Namespace, "PVC.Name", pvc.Name)
			return err
		}
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to get PVC")
		return err
	}

	logger.Info("PVC already exists", "PVC.Namespace", found.Namespace, "PVC.Name", found.Name)
	return nil
}

// reconcileService creates or updates the Service
func (r *MoodleTenantReconciler) reconcileService(ctx context.Context, mt *moodlev1alpha1.MoodleTenant, namespace string) error {
	logger := log.FromContext(ctx)

	service := r.serviceForMoodle(mt, namespace)

	// Check if the Service already exists
	found := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new Service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
		err = r.Create(ctx, service)
		if err != nil {
			logger.Error(err, "Failed to create new Service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
			return err
		}
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to get Service")
		return err
	}

	logger.Info("Service already exists", "Service.Namespace", found.Namespace, "Service.Name", found.Name)
	return nil
}

// reconcileIngress creates or updates the Ingress
func (r *MoodleTenantReconciler) reconcileIngress(ctx context.Context, mt *moodlev1alpha1.MoodleTenant, namespace string) error {
	logger := log.FromContext(ctx)

	ingress := r.ingressForMoodle(mt, namespace)

	// Check if the Ingress already exists
	found := &networkingv1.Ingress{}
	err := r.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new Ingress", "Ingress.Namespace", ingress.Namespace, "Ingress.Name", ingress.Name)
		err = r.Create(ctx, ingress)
		if err != nil {
			logger.Error(err, "Failed to create new Ingress", "Ingress.Namespace", ingress.Namespace, "Ingress.Name", ingress.Name)
			return err
		}
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to get Ingress")
		return err
	}

	logger.Info("Ingress already exists", "Ingress.Namespace", found.Namespace, "Ingress.Name", found.Name)
	return nil
}

// reconcileNetworkPolicy creates or updates the NetworkPolicy
func (r *MoodleTenantReconciler) reconcileNetworkPolicy(ctx context.Context, mt *moodlev1alpha1.MoodleTenant, namespace string) error {
	logger := log.FromContext(ctx)

	networkPolicy := r.networkPolicyForMoodle(mt, namespace)

	// Check if the NetworkPolicy already exists
	found := &networkingv1.NetworkPolicy{}
	err := r.Get(ctx, types.NamespacedName{Name: networkPolicy.Name, Namespace: networkPolicy.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new NetworkPolicy", "NetworkPolicy.Namespace", networkPolicy.Namespace, "NetworkPolicy.Name", networkPolicy.Name)
		err = r.Create(ctx, networkPolicy)
		if err != nil {
			logger.Error(err, "Failed to create new NetworkPolicy", "NetworkPolicy.Namespace", networkPolicy.Namespace, "NetworkPolicy.Name", networkPolicy.Name)
			return err
		}
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to get NetworkPolicy")
		return err
	}

	logger.Info("NetworkPolicy already exists", "NetworkPolicy.Namespace", found.Namespace, "NetworkPolicy.Name", found.Name)
	return nil
}

func (r *MoodleTenantReconciler) reconcileHPA(ctx context.Context, mt *moodlev1alpha1.MoodleTenant, namespace string) error {
	logger := log.FromContext(ctx)

	// Only create HPA if enabled
	if !mt.Spec.HPA.Enabled {
		logger.Info("HPA is disabled, skipping")
		return nil
	}

	hpa := r.hpaForMoodle(mt, namespace)

	foundHPA := &autoscalingv2.HorizontalPodAutoscaler{}
	err := r.Get(ctx, types.NamespacedName{Name: hpa.Name, Namespace: hpa.Namespace}, foundHPA)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new HPA", "HPA.Namespace", hpa.Namespace, "HPA.Name", hpa.Name)
		err = r.Create(ctx, hpa)
		if err != nil {
			logger.Error(err, "Failed to create new HPA", "HPA.Namespace", hpa.Namespace, "HPA.Name", hpa.Name)
			return err
		}
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to get HPA")
		return err
	}

	// HPA exists, update if needed
	logger.Info("HPA already exists", "HPA.Namespace", foundHPA.Namespace, "HPA.Name", foundHPA.Name)
	return nil
}

func (r *MoodleTenantReconciler) reconcileCronJob(ctx context.Context, mt *moodlev1alpha1.MoodleTenant, namespace string) error {
	logger := log.FromContext(ctx)

	cronJob := r.cronJobForMoodle(mt, namespace)

	foundCronJob := &batchv1.CronJob{}
	err := r.Get(ctx, types.NamespacedName{Name: cronJob.Name, Namespace: cronJob.Namespace}, foundCronJob)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new CronJob", "CronJob.Namespace", cronJob.Namespace, "CronJob.Name", cronJob.Name)
		err = r.Create(ctx, cronJob)
		if err != nil {
			logger.Error(err, "Failed to create new CronJob", "CronJob.Namespace", cronJob.Namespace, "CronJob.Name", cronJob.Name)
			return err
		}
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to get CronJob")
		return err
	}

	// CronJob exists, update if needed
	logger.Info("CronJob already exists", "CronJob.Namespace", foundCronJob.Namespace, "CronJob.Name", foundCronJob.Name)
	return nil
}

func (r *MoodleTenantReconciler) reconcilePDB(ctx context.Context, mt *moodlev1alpha1.MoodleTenant, namespace string) error {
	logger := log.FromContext(ctx)

	// Only create PDB if HPA is enabled (implies we have multiple replicas)
	if !mt.Spec.HPA.Enabled {
		logger.Info("HPA is disabled, skipping PDB creation")
		return nil
	}

	pdb := r.pdbForMoodle(mt, namespace)

	foundPDB := &policyv1.PodDisruptionBudget{}
	err := r.Get(ctx, types.NamespacedName{Name: pdb.Name, Namespace: pdb.Namespace}, foundPDB)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new PDB", "PDB.Namespace", pdb.Namespace, "PDB.Name", pdb.Name)
		err = r.Create(ctx, pdb)
		if err != nil {
			logger.Error(err, "Failed to create new PDB", "PDB.Namespace", pdb.Namespace, "PDB.Name", pdb.Name)
			return err
		}
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to get PDB")
		return err
	}

	// PDB exists, update if needed
	logger.Info("PDB already exists", "PDB.Namespace", foundPDB.Namespace, "PDB.Name", foundPDB.Name)
	return nil
}

// reconcileSecret creates or updates the database Secret
func (r *MoodleTenantReconciler) reconcileSecret(ctx context.Context, mt *moodlev1alpha1.MoodleTenant, namespace string) error {
	logger := log.FromContext(ctx)

	secret := r.secretForMoodle(mt, namespace)

	// Check if the Secret already exists
	found := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating a new Secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		err = r.Create(ctx, secret)
		if err != nil {
			logger.Error(err, "Failed to create new Secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
			return err
		}
		return nil
	} else if err != nil {
		logger.Error(err, "Failed to get Secret")
		return err
	}

	logger.Info("Secret already exists", "Secret.Namespace", found.Namespace, "Secret.Name", found.Name)
	return nil
}

// secretForMoodle returns a Secret object for the MoodleTenant
func (r *MoodleTenantReconciler) secretForMoodle(mt *moodlev1alpha1.MoodleTenant, namespace string) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mt.Spec.DatabaseRef.AdminSecret,
			Namespace: namespace,
		},
		StringData: map[string]string{
			"host":     mt.Spec.DatabaseRef.Host,
			"database": mt.Spec.DatabaseRef.Name,
			"username": mt.Spec.DatabaseRef.User,
			"password": mt.Spec.DatabaseRef.Password,
		},
	}

	// Set MoodleTenant instance as the owner
	if err := ctrl.SetControllerReference(mt, secret, r.Scheme); err != nil {
		return nil
	}

	return secret
}

// deploymentForMoodle returns a Deployment object for the MoodleTenant
func (r *MoodleTenantReconciler) deploymentForMoodle(mt *moodlev1alpha1.MoodleTenant, namespace string) *appsv1.Deployment {
	labels := map[string]string{
		"app":                  "moodle",
		"moodle.bsu.by/tenant": mt.Name,
	}

	replicas := int32(1)
	if mt.Spec.HPA.Enabled && mt.Spec.HPA.MinReplicas != nil {
		replicas = *mt.Spec.HPA.MinReplicas
	}

	// Default values for PHP settings
	maxExecTime := 60
	if mt.Spec.PHPSettings.MaxExecutionTime != 0 {
		maxExecTime = mt.Spec.PHPSettings.MaxExecutionTime
	}

	memoryLimit := "512M"
	if mt.Spec.PHPSettings.MemoryLimit != "" {
		memoryLimit = mt.Spec.PHPSettings.MemoryLimit
	}

	memcachedMemory := 128
	if mt.Spec.Memcached.MemoryMB != 0 {
		memcachedMemory = mt.Spec.Memcached.MemoryMB
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mt.Name + "-deployment",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "moodle-php",
							Image: mt.Spec.Image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "PHP_MAX_EXECUTION_TIME",
									Value: fmt.Sprintf("%d", maxExecTime),
								},
								{
									Name:  "PHP_MEMORY_LIMIT",
									Value: memoryLimit,
								},
								{
									Name:  "MOODLE_URL",
									Value: fmt.Sprintf("https://%s", mt.Spec.Hostname),
								},
								{
									Name: "DB_HOST",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: mt.Spec.DatabaseRef.AdminSecret,
											},
											Key: "host",
										},
									},
								},
								{
									Name: "DB_NAME",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: mt.Spec.DatabaseRef.AdminSecret,
											},
											Key: "database",
										},
									},
								},
								{
									Name: "DB_USER",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: mt.Spec.DatabaseRef.AdminSecret,
											},
											Key: "username",
										},
									},
								},
								{
									Name: "DB_PASS",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: mt.Spec.DatabaseRef.AdminSecret,
											},
											Key: "password",
										},
									},
								},
							},
							Resources: mt.Spec.Resources,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "moodle-data",
									MountPath: "/var/www/moodledata",
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(9000),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(9000),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
								TimeoutSeconds:      3,
								FailureThreshold:    3,
							},
						},
						{
							Name:  "memcached",
							Image: "memcached:alpine",
							Command: []string{
								"memcached",
								"-m", fmt.Sprintf("%d", memcachedMemory),
								"-I", "2m",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "memcached",
									ContainerPort: 11211,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", memcachedMemory)),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dMi", memcachedMemory)),
								},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To(int64(33)),
						FSGroup:      ptr.To(int64(33)),
					},
					Volumes: []corev1.Volume{
						{
							Name: "moodle-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: mt.Name + "-data",
								},
							},
						},
					},
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{
						{
							MaxSkew:           1,
							TopologyKey:       "kubernetes.io/hostname",
							WhenUnsatisfiable: corev1.ScheduleAnyway,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: labels,
							},
						},
						{
							MaxSkew:           1,
							TopologyKey:       "topology.kubernetes.io/zone",
							WhenUnsatisfiable: corev1.ScheduleAnyway,
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: labels,
							},
						},
					},
				},
			},
		},
	}

	// Set MoodleTenant instance as the owner
	if err := ctrl.SetControllerReference(mt, deployment, r.Scheme); err != nil {
		return nil
	}

	return deployment
}

// pvcForMoodle returns a PersistentVolumeClaim object for the MoodleTenant
func (r *MoodleTenantReconciler) pvcForMoodle(mt *moodlev1alpha1.MoodleTenant, namespace string) *corev1.PersistentVolumeClaim {
	storageClass := "csi-cephfs-sc"
	if mt.Spec.Storage.StorageClass != "" {
		storageClass = mt.Spec.Storage.StorageClass
	}

	// Determine access mode based on storage class
	// CephFS and NFS support ReadWriteMany, local-path only supports ReadWriteOnce
	accessMode := corev1.ReadWriteMany
	if storageClass == "local-path" || storageClass == "hostpath" {
		accessMode = corev1.ReadWriteOnce
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mt.Name + "-data",
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				accessMode,
			},
			StorageClassName: &storageClass,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: mt.Spec.Storage.Size,
				},
			},
		},
	}

	// Set MoodleTenant instance as the owner
	if err := ctrl.SetControllerReference(mt, pvc, r.Scheme); err != nil {
		return nil
	}

	return pvc
}

// serviceForMoodle returns a Service object for the MoodleTenant
func (r *MoodleTenantReconciler) serviceForMoodle(mt *moodlev1alpha1.MoodleTenant, namespace string) *corev1.Service {
	labels := map[string]string{
		"app":                  "moodle",
		"moodle.bsu.by/tenant": mt.Name,
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mt.Name + "-service",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Type:     corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}

	// Set MoodleTenant instance as the owner
	if err := ctrl.SetControllerReference(mt, service, r.Scheme); err != nil {
		return nil
	}

	return service
}

// ingressForMoodle returns an Ingress object for the MoodleTenant
func (r *MoodleTenantReconciler) ingressForMoodle(mt *moodlev1alpha1.MoodleTenant, namespace string) *networkingv1.Ingress {
	labels := map[string]string{
		"app":                  "moodle",
		"moodle.bsu.by/tenant": mt.Name,
	}

	pathType := networkingv1.PathTypePrefix

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        mt.Name + "-ingress",
			Namespace:   namespace,
			Labels:      labels,
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: ptr.To("nginx"),
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{mt.Spec.Hostname},
					SecretName: fmt.Sprintf("%s-tls", mt.Name),
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: mt.Spec.Hostname,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: mt.Name + "-service",
											Port: networkingv1.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Set MoodleTenant instance as the owner
	if err := ctrl.SetControllerReference(mt, ingress, r.Scheme); err != nil {
		return nil
	}

	return ingress
}

// networkPolicyForMoodle returns a NetworkPolicy object for the MoodleTenant
// Implements Default Deny with explicit allow rules as per TECH_SPEC.md
func (r *MoodleTenantReconciler) networkPolicyForMoodle(mt *moodlev1alpha1.MoodleTenant, namespace string) *networkingv1.NetworkPolicy {
	labels := map[string]string{
		"app":                  "moodle",
		"moodle.bsu.by/tenant": mt.Name,
	}

	protocolTCP := corev1.ProtocolTCP
	protocolUDP := corev1.ProtocolUDP

	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-isolation",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					// Allow ingress from Ingress Controller
					From: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "ingress-nginx",
								},
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					// Allow egress to PostgreSQL database
					To: []networkingv1.NetworkPolicyPeer{
						{
							// This would need to be configured based on actual DB location
							// For now, allowing egress to kube-system for simplicity
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"moodle.bsu.by/db": "true",
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &protocolTCP,
							Port:     ptr.To(intstr.FromInt(5432)),
						},
					},
				},
				{
					// Allow DNS queries
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "kube-system",
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &protocolUDP,
							Port:     ptr.To(intstr.FromInt(53)),
						},
						{
							Protocol: &protocolTCP,
							Port:     ptr.To(intstr.FromInt(53)),
						},
					},
				},
				{
					// Allow HTTP/HTTPS egress for Moodle updates and external integrations
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &protocolTCP,
							Port:     ptr.To(intstr.FromInt(80)),
						},
						{
							Protocol: &protocolTCP,
							Port:     ptr.To(intstr.FromInt(443)),
						},
					},
				},
			},
		},
	}

	// Set MoodleTenant instance as the owner
	if err := ctrl.SetControllerReference(mt, networkPolicy, r.Scheme); err != nil {
		return nil
	}

	return networkPolicy
}

func (r *MoodleTenantReconciler) hpaForMoodle(mt *moodlev1alpha1.MoodleTenant, namespace string) *autoscalingv2.HorizontalPodAutoscaler {
	// Use default if not specified
	minReplicas := int32(2)
	if mt.Spec.HPA.MinReplicas != nil {
		minReplicas = *mt.Spec.HPA.MinReplicas
	}

	targetCPU := int32(75)
	if mt.Spec.HPA.TargetCPU != nil {
		targetCPU = *mt.Spec.HPA.TargetCPU
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mt.Name + "-hpa",
			Namespace: namespace,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "moodle",
			},
			MinReplicas: &minReplicas,
			MaxReplicas: mt.Spec.HPA.MaxReplicas,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: corev1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							Type:               autoscalingv2.UtilizationMetricType,
							AverageUtilization: &targetCPU,
						},
					},
				},
			},
		},
	}

	// Set MoodleTenant instance as the owner
	if err := ctrl.SetControllerReference(mt, hpa, r.Scheme); err != nil {
		return nil
	}

	return hpa
}

func (r *MoodleTenantReconciler) cronJobForMoodle(mt *moodlev1alpha1.MoodleTenant, namespace string) *batchv1.CronJob {
	// Run Moodle's cron.php every 5 minutes (standard Moodle recommendation)
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mt.Name + "-cron",
			Namespace: namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *", // Every 5 minutes
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							SecurityContext: &corev1.PodSecurityContext{
								RunAsNonRoot: ptr.To(true),
								RunAsUser:    ptr.To[int64](33), // www-data
								FSGroup:      ptr.To[int64](33),
							},
							Containers: []corev1.Container{
								{
									Name:  "moodle-cron",
									Image: mt.Spec.Image,
									Command: []string{
										"/usr/local/bin/php",
										"/var/www/html/admin/cli/cron.php",
									},
									Env: []corev1.EnvVar{
										{
											Name: "MOODLE_DATABASE_HOST",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: mt.Spec.DatabaseRef.AdminSecret,
													},
													Key: "host",
												},
											},
										},
										{
											Name: "MOODLE_DATABASE_NAME",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: mt.Spec.DatabaseRef.AdminSecret,
													},
													Key: "database",
												},
											},
										},
										{
											Name: "MOODLE_DATABASE_USER",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: mt.Spec.DatabaseRef.AdminSecret,
													},
													Key: "username",
												},
											},
										},
										{
											Name: "MOODLE_DATABASE_PASSWORD",
											ValueFrom: &corev1.EnvVarSource{
												SecretKeyRef: &corev1.SecretKeySelector{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: mt.Spec.DatabaseRef.AdminSecret,
													},
													Key: "password",
												},
											},
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "moodledata",
											MountPath: "/var/www/moodledata",
										},
									},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("100m"),
											corev1.ResourceMemory: resource.MustParse("256Mi"),
										},
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("500m"),
											corev1.ResourceMemory: resource.MustParse("512Mi"),
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "moodledata",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: mt.Name + "-data",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Set MoodleTenant instance as the owner
	if err := ctrl.SetControllerReference(mt, cronJob, r.Scheme); err != nil {
		return nil
	}

	return cronJob
}

func (r *MoodleTenantReconciler) pdbForMoodle(mt *moodlev1alpha1.MoodleTenant, namespace string) *policyv1.PodDisruptionBudget {
	labels := map[string]string{
		"app":                  "moodle",
		"moodle.bsu.by/tenant": mt.Name,
	}

	// Ensure at least 1 pod is available during disruptions
	minAvailable := intstr.FromInt(1)

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mt.Name + "-pdb",
			Namespace: namespace,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MinAvailable: &minAvailable,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		},
	}

	// Set MoodleTenant instance as the owner
	if err := ctrl.SetControllerReference(mt, pdb, r.Scheme); err != nil {
		return nil
	}

	return pdb
}

// Helper functions
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// SetupWithManager sets up the controller with the Manager.
func (r *MoodleTenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&moodlev1alpha1.MoodleTenant{}).
		Owns(&corev1.Namespace{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&batchv1.CronJob{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Named("moodletenant").
		Complete(r)
}
