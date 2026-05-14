package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

const (
	// DefaultCursorLimit 默认游标分页每页数量.
	DefaultCursorLimit = 20
	// MaxCursorLimit 最大游标分页每页数量.
	MaxCursorLimit = 100
)

// Direction 分页方向.
type Direction string

const (
	// Forward 向前分页，获取更新的数据.
	Forward Direction = "forward"
	// Backward 向后分页，获取更旧的数据.
	Backward Direction = "backward"
)

// ErrInvalidCursor 表示游标格式无效.
var ErrInvalidCursor = errors.New("pagination: invalid cursor")

// CursorRequest 游标分页请求.
type CursorRequest struct {
	Cursor    string    `json:"cursor,omitempty"` // 上一页最后一条的游标，为空表示第一页
	Limit     int       `json:"limit"`            // 每页数量，默认 20
	Direction Direction `json:"direction"`        // 方向，默认 Forward
}

// Apply 应用默认值并校验参数.
func (r *CursorRequest) Apply() *CursorRequest {
	if r.Limit <= 0 {
		r.Limit = DefaultCursorLimit
	}
	if r.Limit > MaxCursorLimit {
		r.Limit = MaxCursorLimit
	}
	if r.Direction == "" {
		r.Direction = Forward
	}
	return r
}

// CursorResponse 游标分页响应.
type CursorResponse[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	PrevCursor string `json:"prev_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// EncodeCursor 将值编码为游标字符串（base64url + JSON）.
func EncodeCursor(values ...any) string {
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(data)
}

// DecodeCursor 解码游标字符串.
func DecodeCursor(cursor string) ([]any, error) {
	if cursor == "" {
		return nil, ErrInvalidCursor
	}
	data, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var values []any
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, ErrInvalidCursor
	}
	return values, nil
}
