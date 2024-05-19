/*
Copyright 2024.

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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	scanv1alpha1 "github.com/EMAYAN08/clusterscan-operator/api/v1alpha1"
)

// ClusterScanReconciler reconciles a ClusterScan object
type ClusterScanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=scan.clusterscandemo.com,resources=clusterscans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=scan.clusterscandemo.com,resources=clusterscans/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=scan.clusterscandemo.com,resources=clusterscans/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=create;delete;get;list;patch;update;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ClusterScan object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *ClusterScanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the ClusterScan instance
	scan := &scanv1alpha1.ClusterScan{}
	err := r.Get(ctx, req.NamespacedName, scan)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Check if the scan is scheduled or needs to run immediately
	if scan.Spec.Schedule != "" {
		// Handle CronJob creation
		cronJob := &batchv1.CronJob{ // Use batch/v1
			ObjectMeta: metav1.ObjectMeta{
				Name:      scan.Name + "-cronjob",
				Namespace: scan.Namespace,
			},
			Spec: batchv1.CronJobSpec{
				Schedule: scan.Spec.Schedule,
				JobTemplate: batchv1.JobTemplateSpec{
					Spec: scan.Spec.JobTemplate,
				},
			},
		}

		// Set ClusterScan instance as the owner and controller
		if err := ctrl.SetControllerReference(scan, cronJob, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		// Check if the CronJob already exists
		found := &batchv1.CronJob{}
		err = r.Get(ctx, client.ObjectKey{Namespace: cronJob.Namespace, Name: cronJob.Name}, found)
		if err != nil && errors.IsNotFound(err) {
			log.Info("Creating a new CronJob", "CronJob.Namespace", cronJob.Namespace, "CronJob.Name", cronJob.Name)
			err = r.Create(ctx, cronJob)
			if err != nil {
				return ctrl.Result{}, err
			}
		} else if err != nil {
			return ctrl.Result{}, err
		}

		// Update the ClusterScan status
		scan.Status.LastScanTime = &metav1.Time{Time: time.Now()}
		scan.Status.LastScanResult = "CronJob created"
		err = r.Status().Update(ctx, scan)
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// Handle one-time Job creation
		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      scan.Name + "-job",
				Namespace: scan.Namespace,
			},
			Spec: scan.Spec.JobTemplate,
		}

		// Set ClusterScan instance as the owner and controller
		if err := ctrl.SetControllerReference(scan, job, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		// Check if the Job already exists
		found := &batchv1.Job{}
		err = r.Get(ctx, client.ObjectKey{Namespace: job.Namespace, Name: job.Name}, found)
		if err != nil && errors.IsNotFound(err) {
			log.Info("Creating a new Job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
			err = r.Create(ctx, job)
			if err != nil {
				return ctrl.Result{}, err
			}
		} else if err != nil {
			return ctrl.Result{}, err
		}

		// Update the ClusterScan status
		scan.Status.LastScanTime = &metav1.Time{Time: time.Now()}
		scan.Status.LastScanResult = "Job created"
		err = r.Status().Update(ctx, scan)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterScanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&scanv1alpha1.ClusterScan{}).
		Owns(&batchv1.Job{}).
		Owns(&batchv1.CronJob{}). // Updated to use batch/v1
		Complete(r)
}
