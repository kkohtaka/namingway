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
	"log"
	"reflect"

	"github.com/packethost/packngo"
	"github.com/pkg/errors"

	cloudprovidersv1alpha1 "github.com/kkohtaka/namingway/pkg/apis/cloudproviders/v1alpha1"
	"k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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

	// Watch for changes to Packet
	err = c.Watch(&source.Kind{Type: &cloudprovidersv1alpha1.Packet{}}, &handler.EnqueueRequestForObject{})
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
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cloudproviders.kohtaka.org,resources=packets,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcilePacket) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Packet instance
	instance := &cloudprovidersv1alpha1.Packet{}
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
	log.Printf("Reconciling Packet resource %s/%s\n", instance.Namespace, instance.Name)

	original := instance.DeepCopy()

	var secret v1.Secret
	objKey := types.NamespacedName{
		Namespace: instance.Namespace,
		Name:      instance.Spec.Secret.SecretName,
	}
	err = r.Get(context.TODO(), objKey, &secret)
	if err != nil {
		return reconcile.Result{}, err
	}

	binToken, ok := secret.Data["token"]
	if !ok {
		return reconcile.Result{}, errors.Errorf(
			"get token from Secret %s/%s", secret.Namespace, secret.Name)
	}
	token := string(binToken)

	c := packngo.NewClientWithAuth("", token, nil)
	project, _, err := c.Projects.Get(instance.Spec.ProjectID, nil)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "get Packet project")
	}

	instance.Status.ProjectName = project.Name

	if !reflect.DeepEqual(original.Status, instance.Status) {
		log.Printf("Updating Packet %s/%s\n", instance.Namespace, instance.Name)
		err = r.Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}
