package packet

import (
	"github.com/packethost/packngo"

	errors "golang.org/x/xerrors"

	corev1 "k8s.io/api/core/v1"

	packetnetv1alpha1 "github.com/kkohtaka/packet-launcher/pkg/apis/packetnet/v1alpha1"
)

type Client interface {
	// CreateDevice creates a device on Packet
	CreateDevice(deviceSpec *packetnetv1alpha1.DeviceSpec) (*packetnetv1alpha1.DeviceStatus, error)
	// GetDevice gets a device on Packet
	GetDevice(deviceID string) (*packetnetv1alpha1.DeviceStatus, error)
	// UpdateDevice updates a device on Packet
	UpdateDevice(deviceID string, deviceSpec *packetnetv1alpha1.DeviceSpec) (*packetnetv1alpha1.DeviceStatus, error)
	// DeleteDevice deletes a device on Packet
	DeleteDevice(deviceID string) error
}

const (
	secretKeyAPIKey = "apiKey"

	defaultBillingCycle = "hourly"
)

func NewClient(secret *corev1.Secret) (Client, error) {
	var (
		apiKey []byte
		ok     bool
	)
	if apiKey, ok = secret.Data[secretKeyAPIKey]; !ok {
		return nil, errors.Errorf(
			"secret %v/%v doesn't contain a key %v", secret.Namespace, secret.Name, secretKeyAPIKey)
	}
	return &client{
		c: packngo.NewClientWithAuth("", string(apiKey), nil),
	}, nil
}

type client struct {
	c *packngo.Client
}

func (c *client) CreateDevice(deviceSpec *packetnetv1alpha1.DeviceSpec) (*packetnetv1alpha1.DeviceStatus, error) {
	if deviceSpec.BillingCycle == "" {
		deviceSpec.BillingCycle = defaultBillingCycle
	}
	d, _, err := c.c.Devices.Create(
		&packngo.DeviceCreateRequest{
			ProjectID:    deviceSpec.ProjectID,
			Facility:     []string{deviceSpec.Facility},
			Plan:         deviceSpec.Plan,
			Hostname:     deviceSpec.Hostname,
			OS:           deviceSpec.OS,
			BillingCycle: deviceSpec.BillingCycle,
			UserData:     deviceSpec.UserData,
		},
	)
	if err != nil {
		return nil, err
	}
	return newStatus(d), nil
}

func (c *client) GetDevice(deviceID string) (*packetnetv1alpha1.DeviceStatus, error) {
	d, _, err := c.c.Devices.Get(deviceID, nil)
	if err != nil {
		return nil, err
	}
	return newStatus(d), nil
}

func (c *client) UpdateDevice(
	deviceID string,
	deviceSpec *packetnetv1alpha1.DeviceSpec,
) (*packetnetv1alpha1.DeviceStatus, error) {
	if deviceSpec.BillingCycle == "" {
		deviceSpec.BillingCycle = defaultBillingCycle
	}
	d, _, err := c.c.Devices.Create(
		&packngo.DeviceCreateRequest{
			ProjectID:    deviceSpec.ProjectID,
			Facility:     []string{deviceSpec.Facility},
			Plan:         deviceSpec.Plan,
			Hostname:     deviceSpec.Hostname,
			OS:           deviceSpec.OS,
			BillingCycle: deviceSpec.BillingCycle,
			UserData:     deviceSpec.UserData,
		},
	)
	if err != nil {
		return nil, err
	}
	return newStatus(d), nil
}

func (c *client) DeleteDevice(deviceID string) error {
	if _, err := c.c.Devices.Delete(deviceID); err != nil {
		return err
	}
	return nil
}

func newStatus(d *packngo.Device) *packetnetv1alpha1.DeviceStatus {
	status := &packetnetv1alpha1.DeviceStatus{}
	status.State = packetnetv1alpha1.StringToState(d.State)
	status.ID = d.ID
	status.IPAddresses = make([]packetnetv1alpha1.IPAddress, len(d.Network))
	for i := range d.Network {
		ipAddress := d.Network[i]
		status.IPAddresses[i] = packetnetv1alpha1.IPAddress{
			ID:            ipAddress.ID,
			Address:       ipAddress.Address,
			Gateway:       ipAddress.Gateway,
			Network:       ipAddress.Network,
			AddressFamily: ipAddress.AddressFamily,
			Netmask:       ipAddress.Netmask,
			Public:        ipAddress.Public,
		}
	}

	status.Ready = status.State == packetnetv1alpha1.StateActive

	return status
}
