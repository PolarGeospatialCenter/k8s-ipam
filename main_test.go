package main

import (
	"testing"
)

func TestUnwrapConfig(t *testing.T) {
	mainConfig := `{
      "cniVersion": "0.3.1",
      "name": "testConf",
      "type": "macvlan",
      "ipam": {
        "type": "ipam-wrapper",
				"kubeConfig": "/path/to/kubeconfig.yml",
				"ipPoolName": "sample-ippool"
        }
    }`

	m, err := parseConfig([]byte(mainConfig))
	if err != nil {
		t.Errorf("Error parsing config: %v", err)
	}

	if m.IPAM.GetKubeConfig() != "/path/to/kubeconfig.yml" {
		t.Errorf("Wrong kubeconfig")
	}

	if m.IPAM.GetIPPoolName() != "sample-ippool" {
		t.Errorf("Wrong ip pool")
	}
}

func TestParseArgs(t *testing.T) {
	args := "K8S_POD_NAMESPACE=foo;K8S_POD_NAME=bar"
	namespace, podName, err := getPodFromArgs(args)
	if err != nil {
		t.Errorf("unable to parse args: %v", err)
	}

	if namespace != "foo" {
		t.Errorf("wrong namespace")
	}

	if podName != "bar" {
		t.Errorf("wrong pod name")
	}

}
