package env

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Tsukikage7/servex/v2/config"
)

func TestSource_Load_NoPrefix(t *testing.T) {
	os.Setenv("TEST_ENV_VAR_A", "value_a")
	os.Setenv("TEST_ENV_VAR_B", "value_b")
	defer os.Unsetenv("TEST_ENV_VAR_A")
	defer os.Unsetenv("TEST_ENV_VAR_B")

	src := New()
	kvs, err := src.Load()
	require.NoError(t, err)
	require.Len(t, kvs, 1)

	var data map[string]string
	require.NoError(t, json.Unmarshal(kvs[0].Value, &data))

	assert.Equal(t, "value_a", data["TEST_ENV_VAR_A"])
	assert.Equal(t, "value_b", data["TEST_ENV_VAR_B"])
}

func TestSource_Load_WithPrefix(t *testing.T) {
	os.Setenv("APP_HOST", "localhost")
	os.Setenv("APP_PORT", "8080")
	os.Setenv("OTHER_VAR", "ignored")
	defer os.Unsetenv("APP_HOST")
	defer os.Unsetenv("APP_PORT")
	defer os.Unsetenv("OTHER_VAR")

	src := New(WithPrefix("APP_"))
	kvs, err := src.Load()
	require.NoError(t, err)
	require.Len(t, kvs, 1)

	var data map[string]string
	require.NoError(t, json.Unmarshal(kvs[0].Value, &data))

	assert.Equal(t, "localhost", data["HOST"])
	assert.Equal(t, "8080", data["PORT"])
	assert.NotContains(t, data, "OTHER_VAR")
	assert.NotContains(t, data, "APP_HOST")
}

func TestSource_Load_Format(t *testing.T) {
	src := New()
	kvs, err := src.Load()
	require.NoError(t, err)
	require.Len(t, kvs, 1)
	assert.Equal(t, "json", kvs[0].Format)
	assert.Equal(t, "env", kvs[0].Key)
}

func TestSource_Watch_NoEnvFile(t *testing.T) {
	src := New()
	watcher, err := src.Watch()
	require.NoError(t, err)
	defer watcher.Stop()

	// 无 envFile 的 watcher 直接返回 ErrSourceClosed
	_, err = watcher.Next()
	assert.True(t, errors.Is(err, config.ErrSourceClosed))
}

func TestSource_Watch_WithEnvFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.env")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("WATCH_HOST=localhost\n")
	tmpFile.Close()

	src := New(WithEnvFile(tmpFile.Name()))
	watcher, err := src.Watch()
	require.NoError(t, err)

	// 停止后 Next 应返回 ErrSourceClosed
	require.NoError(t, watcher.Stop())
	_, err = watcher.Next()
	assert.True(t, errors.Is(err, config.ErrSourceClosed))
}
