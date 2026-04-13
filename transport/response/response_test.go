package response_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tsukikage7/servex/v2/transport/response"
	"github.com/Tsukikage7/servex/v2/xutil/pagination"
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
	err := response.NewErrorWithMessage(response.CodeNotFound, "user not found")
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

func TestBusinessError(t *testing.T) {
	t.Run("error message", func(t *testing.T) {
		err := response.NewError(response.CodeNotFound)
		if err.Error() != response.CodeNotFound.Message {
			t.Errorf("expected %q, got %q", response.CodeNotFound.Message, err.Error())
		}
	})

	t.Run("custom message", func(t *testing.T) {
		err := response.NewErrorWithMessage(response.CodeNotFound, "custom msg")
		if err.Error() != "custom msg" {
			t.Errorf("expected 'custom msg', got %q", err.Error())
		}
		if err.GetMessage() != "custom msg" {
			t.Errorf("GetMessage expected 'custom msg', got %q", err.GetMessage())
		}
	})

	t.Run("with cause", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := response.NewErrorWithCause(response.CodeInternal, cause)
		if !errors.Is(err, cause) {
			t.Error("should unwrap to cause")
		}
		if err.Unwrap() != cause {
			t.Error("Unwrap should return cause")
		}
	})

	t.Run("full", func(t *testing.T) {
		cause := errors.New("db error")
		err := response.NewErrorFull(response.CodeDatabaseError, "query failed", cause)
		if err.GetCode() != response.CodeDatabaseError {
			t.Error("GetCode mismatch")
		}
		if err.Error() != "query failed: db error" {
			t.Errorf("unexpected error: %q", err.Error())
		}
	})

	t.Run("wrap", func(t *testing.T) {
		cause := errors.New("timeout")
		err := response.Wrap(response.CodeTimeout, cause)
		if !response.IsBusinessError(err) {
			t.Error("should be business error")
		}
	})

	t.Run("WrapWithMessage", func(t *testing.T) {
		cause := errors.New("timeout")
		err := response.WrapWithMessage(response.CodeTimeout, "custom wrap", cause)
		if err.GetMessage() != "custom wrap" {
			t.Errorf("expected 'custom wrap', got %q", err.GetMessage())
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

	t.Run("business error", func(t *testing.T) {
		err := response.NewError(response.CodeNotFound)
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
		err := response.NewErrorWithMessage(response.CodeInternal, "sensitive info")
		msg := response.ExtractMessage(err)
		if msg != response.CodeInternal.Message {
			t.Errorf("expected generic internal error message, got %q", msg)
		}
	})

	t.Run("business error shows detail", func(t *testing.T) {
		err := response.NewErrorWithMessage(response.CodeInvalidParam, "name is required")
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
		err := response.NewErrorFull(response.CodeInternal, "oops", errors.New("db fail"))
		msg := response.ExtractMessageUnsafe(err)
		if msg != "oops: db fail" {
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

	t.Run("Is", func(t *testing.T) {
		if !errors.Is(response.CodeNotFound, response.CodeNotFound) {
			t.Error("CodeNotFound should Is CodeNotFound")
		}
		if errors.Is(response.CodeNotFound, response.CodeInternal) {
			t.Error("CodeNotFound should not Is CodeInternal")
		}
	})

	t.Run("NewCode", func(t *testing.T) {
		custom := response.NewCode(99001, "custom error", 418, 0)
		if custom.Num != 99001 {
			t.Error("custom code num mismatch")
		}
	})
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
	w := httptest.NewRecorder()
	bizErr := response.NewError(response.CodeForbidden)
	err := response.WriteError(w, bizErr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
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

	t.Run("GRPCCodeToHTTP", func(t *testing.T) {
		httpCode := response.GRPCCodeToHTTP(5) // NotFound
		if httpCode != http.StatusNotFound {
			t.Errorf("expected 404, got %d", httpCode)
		}
	})

	t.Run("HTTPToGRPCCode roundtrip", func(t *testing.T) {
		tests := []struct {
			http int
		}{
			{200}, {400}, {401}, {403}, {404}, {408}, {409}, {429}, {500}, {501}, {503},
		}
		for _, tt := range tests {
			code := response.HTTPToGRPCCode(tt.http)
			if code == 0 && tt.http != 200 {
				// OK is expected for 200
			}
			_ = code // Just verify no panic
		}
	})
}

func TestIsBusinessError(t *testing.T) {
	if response.IsBusinessError(errors.New("plain")) {
		t.Error("plain error should not be business error")
	}
	if !response.IsBusinessError(response.NewError(response.CodeNotFound)) {
		t.Error("business error should be detected")
	}
}

func TestAsBusinessError(t *testing.T) {
	plain := errors.New("plain")
	if response.AsBusinessError(plain) != nil {
		t.Error("should return nil for plain error")
	}
	bizErr := response.NewError(response.CodeNotFound)
	got := response.AsBusinessError(bizErr)
	if got == nil {
		t.Error("should return business error")
	}
}
