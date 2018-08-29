package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
)

type CniConf struct {
	Name       string                `json:"name"`
	CNIVersion string                `json:"cniVersion"`
	IPAM       *KubernetesIPAMConfig `json:"ipam"`
}

type KubernetesIPAMConfig struct {
	Name       string
	Type       string `json:"type"`
	KubeConfig string `json:"kubeConfig"`
	IPPoolName string `json:"ipPoolName"`
}

func (c KubernetesIPAMConfig) GetKubeConfig() string {
	return c.KubeConfig
}

func (c KubernetesIPAMConfig) GetIPPoolName() string {
	return c.IPPoolName
}

type Address struct {
	Version   string      `json:"version"`
	Interface *uint       `json:"interface,omitempty"`
	Address   types.IPNet `json:"address"`
	Gateway   net.IP      `json:"gateway,omitempty"`
}

type IPAMResult struct {
	CniVersion string        `json:"cniVersion"`
	IPs        []Address     `json:"ips"`
	Routes     []types.Route `json:"routes"`
	DNS        types.DNS     `json:"dns"`
}

func (r *IPAMResult) AddIP(ip net.IPNet, gw net.IP) {
	addr := Address{}
	addr.Version = "4"

	if ip.IP.To4() == nil {
		addr.Version = "6"
	}

	addr.Address = types.IPNet(ip)
	if gw != nil {
		addr.Gateway = gw
		destination := net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)}
		if addr.Version == "4" {
			destination = net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)}
		}
		route := types.Route{
			Dst: destination,
			GW:  addr.Gateway,
		}
		if r.Routes == nil {
			r.Routes = make([]types.Route, 0, 1)
		}
		r.Routes = append(r.Routes, route)
	}

	r.IPs = append(r.IPs, addr)
}

func (r IPAMResult) Version() string {
	return r.CniVersion
}

func (r IPAMResult) GetAsVersion(version string) (types.Result, error) {
	switch version {
	case "0.3.0", current.ImplementedSpecVersion:
		r.CniVersion = version
		return r, nil
	}
	return nil, fmt.Errorf("cannot convert version 0.3.x to %q", version)
}

func (r IPAMResult) Print() error {
	data, err := json.MarshalIndent(r, "", "    ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

func (r IPAMResult) String() string {
	var str string
	if len(r.IPs) > 0 {
		str += fmt.Sprintf("IP:%+v, ", r.IPs)
	}
	if len(r.Routes) > 0 {
		str += fmt.Sprintf("Routes:%+v, ", r.Routes)
	}
	return fmt.Sprintf("%sDNS:%+v", str, r.DNS)
}
