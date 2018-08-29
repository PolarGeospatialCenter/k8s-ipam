package v1alpha1

import (
	"math/rand"
	"net"

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
	Pool               PoolNet          `json:"pool"`
	Gateway            net.IP           `json:"gateway"`
	StaticReservations IPReservationMap `json:"staticReservations"`
}

type IPPoolStatus struct {
	DynamicReservations IPReservationMap
}

type PoolNet net.IPNet

func (in *PoolNet) DeepCopyInto(out *PoolNet) {
	*out = *in
}

func (in *PoolNet) DeepCopy() *PoolNet {
	out := *in
	return &out
}

func (s *IPPoolSpec) Network() *net.IPNet {
	network := net.IPNet(s.Pool)
	return &network
}

// GetExistingReservation checks if a reservation for this pod exists, if so return the IP
func (p *IPPool) GetExistingReservation(namespace, podName string) *net.IP {
	if staticIP := p.Spec.StaticReservations.GetExistingReservation(namespace, podName); staticIP != nil {
		return staticIP
	}

	return p.Status.DynamicReservations.GetExistingReservation(namespace, podName)
}

func (p *IPPool) RandomIP() net.IP {
	ones, bits := p.Spec.Pool.Mask.Size()
	hostBits := bits - ones

	randomBits := rand.Uint64()
	randIp, _ := iputils.SetBits(p.Spec.Pool.IP, randomBits, uint(ones), uint(hostBits))
	return randIp
}

func (p *IPPool) Gateway() net.IP {
	return p.Spec.Gateway
}

// AlreadyReserved checks the pool to see if the IP is reserved by any pod.  Returns false if IP is not contained in the pool.
func (p *IPPool) AlreadyReserved(ip net.IP) bool {
	if !p.Spec.Network().Contains(ip) {
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
	if !p.Spec.Network().Contains(ip) {
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
