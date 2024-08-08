//go:build unit

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/external-dns/endpoint"
)

const (
	IPAddressOne = "127.0.0.1"
	IPAddressTwo = "127.0.0.2"
	TestHostname = "pat.the.cat"
)

var (
	TestListener string
	TestRouting  *Routing
	TestLabels   map[string]string

	TestNamespacedName = types.NamespacedName{
		Name:      "TestName",
		Namespace: "TestNamespace",
	}

	domain      = "example.com"
	clusterHash = "2q5hyv"
	gwHash      = "a8xcra"
	defaultGeo  = "IE"
	clusterID   = "fbf71c44-6b37-4962-ace6-801912e769be"
)

var _ = Describe("DnsrecordEndpoints", func() {
	BeforeEach(func() {
		// reset
		TestRouting = &Routing{}
		TestLabels = map[string]string{}
	})
	Context("Success scenarios", func() {
		Context("Simple routing Strategy", func() {
			BeforeEach(func() {
				TestRouting, _ = NewRoutingBuilder().WithAddresses(map[string]string{
					IPAddressOne: IPAddressType,
					IPAddressTwo: IPAddressType,
				}).Build()
			})
			It("Should generate endpoint", func() {
				TestListener = HostOne(domain)
				endpoints, err := GenerateEndpoints(TestNamespacedName, nil, TestListener, TestRouting)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoints).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"DNSName":       Equal(HostOne(domain)),
						"Targets":       ContainElements(IPAddressOne, IPAddressTwo),
						"RecordType":    Equal("A"),
						"SetIdentifier": Equal(""),
						"RecordTTL":     Equal(endpoint.TTL(60)),
					})),
				))
			})
			It("Should generate wildcard endpoint", func() {
				TestListener := HostWildcard(domain)
				endpoints, err := GenerateEndpoints(TestNamespacedName, nil, TestListener, TestRouting)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoints).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"DNSName":       Equal(HostWildcard(domain)),
						"Targets":       ContainElements(IPAddressOne, IPAddressTwo),
						"RecordType":    Equal("A"),
						"SetIdentifier": Equal(""),
						"RecordTTL":     Equal(endpoint.TTL(60)),
					})),
				))
			})
			It("Should generate hostname endpoint", func() {
				TestRouting, _ = NewRoutingBuilder().WithAddresses(map[string]string{
					TestHostname: HostnameAddressType,
				}).Build()
				TestListener = HostOne(domain)
				endpoints, err := GenerateEndpoints(TestNamespacedName, nil, TestListener, TestRouting)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoints).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"DNSName":       Equal(HostOne(domain)),
						"Targets":       ContainElement(TestHostname),
						"RecordType":    Equal("CNAME"),
						"SetIdentifier": Equal(""),
						"RecordTTL":     Equal(endpoint.TTL(60)),
					})),
				))

			})
			It("Should return no endpoints if missing addresses", func() {
				TestRouting.Addresses = map[string]string{}
				endpoints, err := GenerateEndpoints(TestNamespacedName, nil, TestListener, TestRouting)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoints).To(BeEmpty())
			})
		})
		Context("Load-balanced routing strategy", func() {
			BeforeEach(func() {
				TestRouting, _ = NewRoutingBuilder().WithAddresses(map[string]string{
					IPAddressOne: IPAddressType,
					IPAddressTwo: IPAddressType,
				}).WithLoadBalancing(clusterID, defaultGeo, 120).Build()
				TestLabels[LabelLBAttributeGeoCode] = defaultGeo
			})
			Context("With matching geo", func() {
				It("Should generate endpoints", func() {
					TestListener = HostOne(domain)
					endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
					Expect(err).NotTo(HaveOccurred())
					Expect(EndpointsTraversable(endpoints, HostOne(domain), []string{IPAddressOne, IPAddressTwo})).To(BeTrue())
					Expect(endpoints).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(clusterHash + "-" + gwHash + "." + "klb.test." + domain),
							"Targets":       ConsistOf(IPAddressOne, IPAddressTwo),
							"RecordType":    Equal("A"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(60)),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("ie.klb.test." + domain),
							"Targets":          ConsistOf(clusterHash + "-" + gwHash + "." + "klb.test." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal(clusterHash + "-" + gwHash + "." + "klb.test." + domain),
							"RecordTTL":        Equal(endpoint.TTL(60)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "weight", Value: "120"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("klb.test." + domain),
							"Targets":          ConsistOf("ie.klb.test." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal(defaultGeo),
							"RecordTTL":        Equal(endpoint.TTL(300)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "geo-code", Value: defaultGeo}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("klb.test." + domain),
							"Targets":          ConsistOf("ie.klb.test." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal("default"),
							"RecordTTL":        Equal(endpoint.TTL(300)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "geo-code", Value: "*"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(HostOne(domain)),
							"Targets":       ConsistOf("klb.test." + domain),
							"RecordType":    Equal("CNAME"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(300)),
						})),
					))

				})
				It("Should generate wildcard endpoints", func() {
					TestListener = HostWildcard(domain)
					endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
					Expect(err).NotTo(HaveOccurred())
					Expect(EndpointsTraversable(endpoints, HostWildcard(domain), []string{IPAddressOne, IPAddressTwo})).To(BeTrue())
					Expect(endpoints).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(clusterHash + "-" + gwHash + "." + "klb." + domain),
							"Targets":       ConsistOf(IPAddressOne, IPAddressTwo),
							"RecordType":    Equal("A"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(60)),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("ie.klb." + domain),
							"Targets":          ConsistOf(clusterHash + "-" + gwHash + "." + "klb." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal(clusterHash + "-" + gwHash + "." + "klb." + domain),
							"RecordTTL":        Equal(endpoint.TTL(60)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "weight", Value: "120"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("klb." + domain),
							"Targets":          ConsistOf("ie.klb." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal("default"),
							"RecordTTL":        Equal(endpoint.TTL(300)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "geo-code", Value: "*"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("klb." + domain),
							"Targets":          ConsistOf("ie.klb." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal(defaultGeo),
							"RecordTTL":        Equal(endpoint.TTL(300)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "geo-code", Value: defaultGeo}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(HostWildcard(domain)),
							"Targets":       ConsistOf("klb." + domain),
							"RecordType":    Equal("CNAME"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(300)),
						})),
					))
				})
			})

			Context("Load-balanced routing strategy with non-matching geo", func() {
				BeforeEach(func() {
					TestLabels[LabelLBAttributeGeoCode] = "CAD"
				})
				It("Should generate endpoints", func() {
					TestListener = HostOne(domain)
					endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
					Expect(err).NotTo(HaveOccurred())
					Expect(EndpointsTraversable(endpoints, HostOne(domain), []string{IPAddressOne, IPAddressTwo})).To(BeTrue())
					Expect(endpoints).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(clusterHash + "-" + gwHash + "." + "klb.test." + domain),
							"Targets":       ConsistOf(IPAddressOne, IPAddressTwo),
							"RecordType":    Equal("A"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(60)),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("cad.klb.test." + domain),
							"Targets":          ConsistOf(clusterHash + "-" + gwHash + "." + "klb.test." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal(clusterHash + "-" + gwHash + "." + "klb.test." + domain),
							"RecordTTL":        Equal(endpoint.TTL(60)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "weight", Value: "120"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("klb.test." + domain),
							"Targets":          ConsistOf("cad.klb.test." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal("CAD"),
							"RecordTTL":        Equal(endpoint.TTL(300)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "geo-code", Value: "CAD"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(HostOne(domain)),
							"Targets":       ConsistOf("klb.test." + domain),
							"RecordType":    Equal("CNAME"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(300)),
						})),
					))

				})
				It("Should generate wildcard endpoints", func() {
					TestListener = HostWildcard(domain)
					endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
					Expect(err).NotTo(HaveOccurred())
					Expect(EndpointsTraversable(endpoints, HostWildcard(domain), []string{IPAddressOne, IPAddressTwo})).To(BeTrue())
					Expect(endpoints).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(clusterHash + "-" + gwHash + "." + "klb." + domain),
							"Targets":       ConsistOf(IPAddressOne, IPAddressTwo),
							"RecordType":    Equal("A"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(60)),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("cad.klb." + domain),
							"Targets":          ConsistOf(clusterHash + "-" + gwHash + "." + "klb." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal(clusterHash + "-" + gwHash + "." + "klb." + domain),
							"RecordTTL":        Equal(endpoint.TTL(60)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "weight", Value: "120"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("klb." + domain),
							"Targets":          ConsistOf("cad.klb." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal("CAD"),
							"RecordTTL":        Equal(endpoint.TTL(300)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "geo-code", Value: "CAD"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(HostWildcard(domain)),
							"Targets":       ConsistOf("klb." + domain),
							"RecordType":    Equal("CNAME"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(300)),
						})),
					))
				})
			})

			Context("Load-balanced routing strategy with custom weights", func() {
				BeforeEach(func() {
					TestRouting, _ = NewRoutingBuilder().WithAddresses(map[string]string{
						IPAddressOne: IPAddressType,
						IPAddressTwo: IPAddressType,
					}).WithLoadBalancing(clusterID, defaultGeo, 120).
						WithCustomWeights([]CustomWeight{
							{
								Selector: metav1.LabelSelector{
									MatchLabels: map[string]string{
										"kuadrant.io/my-custom-weight-attr": "FOO",
									},
								},
								Weight: 100,
							},
						}).Build()
					TestLabels["kuadrant.io/my-custom-weight-attr"] = "FOO"

				})
				It("Should generate endpoints", func() {
					TestListener = HostOne(domain)
					endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
					Expect(err).NotTo(HaveOccurred())
					Expect(EndpointsTraversable(endpoints, HostOne(domain), []string{IPAddressOne, IPAddressTwo})).To(BeTrue())
					Expect(endpoints).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(clusterHash + "-" + gwHash + "." + "klb.test." + domain),
							"Targets":       ConsistOf(IPAddressOne, IPAddressTwo),
							"RecordType":    Equal("A"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(60)),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("ie.klb.test." + domain),
							"Targets":          ConsistOf(clusterHash + "-" + gwHash + "." + "klb.test." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal(clusterHash + "-" + gwHash + "." + "klb.test." + domain),
							"RecordTTL":        Equal(endpoint.TTL(60)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "weight", Value: "100"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("klb.test." + domain),
							"Targets":          ConsistOf("ie.klb.test." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal(defaultGeo),
							"RecordTTL":        Equal(endpoint.TTL(300)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "geo-code", Value: defaultGeo}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("klb.test." + domain),
							"Targets":          ConsistOf("ie.klb.test." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal("default"),
							"RecordTTL":        Equal(endpoint.TTL(300)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "geo-code", Value: "*"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(HostOne(domain)),
							"Targets":       ConsistOf("klb.test." + domain),
							"RecordType":    Equal("CNAME"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(300)),
						})),
					))

				})
				It("Should generate wildcard endpoints", func() {
					TestListener = HostWildcard(domain)
					endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
					Expect(err).NotTo(HaveOccurred())
					Expect(EndpointsTraversable(endpoints, HostWildcard(domain), []string{IPAddressOne, IPAddressTwo})).To(BeTrue())
					Expect(endpoints).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(clusterHash + "-" + gwHash + "." + "klb." + domain),
							"Targets":       ConsistOf(IPAddressOne, IPAddressTwo),
							"RecordType":    Equal("A"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(60)),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("ie.klb." + domain),
							"Targets":          ConsistOf(clusterHash + "-" + gwHash + "." + "klb." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal(clusterHash + "-" + gwHash + "." + "klb." + domain),
							"RecordTTL":        Equal(endpoint.TTL(60)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "weight", Value: "100"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("klb." + domain),
							"Targets":          ConsistOf("ie.klb." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal("default"),
							"RecordTTL":        Equal(endpoint.TTL(300)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "geo-code", Value: "*"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("klb." + domain),
							"Targets":          ConsistOf("ie.klb." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal(defaultGeo),
							"RecordTTL":        Equal(endpoint.TTL(300)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "geo-code", Value: defaultGeo}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(HostWildcard(domain)),
							"Targets":       ConsistOf("klb." + domain),
							"RecordType":    Equal("CNAME"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(300)),
						})),
					))
				})

			})

			Context("With missing geo label on Gateway and hostname address", func() {
				BeforeEach(func() {
					TestRouting, _ = NewRoutingBuilder().WithAddresses(map[string]string{
						TestHostname: HostnameAddressType,
					}).WithLoadBalancing(clusterID, defaultGeo, 120).Build()
					TestLabels = map[string]string{}
				})

				It("Should generate endpoints", func() {
					TestListener = HostOne(domain)
					endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
					Expect(err).NotTo(HaveOccurred())
					Expect(EndpointsTraversable(endpoints, HostOne(domain), []string{TestHostname})).To(BeTrue())
					Expect(endpoints).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("default.klb.test." + domain),
							"Targets":          ConsistOf(TestHostname),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal(TestHostname),
							"RecordTTL":        Equal(endpoint.TTL(60)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "weight", Value: "120"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("klb.test." + domain),
							"Targets":          ConsistOf("default.klb.test." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal("default"),
							"RecordTTL":        Equal(endpoint.TTL(300)),
							"ProviderSpecific": BeNil(),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(HostOne(domain)),
							"Targets":       ConsistOf("klb.test." + domain),
							"RecordType":    Equal("CNAME"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(300)),
						})),
					))

				})
				It("Should generate wildcard endpoints", func() {
					TestListener = HostWildcard(domain)
					endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
					Expect(err).NotTo(HaveOccurred())
					Expect(EndpointsTraversable(endpoints, HostWildcard(domain), []string{TestHostname})).To(BeTrue())
					Expect(endpoints).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("default.klb." + domain),
							"Targets":          ConsistOf(TestHostname),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal(TestHostname),
							"RecordTTL":        Equal(endpoint.TTL(60)),
							"ProviderSpecific": Equal(endpoint.ProviderSpecific{{Name: "weight", Value: "120"}}),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":          Equal("klb." + domain),
							"Targets":          ConsistOf("default.klb." + domain),
							"RecordType":       Equal("CNAME"),
							"SetIdentifier":    Equal("default"),
							"RecordTTL":        Equal(endpoint.TTL(300)),
							"ProviderSpecific": BeNil(),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"DNSName":       Equal(HostWildcard(domain)),
							"Targets":       ConsistOf("klb." + domain),
							"RecordType":    Equal("CNAME"),
							"SetIdentifier": Equal(""),
							"RecordTTL":     Equal(endpoint.TTL(300)),
						})),
					))
				})
			})
		})
	})

	Context("Failure scenarios", func() {
		BeforeEach(func() {
			// create valid set of inputs for lb strategy with custom weights.
			TestRouting, _ = NewRoutingBuilder().WithAddresses(map[string]string{
				IPAddressOne: IPAddressType,
				IPAddressTwo: IPAddressType,
			}).WithLoadBalancing(clusterID, defaultGeo, 120).
				WithCustomWeights([]CustomWeight{
					{
						Selector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"kuadrant.io/my-custom-weight-attr": "FOO",
							},
						},
						Weight: 100,
					},
				}).Build()
			TestLabels["kuadrant.io/my-custom-weight-attr"] = "FOO"
			TestListener = HostOne(domain)
		})
		It("Should not accept unknown strategy", func() {
			// if routing created manually or error in builder was ignored
			TestRouting.Strategy = "cat"
			endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
			Expect(endpoints).To(BeNil())
			Expect(err.Error()).To(ContainSubstring(ErrUnknownRoutingStrategy.Error()))
		})
		It("Should not allow for an empty listener", func() {
			TestListener = ""
			endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
			Expect(endpoints).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("listener hostname is empty"))
		})
		It("Should not allow for nil object labels", func() {
			endpoints, err := GenerateEndpoints(TestNamespacedName, nil, TestListener, TestRouting)
			Expect(endpoints).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("object labels required"))
		})
		Context("Should not allow for invalid routing", func() {
			It("with missing cluster id", func() {
				// if routing created manually or error in builder was ignored
				TestRouting.ClusterID = ""
				endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
				Expect(endpoints).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("cluster ID is required"))
			})
			It("with missing addresses", func() {
				// if routing created manually or error in builder was ignored
				TestRouting.Addresses = nil
				endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
				Expect(endpoints).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("must provide addresses"))
			})
			It("with missing or zero default weight", func() {
				// if routing created manually or error in builder was ignored
				TestRouting.DefaultWeight = 0
				endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
				Expect(endpoints).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("default weight is required"))
			})
			It("with missing default geo", func() {
				// if routing created manually or error in builder was ignored
				TestRouting.DefaultGeoCode = ""
				endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
				Expect(endpoints).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("default geocode is required"))
			})
			It("with zero custom weight", func() {
				TestRouting.CustomWeights[0].Weight = 0
				endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
				Expect(endpoints).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("custom weight cannot be zero"))
			})
			It("with missing selector on custom weight", func() {
				TestRouting.CustomWeights[0].Selector = metav1.LabelSelector{}
				endpoints, err := GenerateEndpoints(TestNamespacedName, TestLabels, TestListener, TestRouting)
				Expect(endpoints).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("custom weight must define non-empty selector"))
			})
		})

	})
})
