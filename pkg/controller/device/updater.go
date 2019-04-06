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

	"sigs.k8s.io/controller-runtime/pkg/client"

	finalizerutil "github.com/kkohtaka/namingway/pkg/util/finalizer"
	packetnetv1alpha1 "github.com/kkohtaka/packet-launcher/pkg/apis/packetnet/v1alpha1"
)

type updater struct {
	oldObj, newObj *packetnetv1alpha1.Device
	c              client.Client
}

func newUpdater(c client.Client, device *packetnetv1alpha1.Device) *updater {
	return &updater{
		oldObj: device,
		newObj: device.DeepCopy(),
		c:      c,
	}
}

func (u *updater) setFinalizer(finalizer string) *updater {
	finalizerutil.SetFinalizer(u.newObj, finalizer)
	return u
}

func (u *updater) removeFinalizer(finalizer string) *updater {
	finalizerutil.RemoveFinalizer(u.newObj, finalizer)
	return u
}

func (u *updater) update(ctx context.Context) error {
	if !reflect.DeepEqual(u.newObj, u.oldObj) {
		return u.c.Update(ctx, u.newObj)
	}
	return nil
}
