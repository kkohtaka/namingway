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

package dnsrecord

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	errors "golang.org/x/xerrors"

	dns "google.golang.org/api/dns/v1"
	option "google.golang.org/api/option"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	"github.com/kkohtaka/namingway/pkg/apis"
	networkv1alpha1 "github.com/kkohtaka/namingway/pkg/apis/network/v1alpha1"
	finalizerutil "github.com/kkohtaka/namingway/pkg/util/finalizer"
)

const (
	controllerName = "dnsrecord-controller"

	defaultNamespace  = metav1.NamespaceSystem
	defaultConfigName = "namingway"
	defaultSecretName = "namingway"

	defaultConfigKeyGCPProject        = "gcp-project"
	defaultConfigKeyGCPManagedZone    = "gcp-managed-zone"
	defaultSecretKeyGCPCredentialJSON = "gcp-credential-json"

	defaultTTL = 60
)

var (
	log                      = logf.Log.WithName(controllerName)
	_   reconcile.Reconciler = &ReconcileDNSRecord{}
)

// Add creates a new DNSRecord Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileDNSRecord{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to DNSRecord
	err = c.Watch(&source.Kind{Type: &networkv1alpha1.DNSRecord{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

// ReconcileDNSRecord reconciles a DNSRecord object
type ReconcileDNSRecord struct {
	client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a DNSRecord object and makes changes based on the state read
// and what is in the DNSRecord.Spec
// +kubebuilder:rbac:groups=network.kkohtaka.org,resources=dnsrecords,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileDNSRecord) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the DNSRecord instance
	instance := &networkv1alpha1.DNSRecord{}
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

	project, managedZone, err := getGCPConfig(r)
	if err != nil {
		return reconcile.Result{}, errors.Errorf("get GCP config: %w", err)
	}

	credential, err := getGCPCredential(r)
	if err != nil {
		return reconcile.Result{}, errors.Errorf("get GCP credential: %w", err)
	}

	if finalizerutil.IsDeleting(instance) {
		if err := removeExternalDependency(instance, project, managedZone, credential); err != nil {
			return reconcile.Result{}, errors.Errorf("remove external dependencies of %v: %w", request.NamespacedName, err)
		}
		if err := newUpdater(r, instance).removeFinalizer(apis.FinalizerName).update(context.TODO()); err != nil {
			return reconcile.Result{}, errors.Errorf("remove finalizer of %v: %w", request.NamespacedName, err)
		}
		klog.Infof("DNSRecord %v was finalized by %v", request.NamespacedName, apis.ProductName)
		return reconcile.Result{}, nil
	}

	if !finalizerutil.HasFinalizer(instance, apis.FinalizerName) {
		if err := newUpdater(r, instance).setFinalizer(apis.FinalizerName).update(context.TODO()); err != nil {
			return reconcile.Result{}, errors.Errorf("set finalizer: %w", err)
		}
	}

	if err := prepareExternalDependency(instance, project, managedZone, credential); err != nil {
		if e := newUpdater(r, instance).ready(false).update(context.TODO()); e != nil {
			return reconcile.Result{}, errors.Errorf("update DNSRecord %v: %w", request.NamespacedName, e)
		}
		return reconcile.Result{}, errors.Errorf("prepare external dependencies of %v: %w", request.NamespacedName, err)
	}

	if err := newUpdater(r, instance).ready(true).update(context.TODO()); err != nil {
		return reconcile.Result{}, errors.Errorf("update DNSRecord %v: %w", request.NamespacedName, err)
	}
	return reconcile.Result{}, nil
}

func getGCPConfig(c client.Client) (string, string, error) {
	var config corev1.ConfigMap
	objKey := types.NamespacedName{
		Namespace: defaultNamespace,
		Name:      defaultConfigName,
	}
	if err := c.Get(context.TODO(), objKey, &config); err != nil {
		return "", "", errors.Errorf("get ConfigMap %v: %w", objKey, err)
	}
	project, ok := config.Data[defaultConfigKeyGCPProject]
	if !ok {
		return "", "", errors.Errorf("get GCP project name from config: %v", objKey)
	}
	managedZone, ok := config.Data[defaultConfigKeyGCPManagedZone]
	if !ok {
		return "", "", errors.Errorf("get GCP managed zone name from config: %v", objKey)
	}
	return project, managedZone, nil
}

func getGCPCredential(c client.Client) ([]byte, error) {
	var secret corev1.Secret
	objKey := types.NamespacedName{
		Namespace: defaultNamespace,
		Name:      defaultSecretName,
	}
	if err := c.Get(context.TODO(), objKey, &secret); err != nil {
		return nil, errors.Errorf("get Secret %v: %w", objKey, err)
	}
	credential, ok := secret.Data[defaultSecretKeyGCPCredentialJSON]
	if !ok {
		return nil, errors.Errorf("get GCP credential JSON from secret: %v", objKey)
	}
	return credential, nil
}

func prepareExternalDependency(
	dnsRecord *networkv1alpha1.DNSRecord,
	project, managedZone string,
	credential []byte,
) error {
	svc, err := dns.NewService(context.TODO(), option.WithCredentialsJSON(credential))
	if err != nil {
		return errors.Errorf("create Cloud DNS client: %w", err)
	}

	zone, err := svc.ManagedZones.Get(project, managedZone).Do()
	if err != nil {
		return errors.Errorf("get managed zone %v/%v: %w", project, managedZone, err)
	}

	domain := fmt.Sprintf("%v.%v", dnsRecord.Spec.SubDomain, zone.DnsName)
	rrdatas := dnsRecord.Spec.A

	change := &dns.Change{
		Additions: []*dns.ResourceRecordSet{
			&dns.ResourceRecordSet{
				Name:    domain,
				Type:    "A",
				Ttl:     defaultTTL,
				Rrdatas: rrdatas,
			},
		},
	}

	var deletions []*dns.ResourceRecordSet
	err = svc.ResourceRecordSets.List(project, managedZone).Name(domain).Type("A").Pages(
		context.TODO(),
		func(resp *dns.ResourceRecordSetsListResponse) error {
			for _, rrset := range resp.Rrsets {
				deletions = append(deletions, rrset)
				break
			}
			return nil
		},
	)
	if err != nil {
		return errors.Errorf("get current A record of %v: %w", domain, err)
	}
	if len(deletions) > 0 {
		if shouldUpdateResourceRecordSet(change.Additions[0], deletions[0]) {
			return nil
		} else {
			change.Deletions = deletions
		}
	}

	_, err = svc.Changes.Create(project, managedZone, change).Do()
	if err != nil {
		return errors.Errorf("update A record of %v: %w", domain, err)
	}
	klog.Infof("Registered A record of %v: %v", domain, rrdatas)
	return nil
}

func removeExternalDependency(
	dnsRecord *networkv1alpha1.DNSRecord,
	project, managedZone string,
	credential []byte,
) error {
	svc, err := dns.NewService(context.TODO(), option.WithCredentialsJSON(credential))
	if err != nil {
		return errors.Errorf("create Cloud DNS client: %w", err)
	}

	zone, err := svc.ManagedZones.Get(project, managedZone).Do()
	if err != nil {
		return errors.Errorf("get managed zone %v/%v: %w", project, managedZone, err)
	}

	domain := fmt.Sprintf("%v.%v", dnsRecord.Spec.SubDomain, zone.DnsName)

	var deletions []*dns.ResourceRecordSet
	err = svc.ResourceRecordSets.List(project, managedZone).Name(domain).Type("A").Pages(
		context.TODO(),
		func(resp *dns.ResourceRecordSetsListResponse) error {
			for _, rrset := range resp.Rrsets {
				deletions = append(deletions, rrset)
				break
			}
			return nil
		},
	)
	if err != nil {
		return errors.Errorf("get current A record of %v: %w", domain, err)
	}
	if len(deletions) == 0 {
		klog.Infof("Could not find A record of %v", domain)
		return nil
	}

	change := &dns.Change{
		Deletions: deletions,
	}
	_, err = svc.Changes.Create(project, managedZone, change).Do()
	if err != nil {
		return errors.Errorf("update A record of %v: %w", domain, err)
	}
	klog.Infof("Removed A record of %v", domain)
	return nil
}

func shouldUpdateResourceRecordSet(x, y *dns.ResourceRecordSet) bool {
	if x.Name != y.Name {
		return true
	}
	if x.Type != y.Type {
		return true
	}
	if x.Ttl != y.Ttl {
		return true
	}
	xData, yData := x.Rrdatas, y.Rrdatas
	sort.Strings(xData)
	sort.Strings(yData)
	if !reflect.DeepEqual(xData, yData) {
		return true
	}
	return false
}
