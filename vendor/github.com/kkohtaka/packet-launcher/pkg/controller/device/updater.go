package device

import (
	"context"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	packetnetv1alpha1 "github.com/kkohtaka/packet-launcher/pkg/apis/packetnet/v1alpha1"
	finalizerutil "github.com/kkohtaka/packet-launcher/pkg/util/finalizer"
)

type updater struct {
	oldObj, newObj *packetnetv1alpha1.Device
	c              client.Client
}

func newUpdater(c client.Client, pd *packetnetv1alpha1.Device) *updater {
	return &updater{
		oldObj: pd,
		newObj: pd.DeepCopy(),
		c:      c,
	}
}

func (u *updater) setFinalizer() *updater {
	finalizerutil.SetFinalizer(u.newObj)
	return u
}
func (u *updater) removeFinalizer() *updater {
	finalizerutil.RemoveFinalizer(u.newObj)
	return u
}

func (u *updater) update(ctx context.Context) error {
	if !reflect.DeepEqual(u.newObj, u.oldObj) {
		return u.c.Update(ctx, u.newObj)
	}
	return nil
}
