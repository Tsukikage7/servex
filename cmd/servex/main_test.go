package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestToPascalCase 测试 PascalCase 转换.
func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user", "User"},
		{"user_profile", "UserProfile"},
		{"user-profile", "UserProfile"},
		{"", ""},
		{"ID", "Id"},
	}
	for _, tt := range tests {
		got := toPascalCase(tt.input)
		if got != tt.want {
			t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestToCamelCase 测试 camelCase 转换.
func TestToCamelCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user", "user"},
		{"user_profile", "userProfile"},
		{"user-profile", "userProfile"},
		{"", ""},
	}
	for _, tt := range tests {
		got := toCamelCase(tt.input)
		if got != tt.want {
			t.Errorf("toCamelCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestToSnakeCase 测试 snake_case 转换.
func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"UserProfile", "user_profile"},
		{"user", "user"},
		{"ID", "i_d"},
		{"", ""},
	}
	for _, tt := range tests {
		got := toSnakeCase(tt.input)
		if got != tt.want {
			t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestParseFields 测试字段解析.
func TestParseFields(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"id:uint64,name:string,email:string", 3},
		{"id:uint64", 1},
		{"", 0},
		{"name:string,created_at:time.Time", 2},
	}
	for _, tt := range tests {
		got := parseFields(tt.input)
		if len(got) != tt.want {
			t.Errorf("parseFields(%q) returned %d fields, want %d", tt.input, len(got), tt.want)
		}
	}

	// 验证字段详情
	fields := parseFields("user_name:string,created_at:time.Time")
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}
	if fields[0].Name != "UserName" {
		t.Errorf("field[0].Name = %q, want %q", fields[0].Name, "UserName")
	}
	if fields[0].JSONTag != "user_name" {
		t.Errorf("field[0].JSONTag = %q, want %q", fields[0].JSONTag, "user_name")
	}
	if fields[1].Type != "time.Time" {
		t.Errorf("field[1].Type = %q, want %q", fields[1].Type, "time.Time")
	}
}

// TestNeedsTimeImport 测试 time 导入检测.
func TestNeedsTimeImport(t *testing.T) {
	if needsTimeImport(nil) {
		t.Error("nil fields should not need time import")
	}
	fields := []Field{{Name: "CreatedAt", Type: "time.Time", JSONTag: "created_at"}}
	if !needsTimeImport(fields) {
		t.Error("fields with time.Time should need time import")
	}
	fields = []Field{{Name: "Name", Type: "string", JSONTag: "name"}}
	if needsTimeImport(fields) {
		t.Error("fields without time.Time should not need time import")
	}
}

// TestZeroValue 测试零值字面量生成.
func TestZeroValue(t *testing.T) {
	tests := []struct {
		typ  string
		want string
	}{
		{"string", `""`},
		{"int", "0"},
		{"int64", "0"},
		{"uint64", "0"},
		{"float64", "0"},
		{"bool", "false"},
		{"time.Time", "time.Time{}"},
		{"CustomType", `""`},
	}
	for _, tt := range tests {
		got := zeroValue(tt.typ)
		if got != tt.want {
			t.Errorf("zeroValue(%q) = %q, want %q", tt.typ, got, tt.want)
		}
	}
}

// TestBuildAggregateData 测试聚合数据构建.
func TestBuildAggregateData(t *testing.T) {
	fields := parseFields("id:uint64,name:string,email:string")
	data := buildAggregateData("user", "github.com/example/myservice", fields, "", "")

	if data.Name != "User" {
		t.Errorf("Name = %q, want %q", data.Name, "User")
	}
	if data.NameLower != "user" {
		t.Errorf("NameLower = %q, want %q", data.NameLower, "user")
	}
	if data.IDType != "uint64" {
		t.Errorf("IDType = %q, want %q", data.IDType, "uint64")
	}
	if len(data.NonIDFields) != 2 {
		t.Errorf("NonIDFields count = %d, want 2", len(data.NonIDFields))
	}
}

// TestBuildAggregateDataDefaultID 测试无 ID 字段时的默认值.
func TestBuildAggregateDataDefaultID(t *testing.T) {
	fields := parseFields("name:string,email:string")
	data := buildAggregateData("user", "github.com/example/myservice", fields, "", "")

	if data.IDType != "uint64" {
		t.Errorf("IDType = %q, want default %q", data.IDType, "uint64")
	}
	if len(data.NonIDFields) != 2 {
		t.Errorf("NonIDFields count = %d, want 2", len(data.NonIDFields))
	}
}

// TestGenerateProject 测试项目生成.
func TestGenerateProject(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	infraDefs := parseInfra("mysql,redis")
	data := ProjectData{
		Name:          "testproject",
		Module:        "github.com/example/testproject",
		WithGRPC:      true,
		Infra:         infraDefs,
		AllComponents: infraDefs,
	}

	if err := generateProject(data); err != nil {
		t.Fatalf("generateProject: %v", err)
	}

	// 验证文件存在
	expectedFiles := []string{
		"testproject/go.mod",
		"testproject/README.md",
		"testproject/config.yaml",
		"testproject/justfile",
		"testproject/Dockerfile",
		"testproject/.gitignore",
		"testproject/cmd/server/main.go",
		"testproject/cmd/server/wire.go",
		"testproject/cmd/server/provider.go",
		"testproject/cmd/server/config.go",
		"testproject/internal/server/http.go",
		"testproject/internal/server/grpc.go",
		"testproject/internal/service/service.go",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %q does not exist", f)
		}
	}

	// 验证 go.mod 内容
	content, err := os.ReadFile(filepath.Join(dir, "testproject/go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if got := string(content); !contains(got, "module github.com/example/testproject") {
		t.Errorf("go.mod missing module path, got:\n%s", got)
	}
}

// TestGenerateProjectWithoutGRPC 测试不含 gRPC 的项目生成.
func TestGenerateProjectWithoutGRPC(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	data := ProjectData{
		Name:   "simpleproject",
		Module: "github.com/example/simpleproject",
	}

	if err := generateProject(data); err != nil {
		t.Fatalf("generateProject: %v", err)
	}

	// gRPC 文件不应存在
	grpcPath := filepath.Join(dir, "simpleproject/internal/server/grpc.go")
	if _, err := os.Stat(grpcPath); !os.IsNotExist(err) {
		t.Error("grpc.go should not exist when WithGRPC is false")
	}
}

// TestGenerateAggregate 测试 DDD 聚合代码生成.
func TestGenerateAggregate(t *testing.T) {
	dir := t.TempDir()

	fields := parseFields("id:uint64,name:string,email:string")
	data := buildAggregateData("user", "github.com/example/myservice", fields, "", "")

	if err := generateAggregate(data, dir); err != nil {
		t.Fatalf("generateAggregate: %v", err)
	}

	expectedFiles := []string{
		"domain/user/aggregate.go",
		"domain/user/event.go",
		"domain/user/repository.go",
		"domain/user/command.go",
		"domain/user/query.go",
		"domain/user/aggregate_test.go",
		"domain/user/repository_mock.go",
		"application/user/service.go",
		"application/user/service_test.go",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %q does not exist", f)
		}
	}

	// 验证 aggregate.go 包含 AggregateRoot 和 Reconstruct
	content, err := os.ReadFile(filepath.Join(dir, "domain/user/aggregate.go"))
	if err != nil {
		t.Fatalf("read aggregate.go: %v", err)
	}
	aggStr := string(content)
	if !contains(aggStr, "domain.AggregateRoot[uint64]") {
		t.Error("aggregate.go should contain domain.AggregateRoot[uint64]")
	}
	if !contains(aggStr, "NewUser") {
		t.Error("aggregate.go should contain NewUser constructor")
	}
	if !contains(aggStr, "ReconstructUser") {
		t.Error("aggregate.go should contain ReconstructUser")
	}
	if !contains(aggStr, "RaiseEvent") {
		t.Error("aggregate.go should contain RaiseEvent call")
	}

	// 验证 event.go 包含领域事件
	content, err = os.ReadFile(filepath.Join(dir, "domain/user/event.go"))
	if err != nil {
		t.Fatalf("read event.go: %v", err)
	}
	evtStr := string(content)
	if !contains(evtStr, "UserCreatedEvent") {
		t.Error("event.go should contain UserCreatedEvent")
	}
	if !contains(evtStr, "domain.BaseEvent") {
		t.Error("event.go should contain domain.BaseEvent")
	}

	// 验证 repository.go 包含仓储接口[含 List 和 Filter]
	content, err = os.ReadFile(filepath.Join(dir, "domain/user/repository.go"))
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	repoStr := string(content)
	if !contains(repoStr, "Repository interface") {
		t.Error("repository.go should contain Repository interface")
	}
	if !contains(repoStr, "FindByID") {
		t.Error("repository.go should contain FindByID method")
	}
	if !contains(repoStr, "List(") {
		t.Error("repository.go should contain List method")
	}
	if !contains(repoStr, "UserFilter") {
		t.Error("repository.go should contain UserFilter struct")
	}

	// 验证 command.go 包含 CQRS 命令 (fallback 模式含 UpdateCommand)
	content, err = os.ReadFile(filepath.Join(dir, "domain/user/command.go"))
	if err != nil {
		t.Fatalf("read command.go: %v", err)
	}
	cmdStr := string(content)
	if !contains(cmdStr, "CreateUserCommand") {
		t.Error("command.go should contain CreateUserCommand")
	}
	if !contains(cmdStr, "DeleteUserCommand") {
		t.Error("command.go should contain DeleteUserCommand")
	}
	if !contains(cmdStr, "UpdateUserCommand") {
		t.Error("command.go should contain UpdateUserCommand in fallback mode")
	}

	// 验证 query.go 包含 CQRS 查询
	content, err = os.ReadFile(filepath.Join(dir, "domain/user/query.go"))
	if err != nil {
		t.Fatalf("read query.go: %v", err)
	}
	queryStr := string(content)
	if !contains(queryStr, "GetUserQuery") {
		t.Error("query.go should contain GetUserQuery")
	}
	if !contains(queryStr, "UserView") {
		t.Error("query.go should contain UserView")
	}

	// 验证 service.go 包含应用服务 (fallback 模式含 HandleUpdate)
	content, err = os.ReadFile(filepath.Join(dir, "application/user/service.go"))
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	svcStr := string(content)
	if !contains(svcStr, "HandleCreate") {
		t.Error("service.go should contain HandleCreate")
	}
	if !contains(svcStr, "HandleUpdate") {
		t.Error("service.go should contain HandleUpdate in fallback mode")
	}
	if !contains(svcStr, "HandleDelete") {
		t.Error("service.go should contain HandleDelete")
	}
	if !contains(svcStr, "HandleList") {
		t.Error("service.go should contain HandleList")
	}
	if !contains(svcStr, "eventBus") {
		t.Error("service.go should contain eventBus dependency")
	}

	// 验证 aggregate_test.go 包含聚合测试骨架
	content, err = os.ReadFile(filepath.Join(dir, "domain/user/aggregate_test.go"))
	if err != nil {
		t.Fatalf("read aggregate_test.go: %v", err)
	}
	aggTestStr := string(content)
	if !contains(aggTestStr, "TestNewUser") {
		t.Error("aggregate_test.go should contain TestNewUser")
	}
	if !contains(aggTestStr, "TestReconstructUser") {
		t.Error("aggregate_test.go should contain TestReconstructUser")
	}
	if !contains(aggTestStr, "stretchr/testify") {
		t.Error("aggregate_test.go should import testify")
	}

	// 验证 repository_mock.go 包含 Mock 仓储[含 List]
	content, err = os.ReadFile(filepath.Join(dir, "domain/user/repository_mock.go"))
	if err != nil {
		t.Fatalf("read repository_mock.go: %v", err)
	}
	mockStr := string(content)
	if !contains(mockStr, "MockRepository") {
		t.Error("repository_mock.go should contain MockRepository")
	}
	if !contains(mockStr, "NewMockRepository") {
		t.Error("repository_mock.go should contain NewMockRepository")
	}
	if !contains(mockStr, "FindByID") {
		t.Error("repository_mock.go should contain FindByID")
	}
	if !contains(mockStr, "func (m *MockRepository) List(") {
		t.Error("repository_mock.go should contain List method")
	}

	// 验证 service_test.go 包含服务测试骨架
	content, err = os.ReadFile(filepath.Join(dir, "application/user/service_test.go"))
	if err != nil {
		t.Fatalf("read service_test.go: %v", err)
	}
	svcTestStr := string(content)
	if !contains(svcTestStr, "TestService_HandleCreate") {
		t.Error("service_test.go should contain TestService_HandleCreate")
	}
	if !contains(svcTestStr, "NewMockRepository") {
		t.Error("service_test.go should use NewMockRepository")
	}
	if !contains(svcTestStr, "NewEventBus") {
		t.Error("service_test.go should use NewEventBus")
	}
}

// TestGenerateAggregateWithTimeField 测试包含 time.Time 字段的聚合生成.
func TestGenerateAggregateWithTimeField(t *testing.T) {
	dir := t.TempDir()

	fields := parseFields("id:uint64,name:string,created_at:time.Time")
	data := buildAggregateData("order", "github.com/example/myservice", fields, "", "")

	if err := generateAggregate(data, dir); err != nil {
		t.Fatalf("generateAggregate: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "domain/order/query.go"))
	if err != nil {
		t.Fatalf("read query.go: %v", err)
	}
	if !contains(string(content), "time.Time") {
		t.Error("query.go with time.Time field should contain time.Time type")
	}
}

// TestParseCommands 测试业务命令解析.
func TestParseCommands(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"Place,Cancel,Ship", 3},
		{"Place", 1},
		{"Place, Cancel, Ship", 3},
	}
	for _, tt := range tests {
		got := parseCommands(tt.input)
		if len(got) != tt.want {
			t.Errorf("parseCommands(%q) returned %d commands, want %d", tt.input, len(got), tt.want)
		}
	}

	// 验证命令详情
	cmds := parseCommands("Place,Cancel,Ship")
	if cmds[0].Name != "Place" {
		t.Errorf("cmds[0].Name = %q, want %q", cmds[0].Name, "Place")
	}
	if cmds[0].EventName != "Placed" {
		t.Errorf("cmds[0].EventName = %q, want %q", cmds[0].EventName, "Placed")
	}
	if cmds[1].Name != "Cancel" {
		t.Errorf("cmds[1].Name = %q, want %q", cmds[1].Name, "Cancel")
	}
	if cmds[1].EventName != "Cancelled" {
		t.Errorf("cmds[1].EventName = %q, want %q", cmds[1].EventName, "Cancelled")
	}
	if cmds[2].Name != "Ship" {
		t.Errorf("cmds[2].Name = %q, want %q", cmds[2].Name, "Ship")
	}
	if cmds[2].EventName != "Shipped" {
		t.Errorf("cmds[2].EventName = %q, want %q", cmds[2].EventName, "Shipped")
	}
}

// TestToPastTense 测试过去式转换.
func TestToPastTense(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Place", "Placed"},
		{"Cancel", "Cancelled"},
		{"Ship", "Shipped"},
		{"Create", "Created"},
		{"", ""},
	}
	for _, tt := range tests {
		got := toPastTense(tt.input)
		if got != tt.want {
			t.Errorf("toPastTense(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestParseUniqueFields 测试唯一字段解析.
func TestParseUniqueFields(t *testing.T) {
	fields := parseFields("name:string,email:string,age:int")
	// 去掉 ID
	var nonID []Field
	for _, f := range fields {
		nonID = append(nonID, f)
	}

	// 匹配 by PascalCase
	result := parseUniqueFields("Name,Email", nonID)
	if len(result) != 2 {
		t.Fatalf("parseUniqueFields returned %d fields, want 2", len(result))
	}
	if result[0].Name != "Name" {
		t.Errorf("result[0].Name = %q, want %q", result[0].Name, "Name")
	}
	if result[1].Name != "Email" {
		t.Errorf("result[1].Name = %q, want %q", result[1].Name, "Email")
	}

	// 空值
	result = parseUniqueFields("", nonID)
	if len(result) != 0 {
		t.Errorf("parseUniqueFields with empty string should return 0, got %d", len(result))
	}

	// 不匹配
	result = parseUniqueFields("nonexistent", nonID)
	if len(result) != 0 {
		t.Errorf("parseUniqueFields with nonexistent field should return 0, got %d", len(result))
	}
}

// TestBuildAggregateDataWithCommands 测试带命令的聚合数据构建.
func TestBuildAggregateDataWithCommands(t *testing.T) {
	fields := parseFields("id:uint64,name:string,email:string")
	data := buildAggregateData("order", "github.com/example/myservice", fields, "Place,Cancel,Ship", "email")

	if !data.HasCommands {
		t.Error("HasCommands should be true")
	}
	if len(data.Commands) != 3 {
		t.Errorf("Commands count = %d, want 3", len(data.Commands))
	}
	if len(data.UniqueFields) != 1 {
		t.Errorf("UniqueFields count = %d, want 1", len(data.UniqueFields))
	}
	if data.UniqueFields[0].Name != "Email" {
		t.Errorf("UniqueFields[0].Name = %q, want %q", data.UniqueFields[0].Name, "Email")
	}
}

// TestGenerateAggregateWithCommands 测试带业务命令的聚合生成.
func TestGenerateAggregateWithCommands(t *testing.T) {
	dir := t.TempDir()

	fields := parseFields("id:uint64,name:string,email:string")
	data := buildAggregateData("order", "github.com/example/myservice", fields, "Place,Cancel,Ship", "email")

	if err := generateAggregate(data, dir); err != nil {
		t.Fatalf("generateAggregate: %v", err)
	}

	// 验证 aggregate.go 包含业务命令方法
	content, err := os.ReadFile(filepath.Join(dir, "domain/order/aggregate.go"))
	if err != nil {
		t.Fatalf("read aggregate.go: %v", err)
	}
	aggStr := string(content)
	if !contains(aggStr, "func (o *Order) Place()") {
		t.Error("aggregate.go should contain Place() method")
	}
	if !contains(aggStr, "func (o *Order) Cancel()") {
		t.Error("aggregate.go should contain Cancel() method")
	}
	if !contains(aggStr, "func (o *Order) Ship()") {
		t.Error("aggregate.go should contain Ship() method")
	}
	if !contains(aggStr, "ReconstructOrder") {
		t.Error("aggregate.go should contain ReconstructOrder")
	}

	// 验证 event.go 包含业务事件
	content, err = os.ReadFile(filepath.Join(dir, "domain/order/event.go"))
	if err != nil {
		t.Fatalf("read event.go: %v", err)
	}
	evtStr := string(content)
	if !contains(evtStr, "OrderPlacedEvent") {
		t.Error("event.go should contain OrderPlacedEvent")
	}
	if !contains(evtStr, "OrderCancelledEvent") {
		t.Error("event.go should contain OrderCancelledEvent")
	}
	if !contains(evtStr, "OrderShippedEvent") {
		t.Error("event.go should contain OrderShippedEvent")
	}

	// 验证 command.go 包含业务命令[非 UpdateCommand]
	content, err = os.ReadFile(filepath.Join(dir, "domain/order/command.go"))
	if err != nil {
		t.Fatalf("read command.go: %v", err)
	}
	cmdStr := string(content)
	if !contains(cmdStr, "PlaceOrderCommand") {
		t.Error("command.go should contain PlaceOrderCommand")
	}
	if !contains(cmdStr, "CancelOrderCommand") {
		t.Error("command.go should contain CancelOrderCommand")
	}
	if !contains(cmdStr, "ShipOrderCommand") {
		t.Error("command.go should contain ShipOrderCommand")
	}
	if contains(cmdStr, "UpdateOrderCommand") {
		t.Error("command.go should NOT contain UpdateOrderCommand when commands are specified")
	}

	// 验证 repository.go 包含 FindByEmail
	content, err = os.ReadFile(filepath.Join(dir, "domain/order/repository.go"))
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	repoStr := string(content)
	if !contains(repoStr, "FindByEmail") {
		t.Error("repository.go should contain FindByEmail for unique field")
	}

	// 验证 service.go 包含业务命令处理器[非 HandleUpdate]
	content, err = os.ReadFile(filepath.Join(dir, "application/order/service.go"))
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	svcStr := string(content)
	if !contains(svcStr, "HandlePlace") {
		t.Error("service.go should contain HandlePlace")
	}
	if !contains(svcStr, "HandleCancel") {
		t.Error("service.go should contain HandleCancel")
	}
	if !contains(svcStr, "HandleShip") {
		t.Error("service.go should contain HandleShip")
	}
	if contains(svcStr, "HandleUpdate") {
		t.Error("service.go should NOT contain HandleUpdate when commands are specified")
	}
}

// TestTemplateRendering 测试模板渲染各种输入.
func TestTemplateRendering(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	tests := []struct {
		name string
		data ProjectData
	}{
		{
			name: "minimal",
			data: ProjectData{Name: "minimal", Module: "github.com/test/minimal"},
		},
		{
			name: "full",
			data: func() ProjectData {
				infraDefs := parseInfra("mysql,redis")
				return ProjectData{
					Name: "full", Module: "github.com/test/full",
					WithGRPC: true, Infra: infraDefs, AllComponents: infraDefs,
				}
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := generateProject(tt.data); err != nil {
				t.Errorf("generateProject(%s): %v", tt.name, err)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestGenerateDockerfile 测试 Dockerfile 生成.
func TestGenerateDockerfile(t *testing.T) {
	dir := t.TempDir()

	data := DockerData{
		Name: "myservice",
		Port: "3000",
	}

	if err := generateDockerfile(data, dir); err != nil {
		t.Fatalf("generateDockerfile: %v", err)
	}

	path := filepath.Join(dir, "Dockerfile")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Dockerfile does not exist")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}

	got := string(content)
	if !contains(got, "EXPOSE 3000") {
		t.Error("Dockerfile should contain EXPOSE 3000")
	}
	if !contains(got, "golang:1.26-alpine") {
		t.Error("Dockerfile should contain golang:1.26-alpine builder image")
	}
	if !contains(got, "alpine:3.21") {
		t.Error("Dockerfile should contain alpine:3.21 runtime image")
	}
	if !contains(got, "CGO_ENABLED=0") {
		t.Error("Dockerfile should contain CGO_ENABLED=0")
	}
}

// TestGenerateJustfile 测试 justfile 生成.
func TestGenerateJustfile(t *testing.T) {
	dir := t.TempDir()

	data := JustfileData{
		Name:   "myservice",
		Module: "github.com/example/myservice",
	}

	if err := generateJustfile(data, dir); err != nil {
		t.Fatalf("generateJustfile: %v", err)
	}

	path := filepath.Join(dir, "justfile")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("justfile does not exist")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read justfile: %v", err)
	}

	got := string(content)
	recipes := []string{"build:", "run:", "test:", "lint:", "proto:", "wire:", "docker:", "clean:"}
	for _, recipe := range recipes {
		if !contains(got, recipe) {
			t.Errorf("justfile should contain recipe %q", recipe)
		}
	}
	if !contains(got, "myservice") {
		t.Error("justfile should contain service name myservice")
	}
}

// TestNewProjectWithWire 测试项目始终生成 Wire DI 文件.
func TestNewProjectWithWire(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	infraDefs := parseInfra("mysql,redis")
	data := ProjectData{
		Name:          "wireproject",
		Module:        "github.com/example/wireproject",
		WithGRPC:      true,
		Infra:         infraDefs,
		AllComponents: infraDefs,
	}

	if err := generateProject(data); err != nil {
		t.Fatalf("generateProject: %v", err)
	}

	// 验证 wire.go 文件存在[Wire 始终开启]
	wirePath := filepath.Join(dir, "wireproject/cmd/server/wire.go")
	if _, err := os.Stat(wirePath); os.IsNotExist(err) {
		t.Fatal("wire.go should always exist")
	}

	content, err := os.ReadFile(wirePath)
	if err != nil {
		t.Fatalf("read wire.go: %v", err)
	}

	got := string(content)
	if !contains(got, "wireinject") {
		t.Error("wire.go should contain wireinject build tag")
	}
	if !contains(got, "wire.Build") {
		t.Error("wire.go should contain wire.Build call")
	}
	if !contains(got, "wire.NewSet") {
		t.Error("wire.go should contain wire.NewSet definition")
	}
	if !contains(got, "server.NewGRPC") {
		t.Error("wire.go should contain server.NewGRPC provider when WithGRPC is true")
	}

	// 验证 provider.go 存在
	providerPath := filepath.Join(dir, "wireproject/cmd/server/provider.go")
	if _, err := os.Stat(providerPath); os.IsNotExist(err) {
		t.Fatal("provider.go should exist")
	}

	// 验证 config.go 存在
	configPath := filepath.Join(dir, "wireproject/cmd/server/config.go")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config.go should exist")
	}
}

// TestNewProjectMinimal 测试无 infra 的最小项目也生成 Wire 文件.
func TestNewProjectMinimal(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	data := ProjectData{
		Name:   "nowireproject",
		Module: "github.com/example/nowireproject",
	}

	if err := generateProject(data); err != nil {
		t.Fatalf("generateProject: %v", err)
	}

	// Wire 始终生成，即使无 infra
	wirePath := filepath.Join(dir, "nowireproject/cmd/server/wire.go")
	if _, err := os.Stat(wirePath); os.IsNotExist(err) {
		t.Error("wire.go should always exist (Wire is always on)")
	}
}

// TestIsMonorepo 测试 monorepo 检测.
func TestIsMonorepo(t *testing.T) {
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	// 不存在 services/ 目录时应返回 false
	dir := t.TempDir()
	os.Chdir(dir)
	if isMonorepo() {
		t.Error("isMonorepo() should return false without services/ directory")
	}

	// 存在 services/ 目录时应返回 true
	if err := os.Mkdir(filepath.Join(dir, "services"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !isMonorepo() {
		t.Error("isMonorepo() should return true with services/ directory")
	}
}

// TestGenAggregateInMonorepo 测试 monorepo 下聚合生成路径.
func TestGenAggregateInMonorepo(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	// 创建 services/ 目录模拟 monorepo
	if err := os.Mkdir(filepath.Join(dir, "services"), 0o755); err != nil {
		t.Fatal(err)
	}

	fields := parseFields("id:uint64,name:string")
	data := buildAggregateData("order", "github.com/example/monorepo", fields, "", "")

	if err := generateAggregate(data, dir); err != nil {
		t.Fatalf("generateAggregate: %v", err)
	}

	// 验证文件仍然生成在 domain/ 和 application/ 下
	expectedFiles := []string{
		"domain/order/aggregate.go",
		"domain/order/event.go",
		"domain/order/repository.go",
		"application/order/service.go",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %q does not exist in monorepo", f)
		}
	}
}

// TestNewMonorepo 测试默认 monorepo 项目生成.
func TestNewMonorepo(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	data := ProjectData{
		Name:   "myplatform",
		Module: "github.com/example/myplatform",
	}

	if err := generateMonorepo(data); err != nil {
		t.Fatalf("generateMonorepo: %v", err)
	}

	// 验证 monorepo 结构文件存在
	expectedFiles := []string{
		"myplatform/go.mod",
		"myplatform/justfile",
		"myplatform/.gitignore",
		"myplatform/README.md",
		"myplatform/domain/.gitkeep",
		"myplatform/application/.gitkeep",
		"myplatform/services/.gitkeep",
		"myplatform/api/.gitkeep",
		"myplatform/infrastructure/.gitkeep",
		"myplatform/deploy/docker/.gitkeep",
		"myplatform/deploy/k8s/.gitkeep",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected monorepo file %q does not exist", f)
		}
	}

	// 验证 go.mod 内容
	content, err := os.ReadFile(filepath.Join(dir, "myplatform/go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if got := string(content); !contains(got, "module github.com/example/myplatform") {
		t.Errorf("go.mod missing module path, got:\n%s", got)
	}

	// 验证 justfile 包含 monorepo 配方
	content, err = os.ReadFile(filepath.Join(dir, "myplatform/justfile"))
	if err != nil {
		t.Fatalf("read justfile: %v", err)
	}
	justStr := string(content)
	if !contains(justStr, "build service") {
		t.Error("monorepo justfile should contain 'build service' recipe")
	}
	if !contains(justStr, "build-all") {
		t.Error("monorepo justfile should contain 'build-all' recipe")
	}
	if !contains(justStr, "run service") {
		t.Error("monorepo justfile should contain 'run service' recipe")
	}

	// 验证 standalone 项目文件不应存在
	standalonePaths := []string{
		"myplatform/cmd/server/main.go",
		"myplatform/Dockerfile",
		"myplatform/config.yaml",
	}
	for _, f := range standalonePaths {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("standalone file %q should not exist in monorepo", f)
		}
	}
}

// TestNewStandalone 测试 standalone 项目生成.
func TestNewStandalone(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	infraDefs := parseInfra("mysql,redis")
	data := ProjectData{
		Name:          "myservice",
		Module:        "github.com/example/myservice",
		WithGRPC:      true,
		Infra:         infraDefs,
		AllComponents: infraDefs,
	}

	if err := generateProject(data); err != nil {
		t.Fatalf("generateProject: %v", err)
	}

	// 验证 standalone 结构文件存在
	expectedFiles := []string{
		"myservice/go.mod",
		"myservice/justfile",
		"myservice/Dockerfile",
		"myservice/.gitignore",
		"myservice/config.yaml",
		"myservice/cmd/server/main.go",
		"myservice/cmd/server/wire.go",
		"myservice/cmd/server/provider.go",
		"myservice/cmd/server/config.go",
		"myservice/internal/server/http.go",
		"myservice/internal/server/grpc.go",
		"myservice/internal/service/service.go",
		"myservice/api/third_party/google/api/annotations.proto",
		"myservice/deploy/docker/infra/docker-compose.yaml",
		"myservice/deploy/docker/app/docker-compose.yaml",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected standalone file %q does not exist", f)
		}
	}

	// 验证 monorepo 目录不应存在
	monorepoPaths := []string{
		"myservice/services",
		"myservice/domain",
		"myservice/application",
		"myservice/infrastructure",
	}
	for _, f := range monorepoPaths {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("monorepo directory %q should not exist in standalone project", f)
		}
	}
}
