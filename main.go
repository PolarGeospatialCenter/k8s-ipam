package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
)

// parseConfig parses the supplied configuration (and prevResult) from stdin.
func parseConfig(stdin []byte) (*CniConf, error) {
	conf := &CniConf{}

	if err := json.Unmarshal(stdin, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse network configuration: %v", err)
	}

	if conf.IPAM.GetKubeConfig() == "" {
		return nil, fmt.Errorf("a kubeconfig is required for this ip allocator.")
	}

	if conf.IPAM.GetIPPoolName() == "" {
		return nil, fmt.Errorf("an ip pool name is required for this ip allocator.")
	}

	return conf, nil
}

func getPodFromArgs(args string) (namespace, podName string, err error) {
	argList := strings.Split(args, ";")
	argMap := make(map[string]string, len(argList))
	for _, arg := range argList {
		vals := strings.Split(arg, "=")
		if len(vals) == 2 {
			argMap[vals[0]] = vals[1]
		}
	}

	namespace, ok := argMap["K8S_POD_NAMESPACE"]
	if !ok || namespace == "" {
		return namespace, "", fmt.Errorf("no K8S_POD_NAMESPACE provided in CNI_ARGS")
	}

	podName, ok = argMap["K8S_POD_NAME"]
	if !ok || podName == "" {
		return namespace, podName, fmt.Errorf("no K8S_POD_NAME provided in CNI_ARGS")
	}

	return namespace, podName, nil
}

func cmdAdd(args *skel.CmdArgs) error {
	conf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	// if we have kubeconfig, create client and check for annotation on pod
	namespace, podName, err := getPodFromArgs(args.Args)
	if err != nil {
		return err
	}

	allocator := &KubernetesAllocator{}
	allocator.Client = &KubeClient{
		KubeConfig: conf.IPAM.GetKubeConfig(),
		IPPoolName: conf.IPAM.GetIPPoolName(),
	}

	var ip net.IPNet
	var gw net.IP
	var allocateErr error
	for allocateErr = ErrUpdateConflict; allocateErr == ErrUpdateConflict; {
		ip, gw, allocateErr = allocator.Allocate(namespace, podName)
	}
	if allocateErr != nil {
		return fmt.Errorf("unable to get allocation for pod: %v", allocateErr)
	}

	result := &IPAMResult{}
	result.CniVersion = current.ImplementedSpecVersion
	result.AddIP(ip, gw)
	return types.PrintResult(result, current.ImplementedSpecVersion)
}

// cmdDel is called for DELETE requests
func cmdDel(args *skel.CmdArgs) error {
	conf, err := parseConfig(args.StdinData)
	if err != nil {
		return err
	}

	// if we have kubeconfig, create client and check for annotation on pod
	namespace, podName, err := getPodFromArgs(args.Args)
	if err != nil {
		return err
	}

	allocator := &KubernetesAllocator{}
	allocator.Client = &KubeClient{
		KubeConfig: conf.IPAM.GetKubeConfig(),
		IPPoolName: conf.IPAM.GetIPPoolName(),
	}

	for freeErr := ErrUpdateConflict; freeErr == ErrUpdateConflict; {
		freeErr = allocator.Free(namespace, podName)
	}
	if err != nil {
		return fmt.Errorf("unable to get allocation for pod: %v", err)
	}

	result := &IPAMResult{}
	result.CniVersion = current.ImplementedSpecVersion
	return types.PrintResult(result, current.ImplementedSpecVersion)

}

func main() {
	skel.PluginMain(cmdAdd, cmdDel, version.PluginSupports("", "0.1.0", "0.2.0", version.Current()))
}
