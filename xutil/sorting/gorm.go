package sorting

import "gorm.io/gorm"

// GORMScope 返回 GORM scope 函数，用于链式调用.
// 使用示例:
//
//	db.Scopes(sorting.GORMScope()).Find(&users)
func (s Sorting) GORMScope() func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.IsEmpty() {
			return db
		}
		return db.Order(s.String())
	}
}

// Apply 应用排序到 GORM 查询.
// 使用示例:
//
//	sorting.New("created_time:desc").Apply(db).Find(&users)
func (s Sorting) Apply(db *gorm.DB) *gorm.DB {
	if s.IsEmpty() {
		return db
	}
	return db.Order(s.String())
}
