package pagination

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecode(t *testing.T) {
	tests := []struct {
		name   string
		values []any
	}{
		{"单个字符串", []any{"abc123"}},
		{"单个数字", []any{float64(42)}},
		{"多个值", []any{"id_100", float64(1680000000)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cursor := EncodeCursor(tt.values...)
			assert.NotEmpty(t, cursor)

			decoded, err := DecodeCursor(cursor)
			require.NoError(t, err)
			assert.Equal(t, tt.values, decoded)
		})
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		cursor string
	}{
		{"空字符串", ""},
		{"非 base64", "!!!invalid!!!"},
		{"非 JSON", "bm90IGpzb24="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeCursor(tt.cursor)
			assert.ErrorIs(t, err, ErrInvalidCursor)
		})
	}
}

func TestCursorResponse(t *testing.T) {
	resp := CursorResponse[string]{
		Items:      []string{"a", "b", "c"},
		NextCursor: EncodeCursor("c"),
		HasMore:    true,
	}

	assert.Len(t, resp.Items, 3)
	assert.True(t, resp.HasMore)
	assert.NotEmpty(t, resp.NextCursor)
	assert.Empty(t, resp.PrevCursor)
}

func TestCursorRequestApply(t *testing.T) {
	t.Run("默认值", func(t *testing.T) {
		req := &CursorRequest{}
		req.Apply()
		assert.Equal(t, DefaultCursorLimit, req.Limit)
		assert.Equal(t, Forward, req.Direction)
	})

	t.Run("超过最大值", func(t *testing.T) {
		req := &CursorRequest{Limit: 999}
		req.Apply()
		assert.Equal(t, MaxCursorLimit, req.Limit)
	})

	t.Run("保留有效值", func(t *testing.T) {
		req := &CursorRequest{Limit: 50, Direction: Backward}
		req.Apply()
		assert.Equal(t, 50, req.Limit)
		assert.Equal(t, Backward, req.Direction)
	})
}
