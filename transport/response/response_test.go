package response_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	servexerr "github.com/Tsukikage7/servex/v2/errors"
	"github.com/Tsukikage7/servex/v2/transport/response"
	"github.com/Tsukikage7/servex/v2/xutil/pagination"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestOK(t *testing.T) {
	resp := response.OK("hello")
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
	if resp.Data != "hello" {
		t.Errorf("expected data 'hello', got %v", resp.Data)
	}
	if !resp.IsSuccess() {
		t.Error("should be success")
	}
}

func TestOKWithMessage(t *testing.T) {
	resp := response.OKWithMessage(42, "custom message")
	if resp.Message != "custom message" {
		t.Errorf("expected custom message, got %s", resp.Message)
	}
	if resp.Data != 42 {
		t.Errorf("expected data 42, got %v", resp.Data)
	}
}

func TestFail(t *testing.T) {
	resp := response.Fail[string](response.CodeNotFound)
	if resp.Code != response.CodeNotFound.Num {
		t.Errorf("expected code %d, got %d", response.CodeNotFound.Num, resp.Code)
	}
	if resp.IsSuccess() {
		t.Error("should not be success")
	}
}

func TestFailWithMessage(t *testing.T) {
	resp := response.FailWithMessage[string](response.CodeInvalidParam, "bad input")
	if resp.Message != "bad input" {
		t.Errorf("expected 'bad input', got %s", resp.Message)
	}
}

func TestFailWithError(t *testing.T) {
	err := response.CodeNotFound.ToError().WithMessage("user not found")
	resp := response.FailWithError[string](err)
	if resp.Code != response.CodeNotFound.Num {
		t.Errorf("expected code %d, got %d", response.CodeNotFound.Num, resp.Code)
	}
	if resp.Message != "user not found" {
		t.Errorf("expected 'user not found', got %s", resp.Message)
	}
}

func TestPaged(t *testing.T) {
	result := pagination.Result[string]{
		Items:    []string{"a", "b"},
		Page:     1,
		PageSize: 10,
		Total:    2,
	}
	resp := response.Paged(result)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Data))
	}
	if resp.Pagination == nil {
		t.Fatal("pagination should not be nil")
	}
	if resp.Pagination.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Pagination.Total)
	}
	if !resp.IsSuccess() {
		t.Error("should be success")
	}
}

func TestPagedFail(t *testing.T) {
	resp := response.PagedFail[string](response.CodeInternal)
	if resp.Code != response.CodeInternal.Num {
		t.Errorf("expected code %d, got %d", response.CodeInternal.Num, resp.Code)
	}
	if resp.IsSuccess() {
		t.Error("should not be success")
	}
}

func TestPagedFailWithMessage(t *testing.T) {
	resp := response.PagedFailWithMessage[string](response.CodeInternal, "db error")
	if resp.Message != "db error" {
		t.Errorf("expected 'db error', got %s", resp.Message)
	}
}

func TestServexError(t *testing.T) {
	t.Run("error message", func(t *testing.T) {
		err := response.CodeNotFound.ToError()
		want := "[" + "40001" + "] error.not_found: 资源不存在"
		if err.Error() != want {
			t.Errorf("expected %q, got %q", want, err.Error())
		}
	})

	t.Run("custom message", func(t *testing.T) {
		err := response.CodeNotFound.ToError().WithMessage("custom msg")
		if err.Message != "custom msg" {
			t.Errorf("expected 'custom msg', got %q", err.Error())
		}
	})

	t.Run("with cause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := response.CodeInternal.ToError().WithCause(cause)
		if !errors.Is(err, cause) {
			t.Error("should unwrap to cause")
		}
	})

	t.Run("full", func(t *testing.T) {
		cause := errors.New("db error")
		err := response.CodeDatabaseError.ToError().WithMessage("query failed").WithCause(cause)
		if err.Code != response.CodeDatabaseError.Num {
			t.Error("Code mismatch")
		}
		if err.Error() != "[50003] error.database: query failed: db error" {
			t.Errorf("unexpected error: %q", err.Error())
		}
	})

	t.Run("wrap", func(t *testing.T) {
		cause := errors.New("timeout")
		err := response.CodeTimeout.ToError().WithCause(cause)
		if !errors.Is(err, cause) {
			t.Error("should unwrap cause")
		}
	})

	t.Run("WrapWithMessage", func(t *testing.T) {
		cause := errors.New("timeout")
		err := response.CodeTimeout.ToError().WithMessage("custom wrap").WithCause(cause)
		if err.Message != "custom wrap" {
			t.Errorf("expected 'custom wrap', got %q", err.Message)
		}
	})
}

func TestExtractCode(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		code := response.ExtractCode(nil)
		if code != response.CodeSuccess {
			t.Error("nil error should return CodeSuccess")
		}
	})

	t.Run("servex error", func(t *testing.T) {
		err := response.CodeNotFound.ToError()
		code := response.ExtractCode(err)
		if code.Num != response.CodeNotFound.Num {
			t.Errorf("expected %d, got %d", response.CodeNotFound.Num, code.Num)
		}
	})

	t.Run("plain error", func(t *testing.T) {
		code := response.ExtractCode(errors.New("unknown"))
		if code.Num != response.CodeInternal.Num {
			t.Errorf("expected CodeInternal, got %d", code.Num)
		}
	})
}

func TestExtractMessage(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		msg := response.ExtractMessage(nil)
		if msg != response.CodeSuccess.Message {
			t.Errorf("expected success message, got %q", msg)
		}
	})

	t.Run("internal error hides detail", func(t *testing.T) {
		err := response.CodeInternal.ToError().WithMessage("sensitive info")
		msg := response.ExtractMessage(err)
		if msg != response.CodeInternal.Message {
			t.Errorf("expected generic internal error message, got %q", msg)
		}
	})

	t.Run("business error shows detail", func(t *testing.T) {
		err := response.CodeInvalidParam.ToError().WithMessage("name is required")
		msg := response.ExtractMessage(err)
		if msg != "name is required" {
			t.Errorf("expected 'name is required', got %q", msg)
		}
	})
}

func TestExtractMessageUnsafe(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		msg := response.ExtractMessageUnsafe(nil)
		if msg != response.CodeSuccess.Message {
			t.Errorf("expected success message, got %q", msg)
		}
	})

	t.Run("with cause", func(t *testing.T) {
		err := response.CodeInternal.ToError().WithMessage("oops").WithCause(errors.New("db fail"))
		msg := response.ExtractMessageUnsafe(err)
		if msg != "[50001] error.internal: oops: db fail" {
			t.Errorf("expected full message, got %q", msg)
		}
	})

	t.Run("plain error", func(t *testing.T) {
		msg := response.ExtractMessageUnsafe(errors.New("raw error"))
		if msg != "raw error" {
			t.Errorf("expected 'raw error', got %q", msg)
		}
	})
}

func TestCode(t *testing.T) {
	t.Run("Error interface", func(t *testing.T) {
		if response.CodeNotFound.Error() != response.CodeNotFound.Message {
			t.Error("Code.Error() should return Message")
		}
	})

	t.Run("WithMessage", func(t *testing.T) {
		code := response.CodeNotFound.WithMessage("custom")
		if code.Message != "custom" {
			t.Error("WithMessage should change message")
		}
		// Original should be unchanged
		if response.CodeNotFound.Message == "custom" {
			t.Error("should not modify original")
		}
	})

	t.Run("WithHTTPStatus", func(t *testing.T) {
		code := response.CodeInvalidParam.
			WithHTTPStatus(http.StatusUnprocessableEntity)

		if code.HTTPStatus() != http.StatusUnprocessableEntity {
			t.Error("WithHTTPStatus should change HTTPStatus")
		}
		if response.CodeInvalidParam.HTTPStatus() == http.StatusUnprocessableEntity {
			t.Error("should not modify original")
		}
	})

	t.Run("Is", func(t *testing.T) {
		if !errors.Is(response.CodeNotFound, response.CodeNotFound) {
			t.Error("CodeNotFound should Is CodeNotFound")
		}
		if errors.Is(response.CodeNotFound, response.CodeInternal) {
			t.Error("CodeNotFound should not Is CodeInternal")
		}
	})

	t.Run("NewCodeWithKind", func(t *testing.T) {
		custom := response.NewCodeWithKind(
			40010,
			"error.user_banned",
			"账号已封禁",
			servexerr.KindPermissionDenied,
		)

		if custom.Num != 40010 {
			t.Error("custom code num mismatch")
		}
		if custom.Key != "error.user_banned" {
			t.Error("custom code key mismatch")
		}
		if custom.HTTPStatus() != http.StatusForbidden {
			t.Error("custom code HTTP status should be derived from kind")
		}
		if custom.GRPCCode() != codes.PermissionDenied {
			t.Error("custom code gRPC code should be derived from kind")
		}

		err := custom.ToError()
		if err.Kind != servexerr.KindPermissionDenied {
			t.Error("ToError should preserve semantic kind")
		}
	})
}

func TestCodeToErrorSetsKindWithoutLosingExactMapping(t *testing.T) {
	cases := []struct {
		name string
		code response.Code
		kind servexerr.Kind
	}{
		{"not found", response.CodeNotFound, servexerr.KindNotFound},
		{"timeout", response.CodeTimeout, servexerr.KindDeadlineExceeded},
		{"resource exhausted", response.CodeResourceExhausted, servexerr.KindResourceExhausted},
		{"not implemented", response.CodeNotImplemented, servexerr.KindNotImplemented},
		{"upstream keeps 502", response.CodeUpstreamError, servexerr.KindUnavailable},
		{"conflict", response.CodeConflict, servexerr.KindConflict},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.code.ToError()
			if err.Kind != tt.kind {
				t.Fatalf("Kind = %v, want %v", err.Kind, tt.kind)
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	err := response.WriteJSON(w, http.StatusOK, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("unexpected content-type: %s", w.Header().Get("Content-Type"))
	}
	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got["key"] != "value" {
		t.Errorf("expected value, got %s", got["key"])
	}
}

func TestWriteSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	err := response.WriteSuccess(w, "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWriteFail(t *testing.T) {
	w := httptest.NewRecorder()
	err := response.WriteFail(w, response.CodeNotFound)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestWriteError(t *testing.T) {
	t.Run("default message", func(t *testing.T) {
		w := httptest.NewRecorder()
		bizErr := response.CodeForbidden.ToError()
		err := response.WriteError(w, bizErr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("localized message", func(t *testing.T) {
		w := httptest.NewRecorder()
		bizErr := response.CodeForbidden.ToError()
		err := response.WriteError(w, bizErr, "en")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var body response.Response[any]
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Fatalf("decode error: %v", err)
		}
		if body.Message != "Forbidden" {
			t.Errorf("expected localized message, got %q", body.Message)
		}
	})
}

func TestWriteLocalizedError(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Language", "en")

	err := response.WriteLocalizedError(w, req, response.CodeNotFound.ToError())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body response.Response[any]
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Message != "Resource not found" {
		t.Errorf("expected localized message, got %q", body.Message)
	}
}

func TestGRPCConversions(t *testing.T) {
	t.Run("GRPCStatus nil", func(t *testing.T) {
		s := response.GRPCStatus(nil)
		if s.Code() != 0 {
			t.Errorf("nil err should be OK, got %v", s.Code())
		}
	})

	t.Run("FromGRPCError nil", func(t *testing.T) {
		code := response.FromGRPCError(nil)
		if code != response.CodeSuccess {
			t.Error("nil error should return CodeSuccess")
		}
	})

	t.Run("FromGRPCStatus nil", func(t *testing.T) {
		code := response.FromGRPCStatus(nil)
		if code != response.CodeSuccess {
			t.Error("nil status should return CodeSuccess")
		}
	})

	t.Run("plain grpc status maps to response codes", func(t *testing.T) {
		cases := []struct {
			grpc codes.Code
			want response.Code
		}{
			{codes.Canceled, response.CodeCanceled},
			{codes.Unknown, response.CodeUnknown},
			{codes.DeadlineExceeded, response.CodeTimeout},
			{codes.ResourceExhausted, response.CodeResourceExhausted},
			{codes.Unimplemented, response.CodeNotImplemented},
			{codes.Unavailable, response.CodeServiceUnavailable},
		}

		for _, tt := range cases {
			st := grpcstatus.New(tt.grpc, tt.want.Message)
			got := response.FromGRPCStatus(st)
			if got.Num != tt.want.Num {
				t.Errorf("%s: Num = %d, want %d", tt.grpc, got.Num, tt.want.Num)
			}
			if got.HTTPStatus() != tt.want.HTTPStatus() {
				t.Errorf("%s: HTTPStatus = %d, want %d", tt.grpc, got.HTTPStatus(), tt.want.HTTPStatus())
			}
			if got.GRPCCode() != tt.want.GRPCCode() {
				t.Errorf("%s: GRPCCode = %v, want %v", tt.grpc, got.GRPCCode(), tt.want.GRPCCode())
			}
		}
	})

	// 回归测试：同一 gRPC code (InvalidArgument) 对应多个业务 Code 时，
	// GRPCStatus → FromGRPCStatus 必须保留细粒度 Num，不能发生降级。
	t.Run("fine-grained code preserved through gRPC roundtrip", func(t *testing.T) {
		cases := []response.Code{
			response.CodeInvalidParam,       // 30001
			response.CodeMissingParam,       // 30002
			response.CodeValidationFailed,   // 30003
			response.CodeUnauthorized,       // 20001
			response.CodeTokenExpired,       // 20003
			response.CodeTokenInvalid,       // 20004
			response.CodeInternal,           // 50001
			response.CodeDatabaseError,      // 50003
			response.CodeServiceUnavailable, // 60001
			response.CodeUpstreamError,      // 60002
		}
		for _, want := range cases {
			err := want.ToError()
			st := response.GRPCStatus(err)
			got := response.FromGRPCStatus(st)
			if got.Num != want.Num {
				t.Errorf("code %d (%s): after gRPC roundtrip got Num=%d, want %d",
					want.Num, want.Key, got.Num, want.Num)
			}
			if got.HTTPStatus() != want.HTTPStatus() {
				t.Errorf("code %d: HTTP status got %d, want %d",
					want.Num, got.HTTPStatus(), want.HTTPStatus())
			}
		}
	})
}
