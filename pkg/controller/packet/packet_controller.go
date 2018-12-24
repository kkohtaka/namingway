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

package packet

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/packethost/packngo"
	"github.com/pkg/errors"

	cloudprovidersv1alpha1 "github.com/kkohtaka/namingway/pkg/apis/cloudproviders/v1alpha1"
	"k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new Packet Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this cloudproviders.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePacket{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("packet-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	mgr.GetCache().IndexField(
		&cloudprovidersv1alpha1.PacketDevice{},
		"spec.id",
		client.IndexerFunc(func(o runtime.Object) []string {
			var res []string
			if pd, ok := o.(*cloudprovidersv1alpha1.PacketDevice); ok {
				res = append(res, pd.Spec.ID)
			}
			return res
		}),
	)

	// Watch for changes to Packet
	err = c.Watch(&source.Kind{Type: &cloudprovidersv1alpha1.Packet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &cloudprovidersv1alpha1.PacketDevice{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &cloudprovidersv1alpha1.Packet{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcilePacket{}

// ReconcilePacket reconciles a Packet object
type ReconcilePacket struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Packet object and makes changes based on the state read
// and what is in the Packet.Spec
// +kubebuilder:rbac:groups=cloudproviders.kohtaka.org,resources=packets;packetdevices,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcilePacket) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Packet instance
	instance := &cloudprovidersv1alpha1.Packet{}
	if err := r.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		if kerrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	log.Printf("Reconciling Packet resource %s/%s\n", instance.Namespace, instance.Name)

	original := instance.DeepCopy()

	// Get access token from Secret

	var secret v1.Secret
	objKey := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.Secret.SecretName,
	}
	if err := r.Get(context.TODO(), objKey, &secret); err != nil {
		return reconcile.Result{}, err
	}

	binToken, ok := secret.Data["token"]
	if !ok {
		return reconcile.Result{}, errors.Errorf(
			"get token from Secret %s/%s", secret.Namespace, secret.Name)
	}
	token := string(binToken)

	// Get project name from Packet API

	c := packngo.NewClientWithAuth("", token, nil)
	project, _, err := c.Projects.Get(instance.Spec.ProjectID, nil)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "get Packet project")
	}

	instance.Status.ProjectName = project.Name

	// Get devices from Packet API

	devices, _, err := c.Devices.List(instance.Spec.ProjectID, nil)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "get devices in Packet project")
	}
	for i := range devices {
		d := &devices[i]
		pds := &cloudprovidersv1alpha1.PacketDeviceList{}
		opts := client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("spec.id", d.ID),
		}
		if err := r.List(context.TODO(), &opts, pds); err != nil {
			return reconcile.Result{}, errors.Wrap(err, "list PacketDevice")
		}

		if len(pds.Items) > 0 {
			pd := &pds.Items[0]
			old := pd.DeepCopy()
			pd.Status.ProjectName = instance.Status.ProjectName
			pd.Status.Hostname = d.Hostname
			pd.Status.PublicIPAddresses = getPublicIPAddresses(d)

			if err := controllerutil.SetControllerReference(instance, pd, r.scheme); err != nil {
				return reconcile.Result{}, errors.Wrap(err, "set owner reference")
			}

			if !reflect.DeepEqual(old, pd) {
				log.Printf("Updating PacketDevice %s/%s\n", pd.Namespace, pd.Name)
				if err := r.Update(context.TODO(), pd); err != nil {
					return reconcile.Result{}, errors.Wrap(err, "update PacketDevice")
				}
			}
		} else {
			pd := &cloudprovidersv1alpha1.PacketDevice{}
			pd.Namespace = instance.Namespace
			pd.Name = packetDeviceName(instance.Name, d.Hostname, d.ID)
			pd.Spec.ID = d.ID
			pd.Status.ProjectName = instance.Status.ProjectName
			pd.Status.Hostname = d.Hostname
			pd.Status.PublicIPAddresses = getPublicIPAddresses(d)

			if err := controllerutil.SetControllerReference(instance, pd, r.scheme); err != nil {
				return reconcile.Result{}, errors.Wrap(err, "set owner reference")
			}

			log.Printf("Creating PacketDevice %s/%s\n", pd.Namespace, pd.Name)
			if err := r.Create(context.TODO(), pd); err != nil {
				return reconcile.Result{}, errors.Wrap(err, "create PacketDevice")
			}
		}
	}

	if !reflect.DeepEqual(original.Status, instance.Status) {
		log.Printf("Updating Packet %s/%s\n", instance.Namespace, instance.Name)
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{
		RequeueAfter: time.Minute,
	}, nil
}

func packetDeviceName(pName, hostname, id string) string {
	return fmt.Sprintf("%s-%s-%s", pName, hostname, id[:6])
}

func getPublicIPAddresses(d *packngo.Device) []string {
	var res []string
	for i := range d.Network {
		n := d.Network[i]
		if n.Public && n.AddressFamily == 4 {
			res = append(res, n.Address)
		}
	}
	return res
}
