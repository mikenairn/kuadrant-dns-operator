package v1alpha1

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	externaldns "sigs.k8s.io/external-dns/endpoint"

	"github.com/kuadrant/dns-operator/internal/common/hash"
)

const (
	SimpleRoutingStrategy       RoutingStrategy = "simple"
	LoadBalancedRoutingStrategy RoutingStrategy = "loadbalanced"

	IPAddressType       = "IPAddress"
	HostnameAddressType = "Hostname"

	DefaultTTL      = 60
	DefaultCnameTTL = 300

	ClusterIDLength = 6

	LabelLBAttributeGeoCode = "kuadrant.io/lb-attribute-geo-code"
)

var (
	ErrUnknownRoutingStrategy = fmt.Errorf("unknown routing strategy")
)

// RoutingStrategy specifies a strategy to be used: simple or load-balanced
// +kubebuilder:validation:Enum=simple;loadbalanced
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="RoutingStrategy is immutable"
// +kubebuilder:default=loadbalanced
type RoutingStrategy string

type CustomWeight struct {
	Weight   int
	Selector v1.LabelSelector
}

// Routing holds all necessary information to generate endpoints
type Routing struct {
	Addresses      map[string]string
	Strategy       RoutingStrategy
	DefaultGeoCode string
	DefaultWeight  int
	CustomWeights  []CustomWeight
	ClusterID      string
}

type RoutingBuilder struct {
	*Routing
}

func NewRoutingBuilder() *RoutingBuilder {
	return &RoutingBuilder{
		Routing: &Routing{},
	}
}

func (rb *RoutingBuilder) WithAddresses(addresses map[string]string) *RoutingBuilder {
	// if strategy already set by WithLoadBalancing not override it
	if rb.Strategy == "" {
		rb.Strategy = SimpleRoutingStrategy
	}
	rb.Addresses = addresses
	return rb
}

func (rb *RoutingBuilder) WithLoadBalancing(clusterID, defaultGeo string, defaultWeight int) *RoutingBuilder {
	rb.Strategy = LoadBalancedRoutingStrategy
	rb.ClusterID = clusterID
	rb.DefaultGeoCode = defaultGeo
	rb.DefaultWeight = defaultWeight
	return rb
}

func (rb *RoutingBuilder) WithCustomWeights(weights []CustomWeight) *RoutingBuilder {
	rb.CustomWeights = weights
	return rb
}

func (rb *RoutingBuilder) Build() (*Routing, error) {
	return rb.Routing, rb.Validate()
}

func GenerateEndpoints(namespacedName types.NamespacedName, objectLabels map[string]string, hostname string, routing *Routing) ([]*externaldns.Endpoint, error) {
	if hostname == "" {
		return nil, fmt.Errorf("listener hostname is empty")
	}

	var endpoints []*externaldns.Endpoint

	if err := routing.Validate(); err != nil {
		return nil, err
	}

	switch routing.Strategy {
	case SimpleRoutingStrategy:
		endpoints = getSimpleEndpoints(routing.Addresses, hostname)
	case LoadBalancedRoutingStrategy:
		if objectLabels == nil {
			return nil, fmt.Errorf("object labels required")
		}
		endpoints = getLoadBalancedEndpoints(namespacedName, objectLabels, routing, hostname)
	default:
		return nil, fmt.Errorf("%w : %s", ErrUnknownRoutingStrategy, routing.Strategy)
	}

	sort.Slice(endpoints, func(i, j int) bool {
		return getSetID(endpoints[i]) < getSetID(endpoints[j])
	})

	return endpoints, nil
}

// getSimpleEndpoints returns the endpoints for the given GatewayTarget using the simple routing strategy
func getSimpleEndpoints(addresses map[string]string, hostname string) []*externaldns.Endpoint {
	var endpoints []*externaldns.Endpoint

	ipValues, hostValues := targetsFromAddresses(addresses)

	if len(ipValues) > 0 {
		endpoint := createEndpoint(hostname, ipValues, ARecordType, "", DefaultTTL)
		endpoints = append(endpoints, endpoint)
	}

	if len(hostValues) > 0 {
		endpoint := createEndpoint(hostname, hostValues, CNAMERecordType, "", DefaultTTL)
		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}

// getLoadBalancedEndpoints returns the endpoints for the given Gateway using the loadbalanced routing strategy
//
// Builds an array of externaldns.Endpoint resources. The endpoints expected are calculated using the Gateway
//and the Routing.
//
// A CNAME record is created for the target host (DNSRecord.name), pointing to a generated gateway lb host.
// A CNAME record for the gateway lb host is created with appropriate Geo information from Gateway
// A CNAME record for the geo specific host is created with weight information for that target added,
// pointing to a target cluster hostname.
// An A record for the target cluster hostname is created for any IP targets retrieved for that cluster.
//
// Example(Weighted only)
//
// www.example.com CNAME lb-1ab1.www.example.com
// lb-1ab1.www.example.com CNAME geolocation * default.lb-1ab1.www.example.com
// default.lb-1ab1.www.example.com CNAME weighted 100 1bc1.lb-1ab1.www.example.com
// default.lb-1ab1.www.example.com CNAME weighted 100 aws.lb.com
// 1bc1.lb-1ab1.www.example.com A 192.22.2.1
//
// Example(Geo, default IE)
//
// shop.example.com CNAME lb-a1b2.shop.example.com
// lb-a1b2.shop.example.com CNAME geolocation ireland ie.lb-a1b2.shop.example.com
// lb-a1b2.shop.example.com geolocation default ie.lb-a1b2.shop.example.com (set by the default geo option)
// ie.lb-a1b2.shop.example.com CNAME weighted 100 ab1.lb-a1b2.shop.example.com
// ie.lb-a1b2.shop.example.com CNAME weighted 100 aws.lb.com
// ab1.lb-a1b2.shop.example.com A 192.22.2.1 192.22.2.2

func getLoadBalancedEndpoints(namespacedName types.NamespacedName, objectLabels map[string]string, routing *Routing, hostname string) []*externaldns.Endpoint {
	cnameHost := hostname
	if isWildCardHost(hostname) {
		cnameHost = strings.Replace(hostname, "*.", "", -1)
	}

	var endpoint *externaldns.Endpoint
	endpoints := make([]*externaldns.Endpoint, 0)

	lbName := strings.ToLower(fmt.Sprintf("klb.%s", cnameHost))
	geoCode := getGeoFromLabel(objectLabels)
	geoLbName := strings.ToLower(fmt.Sprintf("%s.%s", geoCode, lbName))

	ipValues, hostValues := targetsFromAddresses(routing.Addresses)

	if len(ipValues) > 0 {
		clusterLbName := strings.ToLower(fmt.Sprintf("%s-%s.%s", getShortCode(routing.ClusterID), getShortCode(fmt.Sprintf("%s-%s", namespacedName.Name, namespacedName.Namespace)), lbName))
		endpoint = createEndpoint(clusterLbName, ipValues, ARecordType, "", DefaultTTL)
		endpoints = append(endpoints, endpoint)
		hostValues = append(hostValues, clusterLbName)
	}

	for _, hostValue := range hostValues {
		endpoint = createEndpoint(geoLbName, []string{hostValue}, CNAMERecordType, hostValue, DefaultTTL)
		endpoint.SetProviderSpecificProperty(ProviderSpecificWeight, strconv.Itoa(routing.getWeight(objectLabels)))
		endpoints = append(endpoints, endpoint)
	}

	// nothing to do
	if len(endpoints) == 0 {
		return endpoints
	}

	//Create lbName CNAME (lb-a1b2.shop.example.com -> <geoCode>.lb-a1b2.shop.example.com)
	endpoint = createEndpoint(lbName, []string{geoLbName}, CNAMERecordType, geoCode, DefaultCnameTTL)
	// don't set provider specific if gateway is missing the label
	if geoCode != DefaultGeo {
		endpoint.SetProviderSpecificProperty(ProviderSpecificGeoCode, geoCode)
	}
	endpoints = append(endpoints, endpoint)

	//Add a default geo (*) endpoint if the current geoCode is equal to the defaultGeo set in the policy spec
	//default geo is the default geo from spec
	if geoCode == routing.DefaultGeoCode {
		endpoint = createEndpoint(lbName, []string{geoLbName}, CNAMERecordType, "default", DefaultCnameTTL)
		endpoint.SetProviderSpecificProperty(ProviderSpecificGeoCode, WildcardGeo)
		endpoints = append(endpoints, endpoint)
	}

	if len(endpoints) > 0 {
		//Create gwListenerHost CNAME (shop.example.com -> lb-a1b2.shop.example.com)
		endpoint = createEndpoint(hostname, []string{lbName}, CNAMERecordType, "", DefaultCnameTTL)
		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}

func createEndpoint(dnsName string, targets externaldns.Targets, recordType DNSRecordType, setIdentifier string,
	recordTTL externaldns.TTL) (endpoint *externaldns.Endpoint) {
	return &externaldns.Endpoint{
		DNSName:       dnsName,
		Targets:       targets,
		RecordType:    string(recordType),
		SetIdentifier: setIdentifier,
		RecordTTL:     recordTTL,
	}
}

func getSetID(endpoint *externaldns.Endpoint) string {
	return endpoint.DNSName + endpoint.SetIdentifier
}

func isWildCardHost(host string) bool {
	return strings.HasPrefix(host, "*")
}

func getShortCode(name string) string {
	return hash.ToBase36HashLen(name, ClusterIDLength)
}

func getGeoFromLabel(objectLabels map[string]string) string {
	if geoCode, found := objectLabels[LabelLBAttributeGeoCode]; found {
		return geoCode
	}
	return DefaultGeo
}

func targetsFromAddresses(addresses map[string]string) ([]string, []string) {
	var ipValues []string
	var hostValues []string

	for key, value := range addresses {
		if value == "IPAddress" {
			ipValues = append(ipValues, key)
		} else {
			hostValues = append(hostValues, key)
		}
	}

	return ipValues, hostValues
}

func (r *Routing) getWeight(objectLabels map[string]string) int {
	weight := r.DefaultWeight
	for _, customWeight := range r.CustomWeights {
		selector, err := v1.LabelSelectorAsSelector(&customWeight.Selector)
		if err != nil {
			return weight
		}
		if selector.Matches(labels.Set(objectLabels)) {
			weight = customWeight.Weight
			break
		}
	}
	return weight
}

func (r *Routing) Validate() error {
	// we don't care about routing for the simple strategy
	if r.Strategy == SimpleRoutingStrategy {
		return nil
	}

	if r.Strategy == "" || r.Addresses == nil {
		return fmt.Errorf("must provide addresses")
	}

	// clusterID must not be an empty string
	if r.ClusterID == "" {
		return fmt.Errorf("cluster ID is required")
	}

	// default weight and geo are required
	if r.DefaultWeight == 0 {
		return fmt.Errorf("default weight is required")
	}
	if r.DefaultGeoCode == "" {
		return fmt.Errorf("default geocode is required")
	}

	// validate custom weights if they were provided
	if r.CustomWeights != nil {
		for _, customWeight := range r.CustomWeights {
			if customWeight.Weight == 0 {
				return fmt.Errorf("custom weight cannot be zero")
			}
			if customWeight.Selector.MatchLabels == nil && len(customWeight.Selector.MatchLabels) == 0 && customWeight.Selector.MatchExpressions == nil {
				return fmt.Errorf("custom weight must define non-empty selector")
			}
		}
	}
	return nil
}
