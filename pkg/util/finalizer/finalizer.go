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

package finalizer

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
)

func IsDeleting(o runtime.Object) bool {
	accessor, err := meta.Accessor(o)
	if err != nil {
		klog.Errorf("Could not access to meta object: %v", err)
		return false
	}
	return accessor.GetDeletionTimestamp() != nil
}

func HasFinalizer(o runtime.Object, finalizerName string) bool {
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

func SetFinalizer(o runtime.Object, finalizerName string) {
	if !HasFinalizer(o, finalizerName) {
		accessor, err := meta.Accessor(o)
		if err != nil {
			klog.Errorf("Could not access to meta object: %v", err)
			return
		}
		accessor.SetFinalizers(append(accessor.GetFinalizers(), finalizerName))
	}
}

func RemoveFinalizer(o runtime.Object, finalizerName string) {
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
