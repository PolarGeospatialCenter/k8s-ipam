package main

import (
	"net"
	"testing"

	"github.com/PolarGeospatialCenter/k8s-ipam/pkg/api/k8s.pgc.umn.edu/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type FakeKubernetesClient struct {
	Pool v1alpha1.IPPool
}

func (c *FakeKubernetesClient) GetIPPool() (*v1alpha1.IPPool, error) {
	return &c.Pool, nil
}

func (c *FakeKubernetesClient) UpdateIPPool(*v1alpha1.IPPool) error {
	return nil
}

func (c *FakeKubernetesClient) GetPod(namespace, podName string) (*corev1.Pod, error) {
	return nil, nil
}

func TestK8SAllocate(t *testing.T) {
	a := &KubernetesAllocator{Client: &FakeKubernetesClient{
		v1alpha1.IPPool{
			Spec: v1alpha1.IPPoolSpec{
				NetworkIp:   net.ParseIP("2001:db8::"),
				NetworkBits: 64,
				Gateway:     net.ParseIP("2001:db8::1"),
			},
		}}}
	ip, gw, err := a.Allocate("foo", "bar")
	if err != nil {
		t.Errorf("error allocating address: %v", err)
	}

	if ip.IP == nil || ip.IP.Equal(net.IPv4zero) {
		t.Errorf("nil IP returned")
	}
	t.Logf("Reserved IP %s", ip.String())

	if !gw.Equal(net.ParseIP("2001:db8::1")) {
		t.Errorf("wrong gateway")
	}
}
