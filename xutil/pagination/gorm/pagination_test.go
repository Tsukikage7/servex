package gorm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	gormdb "gorm.io/gorm"

	"github.com/Tsukikage7/servex/v2/xutil/pagination"
)

type testItem struct {
	ID   int    `gorm:"primaryKey"`
	Name string `gorm:"size:100"`
}

func setupTestDB(t *testing.T) *gormdb.DB {
	t.Helper()

	db, err := gormdb.Open(sqlite.Open(":memory:"), &gormdb.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(&testItem{}))

	items := []testItem{
		{ID: 1, Name: "item1"},
		{ID: 2, Name: "item2"},
		{ID: 3, Name: "item3"},
		{ID: 4, Name: "item4"},
		{ID: 5, Name: "item5"},
	}
	require.NoError(t, db.Create(&items).Error)

	return db
}

func TestPaginate(t *testing.T) {
	t.Run("第一页", func(t *testing.T) {
		db := setupTestDB(t)
		req := &pagination.CursorRequest{Limit: 2}

		var items []testItem
		err := Paginate(db.Model(&testItem{}), req, "id").Find(&items).Error

		require.NoError(t, err)
		assert.Len(t, items, 3) // Limit + 1
		assert.Equal(t, 1, items[0].ID)
		assert.Equal(t, 2, items[1].ID)
	})

	t.Run("使用游标向前", func(t *testing.T) {
		db := setupTestDB(t)
		req := &pagination.CursorRequest{
			Limit:     2,
			Cursor:    pagination.EncodeCursor(float64(2)),
			Direction: pagination.Forward,
		}

		var items []testItem
		err := Paginate(db.Model(&testItem{}), req, "id").Find(&items).Error

		require.NoError(t, err)
		assert.Len(t, items, 3)
		assert.Equal(t, 3, items[0].ID)
	})

	t.Run("使用游标向后", func(t *testing.T) {
		db := setupTestDB(t)
		req := &pagination.CursorRequest{
			Limit:     2,
			Cursor:    pagination.EncodeCursor(float64(4)),
			Direction: pagination.Backward,
		}

		var items []testItem
		err := Paginate(db.Model(&testItem{}), req, "id").Find(&items).Error

		require.NoError(t, err)
		assert.Len(t, items, 3)
		assert.Equal(t, 3, items[0].ID) // 降序
	})

	t.Run("无效游标", func(t *testing.T) {
		db := setupTestDB(t)
		req := &pagination.CursorRequest{
			Limit:  2,
			Cursor: "invalid",
		}

		var items []testItem
		err := Paginate(db.Model(&testItem{}), req, "id").Find(&items).Error

		require.NoError(t, err)
		assert.Empty(t, items)
	})
}
