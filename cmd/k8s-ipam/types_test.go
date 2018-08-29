package main

import (
	"net"
	"testing"
)

func TestIpamResult(t *testing.T) {
	result := &IPAMResult{}

	networkIP, network, _ := net.ParseCIDR("2001:db8::10/64")
	network.IP = networkIP
	result.AddIP(*network, net.ParseIP("2001:db8::1"))
	result.Print()
}
