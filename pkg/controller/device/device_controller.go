/*
Copyright 2019 Kazumasa Kohtaka <kkohtaka@gmail.com>.

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

package device

import (
	"context"
	"reflect"

	errors "golang.org/x/xerrors"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apis "github.com/kkohtaka/namingway/pkg/apis"
	networkv1alpha1 "github.com/kkohtaka/namingway/pkg/apis/network/v1alpha1"
	finalizerutil "github.com/kkohtaka/namingway/pkg/util/finalizer"
	packetnetv1alpha1 "github.com/kkohtaka/packet-launcher/pkg/apis/packetnet/v1alpha1"
)

const (
	controllerName = "device-watcher"
)

var (
	log                      = logf.Log.WithName(controllerName)
	_   reconcile.Reconciler = &ReconcileDevice{}
)

// Add creates a new Device Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDevice{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to Device
	err = c.Watch(&source.Kind{Type: &packetnetv1alpha1.Device{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

// ReconcileDevice reconciles a Device object
type ReconcileDevice struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Device object and makes changes based on the state read
// and what is in the Device.Spec
// +kubebuilder:rbac:groups=packetnet.kkohtaka.org,resources=devices,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=network.kkohtaka.org,resources=dnsrecords,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileDevice) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Device instance
	instance := &packetnetv1alpha1.Device{}
	if err := r.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		if kerrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if finalizerutil.IsDeleting(instance) {
		if err := removeExternalDependency(instance); err != nil {
			return reconcile.Result{}, errors.Errorf("remove external dependencies of %v: %w", request.NamespacedName, err)
		}
		if err := newUpdater(r, instance).removeFinalizer(apis.FinalizerName).update(context.Background()); err != nil {
			return reconcile.Result{}, errors.Errorf("remove finalizer of %v: %w", request.NamespacedName, err)
		}
		klog.Infof("Device %v was finalized by %v", request.NamespacedName, apis.ProductName)
		return reconcile.Result{}, nil
	}

	if !finalizerutil.HasFinalizer(instance, apis.FinalizerName) {
		if err := newUpdater(r, instance).setFinalizer(apis.FinalizerName).update(context.Background()); err != nil {
			return reconcile.Result{}, errors.Errorf("set finalizer: %w", err)
		}
	}

	if err := prepareExternalDependency(instance, r, r.scheme); err != nil {
		return reconcile.Result{}, errors.Errorf("prepare external dependencies of %v: %w", request.NamespacedName, err)
	}
	return reconcile.Result{}, nil
}

func prepareExternalDependency(device *packetnetv1alpha1.Device, c client.Client, scheme *runtime.Scheme) error {
	var dnsrecord *networkv1alpha1.DNSRecord
	objKey := types.NamespacedName{
		Name:      device.Name,
		Namespace: device.Namespace,
	}
	if err := c.Get(context.TODO(), objKey, dnsrecord); err != nil {
		if kerrors.IsNotFound(err) {
			dnsrecord = newDNSRecord(device)
			if err := controllerutil.SetControllerReference(device, dnsrecord, scheme); err != nil {
				return errors.Errorf("set controller reference on DNSRecord %v/%v: %w",
					dnsrecord.Namespace, dnsrecord.Name, err)
			}
			if err := c.Create(context.TODO(), dnsrecord); err != nil {
				return errors.Errorf("create DNSRecord %v/%v: %w", dnsrecord.Namespace, dnsrecord.Name, err)
			}
		}
		return errors.Errorf("get DNSRecord %v: %w", objKey, err)
	}
	updatedRecord := newDNSRecord(device)
	if !reflect.DeepEqual(dnsrecord.Spec, updatedRecord.Spec) {
		dnsrecord.Spec = updatedRecord.Spec
		if err := c.Update(context.TODO(), updatedRecord); err != nil {
			return errors.Errorf("update DNSRecord %v/%v: %w", updatedRecord.Namespace, updatedRecord.Name, err)
		}
	}
	return nil
}

func removeExternalDependency(device *packetnetv1alpha1.Device) error {
	return nil
}

func newDNSRecord(device *packetnetv1alpha1.Device) *networkv1alpha1.DNSRecord {
	var ipV4Addresses []string
	for i := range device.Status.IPAddresses {
		address := &device.Status.IPAddresses[i]
		if address.AddressFamily != 4 {
			continue
		}
		if !address.Public {
			continue
		}
		ipV4Addresses = append(ipV4Addresses, address.Address)
	}
	return &networkv1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      device.Name,
			Namespace: device.Namespace,
		},
		Spec: networkv1alpha1.DNSRecordSpec{
			SubDomain: device.Spec.Hostname,
			A:         ipV4Addresses,
		},
	}
}
