package v1alpha1

import (
	"fmt"
	"net"
	"testing"
)

func TestIPReservationMap(t *testing.T) {
	m := IPReservationMap{}
	ip := net.ParseIP("10.0.0.1")
	if m.AlreadyReserved(ip) {
		t.Errorf("Empty map claims IP is reserved")
	}

	m.Reserve("foo", "bar", ip)

	if !m.AlreadyReserved(ip) {
		t.Errorf("Map claims reserved IP is available")
	}

	existingIP := m.GetExistingReservation("foo", "bar")

	if existingIP == nil || !existingIP.Equal(ip) {
		t.Errorf("Wrong or no ip returned for existing reservation")
	}

	m.FreePodReservation("foo", "bar")

	if m.AlreadyReserved(ip) {
		t.Errorf("Empty map claims IP is reserved")
	}

	if existingIP := m.GetExistingReservation("foo", "bar"); existingIP != nil {
		t.Errorf("IP found for pod after Free was called")
	}

}

func TestIPPoolGetExistingReservation(t *testing.T) {
	p := IPPool{}
	p.Spec.Range = IPRange{"2001:db8::/65"}
	p.Spec.NetmaskBits = 64

	if existingIP := p.GetExistingReservation("foo", "bar"); existingIP != nil {
		t.Errorf("IP returned for unreserved address: %v", existingIP)
	}

	podIP := net.ParseIP("2001:db8::ff32")
	p.Reserve("foo", "bar", podIP)

	if existingIP := p.GetExistingReservation("foo", "bar"); existingIP == nil || !existingIP.Equal(podIP) {
		t.Errorf("Wrong or no IP returned for reserved address: %v", existingIP)
	}

	p.Spec.StaticReservations = NewIPReservationMap()
	staticPodIP := net.ParseIP("2001:db8::ff32")
	p.Spec.StaticReservations.Reserve("foo", "baz", staticPodIP)

	if existingIP := p.GetExistingReservation("foo", "baz"); existingIP == nil || !existingIP.Equal(podIP) {
		t.Errorf("Failed to get or got wrong ip for existing reservation for static pod IP: %v", existingIP)
	}
}

func TestIPPoolFreePodReservation(t *testing.T) {
	p := IPPool{}
	p.Spec.Range = IPRange{"2001:db8::/65"}
	p.Spec.NetmaskBits = 64

	// Try freeing with no reservations
	p.FreeDynamicPodReservation("foo", "bar")

	p.Status.DynamicReservations = NewIPReservationMap()
	dynamicPodIP := net.ParseIP("2001:db8::234")
	p.Reserve("foo", "bar", dynamicPodIP)

	if !p.AlreadyReserved(dynamicPodIP) {
		t.Errorf("Pool claims dynamically reserved address is available")
	}

	p.FreeDynamicPodReservation("foo", "bar")

	if p.AlreadyReserved(dynamicPodIP) {
		t.Errorf("Freed reservation still claims to be reserved")
	}
}

func TestIPPoolRandomIP(t *testing.T) {
	p := IPPool{}
	p.Spec.Range = IPRange{"2001:db8::/65"}
	p.Spec.NetmaskBits = 64

	randomIP := p.RandomIP()
	if !p.RangeContains(randomIP) {
		t.Errorf("Random ip isn't in network: %v", randomIP)
	}
}

func TestIPPoolAlreadyReserved(t *testing.T) {
	staticPodIP := net.ParseIP("2001:db8::ff32")

	staticReservations := NewIPReservationMap()
	staticReservations.Reserve("foo", "bar", staticPodIP)

	p := IPPool{}
	p.Spec.Range = IPRange{"2001:db8::/65"}
	p.Spec.NetmaskBits = 64

	if p.AlreadyReserved(staticPodIP) {
		t.Errorf("Empty pool claims address is already reserved")
	}

	p.Spec.Gateway = net.ParseIP("2001:db8::1")

	if !p.AlreadyReserved(net.ParseIP("2001:db8::1")) {
		t.Errorf("Gateway not marked as reserved")
	}

	p.Spec.StaticReservations = staticReservations
	if !p.AlreadyReserved(staticPodIP) {
		t.Errorf("Pool claims staticly reserved address is available")
	}

	p.Status.DynamicReservations = NewIPReservationMap()
	dynamicPodIP := net.ParseIP("2001:db8::234")
	p.Status.DynamicReservations.Reserve("baz", "pod1", dynamicPodIP)

	if !p.AlreadyReserved(dynamicPodIP) {
		t.Errorf("Pool claims dynamically reserved address is available")
	}

	if p.AlreadyReserved(net.ParseIP("2001:db8:0:1::")) {
		t.Errorf("Pool claims an address outside of our pool is reserved")
	}

}

func BenchmarkIPPoolRandomIPv6(b *testing.B) {
	p := IPPool{}
	p.Spec.Range = IPRange{"2001:db8::/65"}
	p.Spec.NetmaskBits = 64

	for n := 0; n < b.N; n++ {
		_ = p.RandomIP()
	}
}

func BenchmarkIPPoolRandomIPv4(b *testing.B) {
	p := IPPool{}
	p.Spec.Range = IPRange{"2001:db8::/65"}
	p.Spec.NetmaskBits = 64

	for n := 0; n < b.N; n++ {
		_ = p.RandomIP()
	}
}

func BenchmarkIPPoolAlreadyReserved(b *testing.B) {
	p := IPPool{}
	p.Spec.Range = IPRange{"2001:db8::/65"}
	p.Spec.NetmaskBits = 64

	namespaceCount := b.N / 10000
	if namespaceCount == 0 {
		namespaceCount = 1
	}

	for n := 0; n < b.N; n++ {
		randomIP := p.RandomIP()
		for p.AlreadyReserved(randomIP) {
			randomIP = p.RandomIP()
		}
		p.Reserve(fmt.Sprintf("namespace%d", n%namespaceCount), fmt.Sprintf("pod%d", n), randomIP)
	}
}
