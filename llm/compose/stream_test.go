package compose_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/Tsukikage7/servex/v2/llm/compose"
)

func TestNewStreamReader_RecvAll(t *testing.T) {
	items := []string{"a", "b", "c"}
	r := compose.NewSliceStreamReader(items)

	var got []string
	for {
		v, err := r.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		got = append(got, v)
	}
	assert.Equal(t, items, got)
}

func TestBoxStream_SingleFrame(t *testing.T) {
	r := compose.BoxStream("hello")
	v, err := r.Recv()
	require.NoError(t, err)
	assert.Equal(t, "hello", v)

	_, err = r.Recv()
	assert.Equal(t, io.EOF, err)
}

func TestConcatStream_Strings(t *testing.T) {
	items := []string{"foo", "bar", "baz"}
	r := compose.NewSliceStreamReader(items)
	result, err := compose.ConcatStream(r, func(a, b string) string { return a + b })
	require.NoError(t, err)
	assert.Equal(t, "foobarbaz", result)
}

func TestConcatStream_Empty(t *testing.T) {
	r := compose.NewSliceStreamReader([]string{})
	result, err := compose.ConcatStream(r, func(a, b string) string { return a + b })
	require.NoError(t, err)
	assert.Equal(t, "", result)
}
