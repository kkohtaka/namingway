/*
Copyright 2018 Kazumasa Kohtaka <kkohtaka@gmail.com>.

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

package packetdevice

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"github.com/pkg/errors"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cloudprovidersv1alpha1 "github.com/kkohtaka/namingway/pkg/apis/cloudproviders/v1alpha1"
	genericv1alpha1 "github.com/kkohtaka/namingway/pkg/apis/generic/v1alpha1"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new PacketDevice Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this cloudproviders.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePacketDevice{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("packetdevice-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to PacketDevice
	err = c.Watch(&source.Kind{Type: &cloudprovidersv1alpha1.PacketDevice{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &genericv1alpha1.DNSRecord{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &cloudprovidersv1alpha1.PacketDevice{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcilePacketDevice{}

// ReconcilePacketDevice reconciles a PacketDevice object
type ReconcilePacketDevice struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a PacketDevice object and makes changes based on the state read
// and what is in the PacketDevice.Spec
// +kubebuilder:rbac:groups=cloudproviders.kohtaka.org,resources=packetdevices,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcilePacketDevice) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the PacketDevice instance
	instance := &cloudprovidersv1alpha1.PacketDevice{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	original := instance.DeepCopy()

	dr := &genericv1alpha1.DNSRecord{}
	if instance.Status.DNSRecordRef == nil {
		dr.Namespace = instance.Namespace
		dr.Name = instance.Name

		dr.Spec = *newDNSRecordSpec(instance)
		if err := controllerutil.SetControllerReference(instance, dr, r.scheme); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "set owner reference")
		}

		log.Printf("Creating DNSRecord %s/%s\n", dr.Namespace, dr.Name)
		if err := r.Create(context.TODO(), dr); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "create DNSRecord")
		}
	} else {
		objKey := types.NamespacedName{
			Namespace: instance.Namespace,
			Name:      instance.Name,
		}
		if err := r.Get(context.TODO(), objKey, dr); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "get DNSRecord")
		}

		original := dr.DeepCopy()

		dr.Spec = *newDNSRecordSpec(instance)
		if err := controllerutil.SetControllerReference(instance, dr, r.scheme); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "set owner reference")
		}

		if !reflect.DeepEqual(original, dr) {
			log.Printf("Updating DNSRecord %s/%s\n", dr.Namespace, dr.Name)
			if err := r.Update(context.TODO(), dr); err != nil {
				return reconcile.Result{}, errors.Wrap(err, "update DNSRecord")
			}
		}
	}

	instance.Status.DNSRecordRef = &cloudprovidersv1alpha1.DNSRecordRef{
		Name: dr.Name,
	}

	if !reflect.DeepEqual(original.Status, instance.Status) {
		log.Printf("Updating PacketDevice %s/%s\n", instance.Namespace, instance.Name)
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func newDNSRecordSpec(
	pd *cloudprovidersv1alpha1.PacketDevice,
) *genericv1alpha1.DNSRecordSpec {
	return &genericv1alpha1.DNSRecordSpec{
		Domain: fmt.Sprintf(
			"%s.%s", pd.Status.Hostname, pd.Status.ProjectName),
		A: pd.Status.PublicIPAddresses,
	}
}
