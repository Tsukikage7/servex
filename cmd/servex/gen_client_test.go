package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenClient 测试外部服务客户端适配器生成.
func TestGenClient(t *testing.T) {
	dir := t.TempDir()

	err := runGenClient("user", "order", "github.com/example/myplatform", dir)
	if err != nil {
		t.Fatalf("runGenClient: %v", err)
	}

	// 验证 ports.go 生成
	portsPath := filepath.Join(dir, "domain", "order", "ports.go")
	if _, err := os.Stat(portsPath); os.IsNotExist(err) {
		t.Fatal("domain/order/ports.go does not exist")
	}

	content, err := os.ReadFile(portsPath)
	if err != nil {
		t.Fatalf("read ports.go: %v", err)
	}
	portsStr := string(content)
	if !contains(portsStr, "package order") {
		t.Error("ports.go should contain 'package order'")
	}
	if !contains(portsStr, "UserProvider") {
		t.Error("ports.go should contain UserProvider interface")
	}
	if !contains(portsStr, "GetUser") {
		t.Error("ports.go should contain GetUser method")
	}
	if !contains(portsStr, "UserInfo") {
		t.Error("ports.go should contain UserInfo struct")
	}

	// 验证 external client 生成
	clientPath := filepath.Join(dir, "services", "order-service",
		"internal", "adapter", "external", "user_client.go")
	if _, err := os.Stat(clientPath); os.IsNotExist(err) {
		t.Fatal("services/order-service/internal/adapter/external/user_client.go does not exist")
	}

	content, err = os.ReadFile(clientPath)
	if err != nil {
		t.Fatalf("read user_client.go: %v", err)
	}
	clientStr := string(content)
	if !contains(clientStr, "package external") {
		t.Error("user_client.go should contain 'package external'")
	}
	if !contains(clientStr, "UserClient") {
		t.Error("user_client.go should contain UserClient struct")
	}
	if !contains(clientStr, "NewUserClient") {
		t.Error("user_client.go should contain NewUserClient constructor")
	}
	if !contains(clientStr, "grpc.ClientConn") {
		t.Error("user_client.go should contain grpc.ClientConn")
	}
	if !contains(clientStr, "domainorder") {
		t.Error("user_client.go should import domainorder alias")
	}
}

// TestGenClientMissingService 测试缺少 --service 参数时的错误.
func TestGenClientMissingService(t *testing.T) {
	dir := t.TempDir()

	err := runGenClient("user", "", "github.com/example/myplatform", dir)
	if err == nil {
		t.Fatal("expected error when --service is missing")
	}
}

// TestGenClientMissingTarget 测试缺少 target 参数时的错误.
func TestGenClientMissingTarget(t *testing.T) {
	dir := t.TempDir()

	err := runGenClient("", "order", "github.com/example/myplatform", dir)
	if err == nil {
		t.Fatal("expected error when target is missing")
	}
}

// TestGenEntity 测试子实体生成.
func TestGenEntity(t *testing.T) {
	dir := t.TempDir()

	err := runGenEntity("line_item", "order", "id:uint64,product_name:string,quantity:int,price:float64",
		"github.com/example/myplatform", dir)
	if err != nil {
		t.Fatalf("runGenEntity: %v", err)
	}

	outPath := filepath.Join(dir, "domain", "order", "line_item.go")
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Fatal("domain/order/line_item.go does not exist")
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read line_item.go: %v", err)
	}
	entityStr := string(content)

	if !contains(entityStr, "package order") {
		t.Error("entity should have package order")
	}
	if !contains(entityStr, "type LineItem struct") {
		t.Error("entity should contain LineItem struct")
	}
	if !contains(entityStr, "id uint64") {
		t.Error("entity should have id field with uint64 type")
	}
	if !contains(entityStr, "func (e *LineItem) ID() uint64") {
		t.Error("entity should have ID() getter")
	}
	if !contains(entityStr, "NewLineItem") {
		t.Error("entity should have NewLineItem constructor")
	}
	if !contains(entityStr, "ProductName") {
		t.Error("entity should have ProductName getter")
	}
	if !contains(entityStr, "SetProductName") {
		t.Error("entity should have SetProductName setter")
	}
}

// TestGenEntityMissingAggregate 测试缺少 --aggregate 参数时的错误.
func TestGenEntityMissingAggregate(t *testing.T) {
	dir := t.TempDir()

	err := runGenEntity("line_item", "", "id:uint64,name:string",
		"github.com/example/myplatform", dir)
	if err == nil {
		t.Fatal("expected error when --aggregate is missing")
	}
}

// TestGenEntityWithTimeField 测试包含 time.Time 字段的子实体生成.
func TestGenEntityWithTimeField(t *testing.T) {
	dir := t.TempDir()

	err := runGenEntity("payment", "order", "id:uint64,amount:float64,paid_at:time.Time",
		"github.com/example/myplatform", dir)
	if err != nil {
		t.Fatalf("runGenEntity: %v", err)
	}

	outPath := filepath.Join(dir, "domain", "order", "payment.go")
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read payment.go: %v", err)
	}
	if !contains(string(content), "\"time\"") {
		t.Error("entity with time.Time field should import time package")
	}
}

// TestGenValueObject 测试值对象生成.
func TestGenValueObject(t *testing.T) {
	dir := t.TempDir()

	err := runGenValueObject("address", "order", "street:string,city:string,zip_code:string",
		"github.com/example/myplatform", dir)
	if err != nil {
		t.Fatalf("runGenValueObject: %v", err)
	}

	outPath := filepath.Join(dir, "domain", "order", "address.go")
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Fatal("domain/order/address.go does not exist")
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read address.go: %v", err)
	}
	voStr := string(content)

	if !contains(voStr, "package order") {
		t.Error("value object should have package order")
	}
	if !contains(voStr, "type Address struct") {
		t.Error("value object should contain Address struct")
	}
	if !contains(voStr, "NewAddress") {
		t.Error("value object should have NewAddress constructor")
	}
	if !contains(voStr, "Equals") {
		t.Error("value object should have Equals method")
	}
	if !contains(voStr, "func (v Address)") {
		t.Error("value object methods should use value receiver")
	}
	if !contains(voStr, "Street") {
		t.Error("value object should have Street getter")
	}
	if !contains(voStr, "City") {
		t.Error("value object should have City getter")
	}
	if !contains(voStr, "ZipCode") {
		t.Error("value object should have ZipCode getter")
	}
}

// TestGenValueObjectMissingAggregate 测试缺少 --aggregate 参数时的错误.
func TestGenValueObjectMissingAggregate(t *testing.T) {
	dir := t.TempDir()

	err := runGenValueObject("address", "", "street:string,city:string",
		"github.com/example/myplatform", dir)
	if err == nil {
		t.Fatal("expected error when --aggregate is missing")
	}
}

// TestGenValueObjectImmutable 测试值对象不含 setter[不可变性].
func TestGenValueObjectImmutable(t *testing.T) {
	dir := t.TempDir()

	err := runGenValueObject("money", "order", "amount:float64,currency:string",
		"github.com/example/myplatform", dir)
	if err != nil {
		t.Fatalf("runGenValueObject: %v", err)
	}

	outPath := filepath.Join(dir, "domain", "order", "money.go")
	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read money.go: %v", err)
	}
	voStr := string(content)

	// 值对象不应有 Set 方法
	if contains(voStr, "Set") {
		t.Error("value object should NOT have setter methods (immutable)")
	}
}
