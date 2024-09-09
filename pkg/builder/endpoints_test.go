package builder

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"testing"

	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/dns-operator/api/v1alpha1"
)

type GatewayTarget struct {
	*gatewayapiv1.Gateway
	hostname string
}

func (t GatewayTarget) GetName() string {
	return fmt.Sprintf("%s-%s", t.Gateway.Name, t.Gateway.Namespace)
}

func (t GatewayTarget) GetShortCode() string {
	return t.GetName()
	//return common.ToBase36HashLen(t.GetName(), utils.ClusterIDLength)
}

func (t GatewayTarget) GetAddresses() []TargetAddress {
	targetAddrs := []TargetAddress{}
	for gws := range t.Status.Addresses {
		targetAddrs = append(targetAddrs, TargetAddress{
			Type:  AddressType(*t.Status.Addresses[gws].Type),
			Value: t.Status.Addresses[gws].Value,
		})
	}
	return targetAddrs
}

func (t GatewayTarget) GetHostname() string {
	return t.hostname
}

func TestEndpointsBuilder_Build(t *testing.T) {
	gw := &gatewayapiv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testgw",
			Namespace: "testgw",
		},
		Spec: gatewayapiv1.GatewaySpec{
			Listeners: []gatewayapiv1.Listener{
				{
					Name:     "testlistener",
					Hostname: ptr.To(gatewayapiv1.Hostname("foo.example.com")),
				},
			},
			Addresses: nil,
		},
		Status: gatewayapiv1.GatewayStatus{
			Addresses: []gatewayapiv1.GatewayStatusAddress{
				{
					Type:  ptr.To(gatewayapiv1.IPAddressType),
					Value: "127.0.0.1",
				},
			},
			Listeners: []gatewayapiv1.ListenerStatus{
				{
					Name:           "testlistener",
					SupportedKinds: []gatewayapiv1.RouteGroupKind{},
					AttachedRoutes: 1,
					Conditions:     []metav1.Condition{},
				},
			},
		},
	}
	gwTarget := &GatewayTarget{
		Gateway:  gw,
		hostname: string(*gw.Spec.Listeners[0].Hostname),
	}
	endpoints, err := NewEndpointsBuilder().
		ForTarget(gwTarget).
		WithLoadBalancing(&v1alpha1.LoadBalancingSpec{
			Weighted: v1alpha1.LoadBalancingWeighted{
				DefaultWeight: 100,
				Custom:        nil,
			},
			Geo: v1alpha1.LoadBalancingGeo{
				DefaultGeo: "EU",
			},
		}).
		Build()
	fmt.Printf("endpoints: %v, err: %v\n", endpoints, err)
}
