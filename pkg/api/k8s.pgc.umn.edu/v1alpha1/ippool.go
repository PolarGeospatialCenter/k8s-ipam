package v1alpha1

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/azenk/iputils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type IPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []IPPool `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type IPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              IPPoolSpec   `json:"spec"`
	Status            IPPoolStatus `json:"status,omitempty"`
}

type IPPoolSpec struct {
	Range              IPRange          `json:"range"`
	NetmaskBits        int              `json:"netmaskBits"`
	Gateway            net.IP           `json:"gateway"`
	StaticReservations IPReservationMap `json:"staticReservations"`
}

type IPPoolStatus struct {
	DynamicReservations IPReservationMap
}

// GetMask returns the netmask for ips allocated in this range
func (s *IPPoolSpec) GetMask() net.IPMask {
	bits := s.Range.IPSizeBits()
	return net.CIDRMask(s.NetmaskBits, bits)
}

type IPRange string

// AsNet returns the range as a net.IPNet struct.  *Any parse errors are silently ignored.*
func (r IPRange) AsNet() *net.IPNet {
	_, network, _ := net.ParseCIDR(string(r))
	return network
}

// IPSizeBits returns the number of bits required for IPs in this range
func (r IPRange) IPSizeBits() int {
	_, bits := r.AsNet().Mask.Size()
	return bits
}

// RangeMaskBits returns the number of bits in the pre-allocated portion of this IPRange
func (r IPRange) RangeMaskBits() int {
	ones, _ := r.AsNet().Mask.Size()
	return ones
}

// Validate Returns nil if IPRange can be parsed
func (r IPRange) Validate() error {
	_, _, err := net.ParseCIDR(string(r))
	return err
}

// RangeContains returns true if ip is within the range allocated from this pool
func (p IPPool) RangeContains(ip net.IP) bool {
	return p.Spec.Range.AsNet().Contains(ip)
}

// GetExistingReservation checks if a reservation for this pod exists, if so return the IP
func (p *IPPool) GetExistingReservation(namespace, podName string) *net.IP {
	if p.Spec.StaticReservations != nil {
		if staticIP := p.Spec.StaticReservations.GetExistingReservation(namespace, podName); staticIP != nil {
			return staticIP
		}
	}

	if p.Status.DynamicReservations == nil {
		return nil
	}
	return p.Status.DynamicReservations.GetExistingReservation(namespace, podName)
}

func (p *IPPool) RandomIP() net.IP {
	rand.Seed(time.Now().UnixNano())
	allocationRange := p.Spec.Range.AsNet()
	ones, bits := allocationRange.Mask.Size()
	hostBits := bits - ones

	randomBits := rand.Uint64()
	randIp, _ := iputils.SetBits(allocationRange.IP, randomBits, uint(ones), uint(hostBits))
	return randIp
}

func (p *IPPool) Gateway() net.IP {
	return p.Spec.Gateway
}

// AlreadyReserved checks the pool to see if the IP is reserved by any pod.  Returns false if IP is not contained in the pool.
func (p *IPPool) AlreadyReserved(ip net.IP) bool {
	if !p.RangeContains(ip) {
		return false
	}

	if p.Spec.Gateway.Equal(ip) {
		return true
	}

	_, _, reserved := p.GetPodForIP(ip)

	return reserved
}

// GetPodForIP returns the namespace and pod name for the pod associated with a reservation.  found is set to false if no pod is found.
func (p *IPPool) GetPodForIP(ip net.IP) (namespace, podName string, found bool) {
	if !p.RangeContains(ip) {
		return "", "", false
	}

	if p.Spec.Gateway.Equal(ip) {
		return "", "", false
	}

	if p.Spec.StaticReservations != nil {
		namespace, podName, found := p.Spec.StaticReservations.GetPodForIP(ip)
		if found {
			return namespace, podName, true
		}
	}

	if p.Status.DynamicReservations != nil {
		namespace, podName, found := p.Status.DynamicReservations.GetPodForIP(ip)
		if found {
			return namespace, podName, true
		}
	}

	return "", "", false
}

func (p *IPPool) Reserve(namespace, podName string, ip net.IP) {
	if p.Status.DynamicReservations == nil {
		p.Status.DynamicReservations = NewIPReservationMap()
	}
	p.Status.DynamicReservations.Reserve(namespace, podName, ip)
}

// FreeDynamicPodReservation removes any existing dynamic reservations for a given pod
func (p *IPPool) FreeDynamicPodReservation(namespace, podName string) {
	if p.Status.DynamicReservations == nil {
		return
	}

	p.Status.DynamicReservations.FreePodReservation(namespace, podName)
}

// Validate returns nil if there are no obvious errors in IP Pool configuration
func (s IPPoolSpec) Validate() error {
	// Range is valid
	if err := s.Range.Validate(); err != nil {
		return fmt.Errorf("IP range is invalid (%v), please check your syntax: %v", s.Range, err)
	}

	// NetmaskBits are valid and less than or equal to Range Bits
	if s.NetmaskBits < 0 || s.NetmaskBits > s.Range.IPSizeBits() {
		return fmt.Errorf("specified netmask is invalid")
	}

	if s.NetmaskBits > s.Range.RangeMaskBits() {
		return fmt.Errorf("specified netmask doesn't completely contain the Range.  Please adjust.")
	}

	// Gateway must be within specified network
	containingNetwork := net.IPNet{
		IP:   s.Range.AsNet().IP,
		Mask: net.CIDRMask(s.NetmaskBits, s.Range.IPSizeBits()),
	}
	if s.Gateway != nil && !containingNetwork.Contains(s.Gateway) {
		return fmt.Errorf("Gateway must be on the subnet that includes this range.")
	}

	return nil
}

type IPReservationMap map[string]map[string]net.IP

func NewIPReservationMap() IPReservationMap {
	return make(map[string]map[string]net.IP)
}

func (m IPReservationMap) GetExistingReservation(namespace, podName string) *net.IP {
	if namespaceMap, nsFound := m[namespace]; nsFound {
		if podIp, podFound := namespaceMap[podName]; podFound {
			return &podIp
		}
	}
	return nil
}

func (m IPReservationMap) GetPodForIP(ip net.IP) (namespace, podName string, found bool) {
	for namespace, nsMap := range m {
		for podName, podIp := range nsMap {
			if podIp.Equal(ip) {
				return namespace, podName, true
			}
		}
	}
	return "", "", false
}

func (m IPReservationMap) Reserve(namespace, podName string, ip net.IP) {
	if _, ok := m[namespace]; !ok {
		m[namespace] = make(map[string]net.IP, 0)
	}
	m[namespace][podName] = ip
}

func (m IPReservationMap) AlreadyReserved(ip net.IP) bool {
	_, _, found := m.GetPodForIP(ip)
	return found
}

func (m IPReservationMap) FreePodReservation(namespace, podName string) {
	if _, nsFound := m[namespace]; nsFound {
		if _, podFound := m[namespace][podName]; podFound {
			delete(m[namespace], podName)
		}

		if len(m[namespace]) == 0 {
			delete(m, namespace)
		}
	}
}
