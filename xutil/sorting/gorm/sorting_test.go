package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	gormdb "gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/xutil/sorting"
)

func newTestDB(t *testing.T) *gormdb.DB {
	t.Helper()

	db, err := gormdb.Open(sqlite.Open(":memory:"), &gormdb.Config{})
	require.NoError(t, err)

	return db
}

func TestScope(t *testing.T) {
	scope := Scope(sorting.New("created_at:desc"))
	db := scope(newTestDB(t))

	assert.NotNil(t, db)
}

func TestApply(t *testing.T) {
	db := Apply(newTestDB(t), sorting.New("created_at:desc"))

	assert.NotNil(t, db)
}

func TestApplyEmpty(t *testing.T) {
	base := newTestDB(t)
	db := Apply(base, sorting.Sorting{})

	assert.Same(t, base, db)
}
