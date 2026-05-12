package httpclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/Tsukikage7/servex/v2/errors"
)

// Response HTTP 响应封装.
type Response struct {
	*http.Response
}

// JSON 将响应体解码为 JSON.
func (r *Response) JSON(v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// Text 将响应体读取为字符串.
func (r *Response) Text() (string, error) {
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Bytes 将响应体读取为字节切片.
func (r *Response) Bytes() ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

// CheckStatus 检查响应状态码是否为 2xx，否则返回错误.
func (r *Response) CheckStatus() error {
	if r.StatusCode >= 200 && r.StatusCode < 300 {
		return nil
	}
	return errors.New(
		r.StatusCode,
		fmt.Sprintf("http.%d", r.StatusCode),
		fmt.Sprintf("HTTP %d: %s", r.StatusCode, http.StatusText(r.StatusCode)),
	).WithKind(kindFromHTTPStatus(r.StatusCode)).
		WithMeta("http_status", strconv.Itoa(r.StatusCode))
}

func kindFromHTTPStatus(status int) errors.Kind {
	switch status {
	case http.StatusBadRequest:
		return errors.KindInvalidArgument
	case http.StatusUnauthorized:
		return errors.KindUnauthenticated
	case http.StatusForbidden:
		return errors.KindPermissionDenied
	case http.StatusNotFound:
		return errors.KindNotFound
	case http.StatusConflict:
		return errors.KindConflict
	case http.StatusPreconditionFailed:
		return errors.KindFailedPrecondition
	case http.StatusTooManyRequests:
		return errors.KindResourceExhausted
	case http.StatusNotImplemented:
		return errors.KindNotImplemented
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return errors.KindUnavailable
	default:
		if status >= 400 && status < 500 {
			return errors.KindInvalidArgument
		}
		return errors.KindInternal
	}
}
