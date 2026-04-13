package k8s_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/config/source/k8s"
)

func ExampleResourceConfigMap() {
	// k8s.New requires a kubernetes.Interface which needs a running K8s cluster.
	// This example shows how you would configure the source:
	//
	//   src := k8s.New(clientset, "default", "my-config",
	//       k8s.WithResourceType(k8s.ResourceConfigMap),
	//       k8s.WithKey("app.json"),
	//       k8s.WithFormat("json"),
	//   )
	//   kvs, _ := src.Load()
	//
	// ResourceConfigMap reads from a Kubernetes ConfigMap.
	fmt.Println(k8s.ResourceConfigMap)
	fmt.Println(k8s.ResourceSecret)
	// Output:
	// 0
	// 1
}
