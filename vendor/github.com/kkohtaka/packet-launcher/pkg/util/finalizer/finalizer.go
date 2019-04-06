package finalizer

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
)

const (
	finalizerName = "finalizer.kkohtaka.org"
)

func IsDeleting(o runtime.Object) bool {
	accessor, err := meta.Accessor(o)
	if err != nil {
		klog.Errorf("Could not access to meta object: %v", err)
		return false
	}
	return accessor.GetDeletionTimestamp() != nil
}

func HasFinalizer(o runtime.Object) bool {
	accessor, err := meta.Accessor(o)
	if err != nil {
		klog.Errorf("Could not access to meta object: %v", err)
		return false
	}
	fs := accessor.GetFinalizers()
	for _, finalizer := range fs {
		if finalizer == finalizerName {
			return true
		}
	}
	return false
}

func SetFinalizer(o runtime.Object) {
	if !HasFinalizer(o) {
		accessor, err := meta.Accessor(o)
		if err != nil {
			klog.Errorf("Could not access to meta object: %v", err)
			return
		}
		accessor.SetFinalizers(append(accessor.GetFinalizers(), finalizerName))
	}
}

func RemoveFinalizer(o runtime.Object) {
	accessor, err := meta.Accessor(o)
	if err != nil {
		klog.Errorf("Could not access to meta object: %v", err)
		return
	}
	fs := accessor.GetFinalizers()
	for i := range fs {
		if fs[i] == finalizerName {
			accessor.SetFinalizers(append(fs[:i], fs[i+1:]...))
			return
		}
	}
}
