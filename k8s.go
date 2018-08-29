package main

import (
	"errors"
	"fmt"
	"net"

	"github.com/PolarGeospatialCenter/k8s-ipam/pkg/api/k8s.pgc.umn.edu/v1alpha1"
	ipamclient "github.com/PolarGeospatialCenter/k8s-ipam/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var ErrUpdateConflict = errors.New("failed to update, most likely due to resource version mismatch.  Did someone else update this?  Retry.")

type PodRetriever interface {
	GetPod(string, string) (*corev1.Pod, error)
}

type IPPoolManipulator interface {
	GetIPPool() (*v1alpha1.IPPool, error)
	UpdateIPPool(*v1alpha1.IPPool) error
}

type KubernetesAllocatorClient interface {
	PodRetriever
	IPPoolManipulator
}

type KubeClient struct {
	KubeConfig string
	IPPoolName string
}

func (k *KubeClient) client() (*kubernetes.Clientset, error) {
	conf, err := clientcmd.BuildConfigFromFlags("", k.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to load kubeconfig from %s: %v", k.KubeConfig, err)
	}

	return kubernetes.NewForConfig(conf)
}

func (k *KubeClient) GetPod(namespace, podName string) (*corev1.Pod, error) {
	client, err := k.client()
	if err != nil {
		return nil, fmt.Errorf("error getting client: %v", err)
	}

	pod, err := client.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil && kubeerrors.IsNotFound(err) {
		return nil, nil
	}

	return pod, err
}

func (k *KubeClient) GetIPPool() (*v1alpha1.IPPool, error) {
	conf, err := clientcmd.BuildConfigFromFlags("", k.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to load kubeconfig from %s: %v", k.KubeConfig, err)
	}

	client, err := ipamclient.NewForConfig(conf)
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %v", err)
	}

	return client.K8sV1alpha1().IPPools().Get(k.IPPoolName, metav1.GetOptions{})
}

func (k *KubeClient) UpdateIPPool(pool *v1alpha1.IPPool) error {
	conf, err := clientcmd.BuildConfigFromFlags("", k.KubeConfig)
	if err != nil {
		return fmt.Errorf("unable to load kubeconfig from %s: %v", k.KubeConfig, err)
	}

	client, err := ipamclient.NewForConfig(conf)
	if err != nil {
		return fmt.Errorf("unable to create client: %v", err)
	}

	_, err = client.K8sV1alpha1().IPPools().UpdateStatus(pool)
	return err
}

type KubernetesAllocator struct {
	Client KubernetesAllocatorClient
}

func (a *KubernetesAllocator) Allocate(namespace, podName string) (ip net.IPNet, gateway net.IP, err error) {
	p, err := a.Client.GetIPPool()
	if err != nil {
		return ip, gateway, err
	}
	gateway = p.Gateway()
	ip = *p.Spec.Network()

	// * If an IP is already assigned to a pod with a matching name/namespace tuple, that ip is reassigned (any pod that's named the same will get the same IP when relaunched)
	if existingIP := p.GetExistingReservation(namespace, podName); existingIP != nil {
		ip.IP = *existingIP
		return ip, gateway, nil
	}
	// * Otherwise an IP is chosen randomly
	var allocatedIP net.IP
	for allocatedIP == nil {
		candidateIP := p.RandomIP()
		if existingPodNS, existingPodName, found := p.GetPodForIP(candidateIP); found {
			// If the chosen IP is assigned, we check to see if the pod that has claimed it is still running.
			pod, err := a.Client.GetPod(existingPodNS, existingPodName)
			if err != nil {
				return ip, gateway, err
			}

			// * If the pod is no longer running, the IP is reclaimed by us.
			if pod == nil {
				p.Reserve(namespace, podName, candidateIP)
			}
			// * If the pod is running a new IP is chosen and the process is repeated until an ip is assigned.
			continue
		}

		if !p.AlreadyReserved(candidateIP) {
			// If the chosen IP is available it is marked as belonging to this pod in the pool and assigned.
			allocatedIP = candidateIP
			break
		}
	}

	ip.IP = allocatedIP

	err = a.Client.UpdateIPPool(p)
	if err != nil && kubeerrors.IsConflict(err) {
		// update failed due to stale resourceversion
		return ip, gateway, ErrUpdateConflict
	}

	return ip, gateway, err
}

func (a *KubernetesAllocator) Free(namespace, podName string) error {
	p, err := a.Client.GetIPPool()
	if err != nil {
		return err
	}

	p.FreeDynamicPodReservation(namespace, podName)

	err = a.Client.UpdateIPPool(p)
	if err != nil && kubeerrors.IsConflict(err) {
		// update failed due to stale resourceversion
		return ErrUpdateConflict
	}
	return err
}
