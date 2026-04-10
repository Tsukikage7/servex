package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenK8s 测试 K8s 清单生成.
func TestGenK8s(t *testing.T) {
	dir := t.TempDir()

	err := runGenK8s("myservice", 3000, 3, "registry.example.com/myservice:v1.0", dir)
	if err != nil {
		t.Fatalf("runGenK8s: %v", err)
	}

	// 验证 deployment.yaml
	deployPath := filepath.Join(dir, "deployment.yaml")
	if _, err := os.Stat(deployPath); os.IsNotExist(err) {
		t.Fatal("deployment.yaml 不存在")
	}
	content, err := os.ReadFile(deployPath)
	if err != nil {
		t.Fatalf("读取 deployment.yaml: %v", err)
	}
	deployStr := string(content)
	if !contains(deployStr, "kind: Deployment") {
		t.Error("deployment.yaml 应包含 kind: Deployment")
	}
	if !contains(deployStr, "name: myservice") {
		t.Error("deployment.yaml 应包含 name: myservice")
	}
	if !contains(deployStr, "replicas: 3") {
		t.Error("deployment.yaml 应包含 replicas: 3")
	}
	if !contains(deployStr, "image: registry.example.com/myservice:v1.0") {
		t.Error("deployment.yaml 应包含指定镜像")
	}
	if !contains(deployStr, "containerPort: 3000") {
		t.Error("deployment.yaml 应包含 containerPort: 3000")
	}
	if !contains(deployStr, "/healthz") {
		t.Error("deployment.yaml 应包含 livenessProbe 路径 /healthz")
	}
	if !contains(deployStr, "/readyz") {
		t.Error("deployment.yaml 应包含 readinessProbe 路径 /readyz")
	}

	// 验证 service.yaml
	svcPath := filepath.Join(dir, "service.yaml")
	if _, err := os.Stat(svcPath); os.IsNotExist(err) {
		t.Fatal("service.yaml 不存在")
	}
	content, err = os.ReadFile(svcPath)
	if err != nil {
		t.Fatalf("读取 service.yaml: %v", err)
	}
	svcStr := string(content)
	if !contains(svcStr, "kind: Service") {
		t.Error("service.yaml 应包含 kind: Service")
	}
	if !contains(svcStr, "name: myservice") {
		t.Error("service.yaml 应包含 name: myservice")
	}
	if !contains(svcStr, "port: 3000") {
		t.Error("service.yaml 应包含 port: 3000")
	}
	if !contains(svcStr, "ClusterIP") {
		t.Error("service.yaml 应包含 ClusterIP 类型")
	}

	// 验证 hpa.yaml
	hpaPath := filepath.Join(dir, "hpa.yaml")
	if _, err := os.Stat(hpaPath); os.IsNotExist(err) {
		t.Fatal("hpa.yaml 不存在")
	}
	content, err = os.ReadFile(hpaPath)
	if err != nil {
		t.Fatalf("读取 hpa.yaml: %v", err)
	}
	hpaStr := string(content)
	if !contains(hpaStr, "kind: HorizontalPodAutoscaler") {
		t.Error("hpa.yaml 应包含 kind: HorizontalPodAutoscaler")
	}
	if !contains(hpaStr, "minReplicas: 3") {
		t.Error("hpa.yaml 应包含 minReplicas: 3")
	}
	if !contains(hpaStr, "maxReplicas: 9") {
		t.Error("hpa.yaml 应包含 maxReplicas: 9 (3 * 3)")
	}
}

// TestGenK8sDefaults 测试 K8s 生成的默认值.
func TestGenK8sDefaults(t *testing.T) {
	dir := t.TempDir()

	// 不指定镜像，使用默认值
	err := runGenK8s("api", 8080, 2, "", dir)
	if err != nil {
		t.Fatalf("runGenK8s: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "deployment.yaml"))
	if err != nil {
		t.Fatalf("读取 deployment.yaml: %v", err)
	}
	deployStr := string(content)
	if !contains(deployStr, "image: api:latest") {
		t.Error("默认镜像应为 <name>:latest")
	}
	if !contains(deployStr, "replicas: 2") {
		t.Error("deployment.yaml 应包含 replicas: 2")
	}

	// 验证 HPA maxReplicas 至少为 3
	content, err = os.ReadFile(filepath.Join(dir, "hpa.yaml"))
	if err != nil {
		t.Fatalf("读取 hpa.yaml: %v", err)
	}
	hpaStr := string(content)
	if !contains(hpaStr, "maxReplicas: 6") {
		t.Error("hpa.yaml maxReplicas 应为 6 (2 * 3)")
	}
}

// TestGenK8sMinReplicas 测试副本数为 1 时 HPA 最小值保证.
func TestGenK8sMinReplicas(t *testing.T) {
	dir := t.TempDir()

	err := runGenK8s("tiny", 8080, 1, "tiny:v1", dir)
	if err != nil {
		t.Fatalf("runGenK8s: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "hpa.yaml"))
	if err != nil {
		t.Fatalf("读取 hpa.yaml: %v", err)
	}
	hpaStr := string(content)
	if !contains(hpaStr, "maxReplicas: 3") {
		t.Error("hpa.yaml maxReplicas 应至少为 3")
	}
}

// TestGenK8sMissingName 测试缺少名称参数时的错误.
func TestGenK8sMissingName(t *testing.T) {
	dir := t.TempDir()

	err := runGenK8s("", 8080, 2, "", dir)
	if err == nil {
		t.Fatal("缺少名称应返回错误")
	}
}
