package echoserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/Tsukikage7/servex/v2/transport/response"
)

type testReq struct {
	Name string `json:"name"`
}

type testResp struct {
	Greeting string `json:"greeting"`
}

func TestHandle(t *testing.T) {
	e := echo.New()

	handler := Handle(func(ctx context.Context, req testReq) (*testResp, error) {
		return &testResp{Greeting: "Hello, " + req.Name}, nil
	})

	body := `{"name":"World"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler(c); err != nil {
		t.Fatalf("Handle error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp response.Response[*testResp]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("response code = %d, want 0", resp.Code)
	}
	if resp.Data.Greeting != "Hello, World" {
		t.Errorf("greeting = %q", resp.Data.Greeting)
	}
}

func TestHandleBindError(t *testing.T) {
	e := echo.New()

	handler := Handle(func(ctx context.Context, req testReq) (*testResp, error) {
		return &testResp{}, nil
	})

	// Send invalid JSON.
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{invalid"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler(c); err != nil {
		t.Fatalf("Handle error: %v", err)
	}
	// Should return error response with invalid param code.
	var resp response.Response[any]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Code == 0 {
		t.Error("expected non-zero error code for invalid input")
	}
}

func TestHandleWith(t *testing.T) {
	e := echo.New()

	handler := HandleWith(
		func(c echo.Context) (testReq, error) {
			return testReq{Name: c.QueryParam("name")}, nil
		},
		func(ctx context.Context, req testReq) (*testResp, error) {
			return &testResp{Greeting: "Hi, " + req.Name}, nil
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/?name=Test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler(c); err != nil {
		t.Fatalf("HandleWith error: %v", err)
	}

	var resp response.Response[*testResp]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Data.Greeting != "Hi, Test" {
		t.Errorf("greeting = %q", resp.Data.Greeting)
	}
}

func TestHandleServiceError(t *testing.T) {
	e := echo.New()

	handler := Handle(func(ctx context.Context, req testReq) (*testResp, error) {
		return nil, response.CodeNotFound
	})

	body := `{"name":"err"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler(c); err != nil {
		t.Fatalf("Handle error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

type validatableReq struct {
	Name string `json:"name"`
}

func (r *validatableReq) Validate() error {
	if r.Name == "" {
		return response.CodeMissingParam
	}
	return nil
}

func TestHandleValidation(t *testing.T) {
	e := echo.New()

	handler := Handle(func(ctx context.Context, req validatableReq) (*testResp, error) {
		return &testResp{Greeting: req.Name}, nil
	})

	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler(c); err != nil {
		t.Fatalf("Handle error: %v", err)
	}

	var resp response.Response[any]
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp.Code == 0 {
		t.Error("expected validation error code")
	}
}

func TestWrapEnvelopeAlreadyWrapped(t *testing.T) {
	envelope := response.OK("already wrapped")
	result := wrapEnvelope(envelope)
	// Should return the same envelope without double-wrapping.
	if _, ok := result.(response.Envelope); !ok {
		t.Error("expected result to still be Envelope")
	}
}
