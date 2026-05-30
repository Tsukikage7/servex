// openapi/operation_test.go
package openapi

import "testing"

func TestBuilder_GET(t *testing.T) {
	op := GET("/users/{id}").
		Summary("获取用户").
		Description("根据 ID 获取用户详情").
		Tags("users").
		OperationID("getUser").
		Response(struct {
			Name string `json:"name"`
		}{}).
		Build()

	if op.Method != "GET" {
		t.Errorf("method = %s", op.Method)
	}
	if op.Path != "/users/{id}" {
		t.Errorf("path = %s", op.Path)
	}
	if op.Summary != "获取用户" {
		t.Errorf("summary = %s", op.Summary)
	}
	if len(op.Tags) != 1 || op.Tags[0] != "users" {
		t.Errorf("tags = %v", op.Tags)
	}
}

func TestBuilder_POST(t *testing.T) {
	type createReq struct {
		Name string `json:"name" validate:"required"`
	}
	type createResp struct {
		ID string `json:"id"`
	}

	op := POST("/users").
		Summary("创建用户").
		Tags("users").
		Request(createReq{}).
		Response(createResp{}).
		Build()

	if op.Method != "POST" {
		t.Errorf("method = %s", op.Method)
	}
	if op.RequestType == nil {
		t.Error("request type should be set")
	}
	if op.ResponseType == nil {
		t.Error("response type should be set")
	}
}

func TestBuilder_MultipleTags(t *testing.T) {
	op := GET("/x").Tags("a", "b", "c").Build()
	if len(op.Tags) != 3 {
		t.Errorf("tags count = %d", len(op.Tags))
	}
}
