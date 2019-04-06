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
	"time"

	errors "golang.org/x/xerrors"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	packetnetv1alpha1 "github.com/kkohtaka/packet-launcher/pkg/apis/packetnet/v1alpha1"
	"github.com/kkohtaka/packet-launcher/pkg/client/packet"
	finalizerutil "github.com/kkohtaka/packet-launcher/pkg/util/finalizer"
)

const (
	controllerName = "device-controller"

	defaultPacketSecretName = "packet-secret"
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
// +kubebuilder:rbac:groups=packetnet.kkohtaka.org,resources=devices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=secret,verbs=get
func (r *ReconcileDevice) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Device instance
	instance := &packetnetv1alpha1.Device{}
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

	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      defaultPacketSecretName,
	}
	if err := r.Get(context.TODO(), secretKey, secret); err != nil {
		return reconcile.Result{}, errors.Errorf("get Secret %v: %w", secretKey, err)
	}

	packet, err := packet.NewClient(secret)
	if err != nil {
		return reconcile.Result{}, errors.Errorf("create Packet client: %w", err)
	}

	if finalizerutil.IsDeleting(instance) {
		if err := removeExternalDependency(instance, packet); err != nil {
			return reconcile.Result{}, errors.Errorf("remove external dependencies: %w", err)
		}
		if err := newUpdater(r, instance).removeFinalizer().update(context.Background()); err != nil {
			return reconcile.Result{}, errors.Errorf("remove finalizer: %w", err)
		}
		klog.Infof("Device %v was finalized", request.NamespacedName)
		return reconcile.Result{}, nil
	}

	if !finalizerutil.HasFinalizer(instance) {
		if err := newUpdater(r, instance).setFinalizer().update(context.Background()); err != nil {
			return reconcile.Result{}, errors.Errorf("set finalizer: %w", err)
		}
	}

	if status, err := prepareExternalDependency(instance, packet); err != nil {
		return reconcile.Result{}, errors.Errorf("prepare external dependency: %w", err)
	} else if !reflect.DeepEqual(status, &instance.Status) {
		if err := r.Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, errors.Errorf("update Device: %w", err)
		}
	}

	if !instance.Status.Ready {
		return reconcile.Result{
			RequeueAfter: 15 * time.Second,
		}, nil
	}

	// Don't requeue and use a prober instead
	return reconcile.Result{
		RequeueAfter: 5 * time.Minute,
	}, nil
}

func prepareExternalDependency(
	pd *packetnetv1alpha1.Device,
	c packet.Client,
) (*packetnetv1alpha1.DeviceStatus, error) {
	var status *packetnetv1alpha1.DeviceStatus
	var err error
	if pd.Status.ID != "" {
		if status, err = c.CreateDevice(&pd.Spec); err != nil {
			return nil, errors.Errorf("create Packet device: %w", err)
		}
	} else {
		status, err = c.GetDevice(pd.Status.ID)
		if err != nil {
			return nil, errors.Errorf("get Packet device: %w", err)
		}
		if shouldUpdateDevice(&pd.Spec, status) {
			status, err = c.UpdateDevice(pd.Status.ID, &pd.Spec)
			if err != nil {
				return nil, errors.Errorf("update Packet device: %w", err)
			}
		}
	}
	return status, nil
}

func removeExternalDependency(pd *packetnetv1alpha1.Device, c packet.Client) error {
	if pd.Status.ID != "" {
		if err := c.DeleteDevice(pd.Status.ID); err != nil {
			return errors.Errorf("delete Packet device: %w", err)
		}
	}
	return nil
}

func shouldUpdateDevice(
	spec *packetnetv1alpha1.DeviceSpec,
	status *packetnetv1alpha1.DeviceStatus,
) bool {
	// TODO: Implement this function by appending properties on DeviceStatus
	return false
}
