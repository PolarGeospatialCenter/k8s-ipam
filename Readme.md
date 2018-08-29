# Kubernetes IPAM Plugin

A CNI Ipam plugin that uses a CRD for allocating IPs for a given network.  

When an ip is requested, the plugin retrieves the configured IPPool from the kubernetes API, an IP is allocated from the pool as follows:
* If an IP is already assigned to a pod with a matching name/namespace tuple, that ip is reassigned (any pod that's named the same will get the same IP when relaunched)
* Otherwise an IP is chosen randomly
  * If the chosen IP is available it is marked as belonging to this pod in the pool and assigned.
  * If the chosen IP is assigned, we check to see if the pod that has claimed it is still running.
    * If the pod is no longer running, the IP is reclaimed by us.
    * If the pod is running a new IP is chosen and the process is repeated until an ip is assigned.


Example IPPool CRD:
```yaml
apiVersion: k8s.pgc.umn.edu/v1alpha1
kind: IPPool
objectMeta:
  name: samplePool
spec:
  range: "2001:db8:0:1::/65"
  netmaskBits: 64
  gateway: "2001:db8:0:1::1"
  staticReservations:
    namespace-bar:
      pod-foo: 2001:db8:0:1::23
```  
