package builder

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kuadrant/dns-operator/api/v1alpha1"
)

// DNSRecordBuilder builds a DNSRecord.
type DNSRecordBuilder struct {
	name            string
	namespace       string
	ownerID         string
	rootHost        string
	providerRef     v1alpha1.ProviderRef
	endpointBuilder *EndpointsBuilder
}

// NewDNSRecordBuilder returns a new dnsrecord builder
func NewDNSRecordBuilder(name, namespace string) *DNSRecordBuilder {
	return &DNSRecordBuilder{
		name:      name,
		namespace: namespace,
	}
}

func (blder *DNSRecordBuilder) ForTarget(target Target) *DNSRecordBuilder {
	if blder.endpointBuilder == nil {
		blder.endpointBuilder = NewEndpointsBuilder()
	}
	blder.endpointBuilder = blder.endpointBuilder.ForTarget(target)
	blder.namespace = target.GetNamespace()
	return blder
}

func (blder *DNSRecordBuilder) ForRoutingStrategy(rs v1alpha1.RoutingStrategy) *DNSRecordBuilder {
	blder.endpointBuilder = blder.endpointBuilder.ForRoutingStrategy(rs)
	return blder
}

func (blder *DNSRecordBuilder) WithName(name string) *DNSRecordBuilder {
	blder.name = name
	return blder
}

func (blder *DNSRecordBuilder) WithNamespace(namespace string) *DNSRecordBuilder {
	blder.namespace = namespace
	return blder
}

func (blder *DNSRecordBuilder) WithOwnerID(ownerID string) *DNSRecordBuilder {
	blder.ownerID = ownerID
	return blder
}

func (blder *DNSRecordBuilder) WithRootHost(rootHost string) *DNSRecordBuilder {
	blder.rootHost = rootHost
	return blder
}

func (blder *DNSRecordBuilder) WithProviderRef(providerRef v1alpha1.ProviderRef) *DNSRecordBuilder {
	blder.providerRef = providerRef
	return blder
}

// Build builds and returns the DNSRecord.
func (blder *DNSRecordBuilder) Build() (*v1alpha1.DNSRecord, error) {
	eps, err := blder.endpointBuilder.Build()
	if err != nil {
		return nil, err
	}
	return &v1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      blder.name,
			Namespace: blder.namespace,
		},
		Spec: v1alpha1.DNSRecordSpec{
			OwnerID:     blder.ownerID,
			RootHost:    blder.rootHost,
			ProviderRef: blder.providerRef,
			Endpoints:   eps,
		},
	}, nil
}
