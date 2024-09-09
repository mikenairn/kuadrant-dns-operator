/*
Copyright 2024.

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

package builder

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	externaldns "sigs.k8s.io/external-dns/endpoint"

	"github.com/kuadrant/dns-operator/api/v1alpha1"
)

type AddressType string

const IPAddressType AddressType = "IPAddress"

type TargetAddress struct {
	Type  AddressType
	Value string
}

// Target wraps a kubernetes ingress traffic resource e.g.Gateway, Ingress, Route etc.. but can wrap any resources
// that has the desired geo and weight labels being applied, and can provide the required target hostname and address data.
// This should be implemented as required for each type of ingress resource i.e. Gateway (These are not implemented here,
// create a concrete implementation of this in kuadrant operator for Gateways to avoid the dependency in this repo)
type Target interface {
	metav1.Object
	GetName() string
	GetShortCode() string
	GetHostname() string
	GetAddresses() []TargetAddress
}

// EndpointsBuilder builds an endpoints array.
type EndpointsBuilder struct {
	// target kubernetes resource that may have geo/weight labels applied and provides target addresses and hostname information (Optional).
	// If target is set a list of endpoints for the target will be created and merged with any additional endpoints added to the builder (WithEndpoint).
	target *Target
	// routingStrategy to be used [Simple|LoadBalanced] (Optional). Arguably redundant if we just rely on loadBalancing
	routingStrategy v1alpha1.RoutingStrategy
	// loadBalancing specification (Optional),
	// If set the builder will create a loadbalanced set of endpoints for the target resource.
	// If unset, the builder will create a simple set of endpoints for the target resource.
	loadBalancing *v1alpha1.LoadBalancingSpec
	// endpoints list of endpoints that will be returned by the builder.
	// Endpoints can be added to the builder (WithEndpoint) and will be merged with any generated Endpoints for the target resource.
	endpoints []*externaldns.Endpoint
}

// NewEndpointsBuilder returns a new endpoints builder
func NewEndpointsBuilder() *EndpointsBuilder {
	return &EndpointsBuilder{}
}

// ForTarget a target ingress resource for which endpoints should be generated
func (blder *EndpointsBuilder) ForTarget(target Target) *EndpointsBuilder {
	blder.target = &target
	return blder
}

// ForRoutingStrategy the routing strategy to be used when generating the endpoints for the target resource if set.
func (blder *EndpointsBuilder) ForRoutingStrategy(rs v1alpha1.RoutingStrategy) *EndpointsBuilder {
	blder.routingStrategy = rs
	return blder
}

// WithLoadBalancing loadBalancing specification to be used when generating endpoints for the target resource if set.
func (blder *EndpointsBuilder) WithLoadBalancing(lb *v1alpha1.LoadBalancingSpec) *EndpointsBuilder {
	blder.loadBalancing = lb
	return blder
}

// WithEndpoint add an endpoint to the list of endpoints that will be returned by the builder.
func (blder *EndpointsBuilder) WithEndpoint(ep *externaldns.Endpoint) *EndpointsBuilder {
	blder.endpoints = append(blder.endpoints, ep)
	return blder
}

// Build builds and returns the endpoint array using the given inputs to the builder.
// Can optionally do validation of the endpoints and return an error if needs be.
func (blder *EndpointsBuilder) Build() ([]*externaldns.Endpoint, error) {
	if blder.target != nil {
		if blder.loadBalancing != nil {
			// get loadbalanced endpoints
			blder.endpoints = append(blder.endpoints, getLoadBalancedEndpoints(*blder.target, *blder.loadBalancing)...)
		} else {
			// get simple endpoints
			blder.endpoints = append(blder.endpoints, getSimpleEndpoints(*blder.target)...)
		}
	}
	return blder.endpoints, nil
}

// getSimpleEndpoints returns the endpoints for the given Target using the simple routing strategy
func getSimpleEndpoints(_ Target) []*externaldns.Endpoint {
	return []*externaldns.Endpoint{}
}

// // getLoadBalancedEndpoints returns the endpoints for the given Target using the loadbalanced routing strategy
func getLoadBalancedEndpoints(_ Target, _ v1alpha1.LoadBalancingSpec) []*externaldns.Endpoint {
	return []*externaldns.Endpoint{}
}
