package timeout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRemaining(t *testing.T) {
	t.Run("no deadline", func(t *testing.T) {
		ctx := t.Context()
		remaining, ok := Remaining(ctx)
		if ok {
			t.Error("expected no deadline")
		}
		if remaining != 0 {
			t.Errorf("expected 0, got %v", remaining)
		}
	})

	t.Run("with deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		remaining, ok := Remaining(ctx)
		if !ok {
			t.Error("expected deadline")
		}
		if remaining <= 0 || remaining > 5*time.Second {
			t.Errorf("unexpected remaining time: %v", remaining)
		}
	})
}

func TestCascade(t *testing.T) {
	t.Run("no parent deadline", func(t *testing.T) {
		ctx := t.Context()
		newCtx, cancel := Cascade(ctx, 5*time.Second)
		defer cancel()

		deadline, ok := newCtx.Deadline()
		if !ok {
			t.Error("expected deadline")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > 5*time.Second {
			t.Errorf("unexpected remaining time: %v", remaining)
		}
	})

	t.Run("parent deadline shorter", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		newCtx, newCancel := Cascade(ctx, 5*time.Second)
		defer newCancel()

		remaining, _ := Remaining(newCtx)
		if remaining > 2*time.Second {
			t.Errorf("expected <= 2s, got %v", remaining)
		}
	})

	t.Run("parent deadline longer", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()

		newCtx, newCancel := Cascade(ctx, 5*time.Second)
		defer newCancel()

		remaining, _ := Remaining(newCtx)
		if remaining > 5*time.Second {
			t.Errorf("expected <= 5s, got %v", remaining)
		}
	})

	t.Run("zero timeout", func(t *testing.T) {
		ctx := t.Context()
		newCtx, cancel := Cascade(ctx, 0)
		defer cancel()

		if newCtx != ctx {
			t.Error("expected same context")
		}
	})
}

func TestShrinkBy(t *testing.T) {
	t.Run("no deadline", func(t *testing.T) {
		ctx := t.Context()
		newCtx, cancel := ShrinkBy(ctx, time.Second)
		defer cancel()

		if newCtx != ctx {
			t.Error("expected same context")
		}
	})

	t.Run("with buffer", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		newCtx, newCancel := ShrinkBy(ctx, time.Second)
		defer newCancel()

		remaining, _ := Remaining(newCtx)
		if remaining > 4*time.Second {
			t.Errorf("expected <= 4s, got %v", remaining)
		}
	})

	t.Run("buffer exceeds remaining", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()

		newCtx, newCancel := ShrinkBy(ctx, 2*time.Second)
		defer newCancel()

		select {
		case <-newCtx.Done():
			// expected
		default:
			t.Error("expected context to be cancelled")
		}
	})
}

func TestWithTimeout(t *testing.T) {
	t.Run("valid timeout", func(t *testing.T) {
		ctx, cancel, err := WithTimeout(t.Context(), time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		defer cancel()

		_, ok := ctx.Deadline()
		if !ok {
			t.Error("expected deadline")
		}
	})

	t.Run("invalid timeout", func(t *testing.T) {
		_, _, err := WithTimeout(t.Context(), 0)
		if !errors.Is(err, ErrInvalidTimeout) {
			t.Errorf("expected ErrInvalidTimeout, got %v", err)
		}
	})
}

func TestEndpointMiddleware(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		endpoint := func(ctx context.Context, request any) (any, error) {
			return "ok", nil
		}

		wrapped := EndpointMiddleware(time.Second)(endpoint)
		resp, err := wrapped(t.Context(), nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp != "ok" {
			t.Errorf("unexpected response: %v", resp)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		endpoint := func(ctx context.Context, request any) (any, error) {
			time.Sleep(500 * time.Millisecond)
			return "ok", nil
		}

		wrapped := EndpointMiddleware(100 * time.Millisecond)(endpoint)
		_, err := wrapped(t.Context(), nil)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("panic on invalid timeout", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		EndpointMiddleware(0)
	})
}

func TestEndpointMiddlewareWithFallback(t *testing.T) {
	t.Run("success no fallback", func(t *testing.T) {
		endpoint := func(ctx context.Context, request any) (any, error) {
			return "ok", nil
		}
		fallback := func(ctx context.Context, request any) (any, error) {
			return "fallback", nil
		}

		wrapped := EndpointMiddlewareWithFallback(time.Second, fallback)(endpoint)
		resp, err := wrapped(t.Context(), nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp != "ok" {
			t.Errorf("expected ok, got %v", resp)
		}
	})

	t.Run("timeout uses fallback", func(t *testing.T) {
		endpoint := func(ctx context.Context, request any) (any, error) {
			time.Sleep(500 * time.Millisecond)
			return "ok", nil
		}
		fallback := func(ctx context.Context, request any) (any, error) {
			return "fallback", nil
		}

		wrapped := EndpointMiddlewareWithFallback(100*time.Millisecond, fallback)(endpoint)
		resp, err := wrapped(t.Context(), nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp != "fallback" {
			t.Errorf("expected fallback, got %v", resp)
		}
	})
}

func TestHTTPMiddleware(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})

		wrapped := HTTPMiddleware(time.Second)(handler)
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(500 * time.Millisecond):
				w.WriteHeader(http.StatusOK)
			}
		})

		wrapped := HTTPMiddleware(100 * time.Millisecond)(handler)
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("panic on invalid timeout", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		HTTPMiddleware(0)
	})
}

func TestHTTPTimeoutHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := HTTPTimeoutHandler(handler, time.Second, "timeout")
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("timeout with message", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skip timeout race-sensitive test in short mode")
		}
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(500 * time.Millisecond):
				w.WriteHeader(http.StatusOK)
			}
		})

		wrapped := HTTPTimeoutHandler(handler, 100*time.Millisecond, "custom timeout message")
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
		if rec.Body.String() != "custom timeout message" {
			t.Errorf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("late handler write is ignored after timeout", func(t *testing.T) {
		handlerDone := make(chan struct{})
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer close(handlerDone)
			<-r.Context().Done()
			_, _ = w.Write([]byte("late body"))
		})

		wrapped := HTTPTimeoutHandler(handler, 10*time.Millisecond, "timeout")
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)
		<-handlerDone

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}
		if rec.Body.String() != "timeout" {
			t.Errorf("late handler write polluted response body: %q", rec.Body.String())
		}
	})
}

func TestUnaryServerInterceptor(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		handler := func(ctx context.Context, req any) (any, error) {
			return "ok", nil
		}

		interceptor := UnaryServerInterceptor(time.Second)
		resp, err := interceptor(
			t.Context(),
			nil,
			&grpc.UnaryServerInfo{FullMethod: "/test/Method"},
			handler,
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if resp != "ok" {
			t.Errorf("unexpected response: %v", resp)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		handler := func(ctx context.Context, req any) (any, error) {
			time.Sleep(500 * time.Millisecond)
			return "ok", nil
		}

		interceptor := UnaryServerInterceptor(100 * time.Millisecond)
		_, err := interceptor(
			t.Context(),
			nil,
			&grpc.UnaryServerInfo{FullMethod: "/test/Method"},
			handler,
		)
		if err == nil {
			t.Error("expected error")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Error("expected grpc status error")
		}
		if st.Code() != codes.DeadlineExceeded {
			t.Errorf("expected DeadlineExceeded, got %v", st.Code())
		}
	})

	t.Run("panic on invalid timeout", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		UnaryServerInterceptor(0)
	})
}

func TestStreamServerInterceptor(t *testing.T) {
	t.Run("panic on invalid timeout", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		StreamServerInterceptor(0)
	})
}

func TestUnaryClientInterceptor(t *testing.T) {
	t.Run("panic on invalid timeout", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		UnaryClientInterceptor(0)
	})
}

func TestStreamClientInterceptor(t *testing.T) {
	t.Run("panic on invalid timeout", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic")
			}
		}()
		StreamClientInterceptor(0)
	})
}
