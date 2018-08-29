

k8s: vendor/k8s.io/code-generator
	vendor/k8s.io/code-generator/generate-groups.sh all github.com/PolarGeospatialCenter/k8s-ipam/pkg/client github.com/PolarGeospatialCenter/k8s-ipam/pkg/api "k8s.pgc.umn.edu:v1alpha1"
	grep -Rl "github.com/polargeospatialcenter"  pkg/client | xargs sed -i "" -e "s@github.com/polargeospatialcenter@github.com/PolarGeospatialCenter@g"
	

vendor/k8s.io/code-generator:
	git clone https://github.com/kubernetes/code-generator vendor/k8s.io/code-generator
