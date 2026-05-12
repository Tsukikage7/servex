package main

import (
	"fmt"
	"path/filepath"
)

// K8sData K8s 清单模板数据.
type K8sData struct {
	Name        string // 服务名称
	Port        int    // 容器端口
	Replicas    int    // 副本数
	Image       string // 容器镜像
	MaxReplicas int    // HPA 最大副本数
}

// k8sFiles K8s 模板文件映射.
var k8sFiles = []struct {
	tmpl string // 模板路径
	out  string // 输出文件名
}{
	{"templates/gen/k8s/deployment.yaml.tmpl", "deployment.yaml"},
	{"templates/gen/k8s/service.yaml.tmpl", "service.yaml"},
	{"templates/gen/k8s/hpa.yaml.tmpl", "hpa.yaml"},
}

// runGenK8s 执行 servex gen k8s 命令.
func runGenK8s(name string, port, replicas int, image, outputDir string) error {
	if name == "" {
		return fmt.Errorf("必须指定服务名称 (--name)")
	}
	if image == "" {
		image = name + ":latest"
	}

	data := K8sData{
		Name:        name,
		Port:        port,
		Replicas:    replicas,
		Image:       image,
		MaxReplicas: replicas * 3,
	}
	if data.MaxReplicas < 3 {
		data.MaxReplicas = 3
	}

	return generateK8s(data, outputDir)
}

// generateK8s 根据模板数据生成 K8s 清单文件.
func generateK8s(data K8sData, outputDir string) error {
	for _, kf := range k8sFiles {
		outPath := filepath.Join(outputDir, kf.out)
		if err := renderTemplate(genTemplates, kf.tmpl, outPath, data, nil); err != nil {
			return fmt.Errorf("渲染 %s: %w", kf.tmpl, err)
		}
	}

	fmt.Printf("K8s 清单已生成:\n")
	fmt.Printf("  %s/deployment.yaml  (Deployment, %d 副本)\n", outputDir, data.Replicas)
	fmt.Printf("  %s/service.yaml     (Service, 端口 %d)\n", outputDir, data.Port)
	fmt.Printf("  %s/hpa.yaml         (HPA, %d-%d 副本)\n", outputDir, data.Replicas, data.MaxReplicas)

	return nil
}
