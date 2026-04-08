package migration_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/storage/migration"
	"gorm.io/gorm"
)

func ExampleNewRegistry() {
	reg := migration.NewRegistry()

	reg.Add(migration.Migration{
		Version:     20240101000000,
		Description: "create users table",
		Up:          func(tx *gorm.DB) error { return nil },
		Down:        func(tx *gorm.DB) error { return nil },
	})

	reg.Add(migration.Migration{
		Version:     20240102000000,
		Description: "add email column",
		Up:          func(tx *gorm.DB) error { return nil },
		Down:        func(tx *gorm.DB) error { return nil },
	})

	migrations := reg.Migrations()
	fmt.Println("count:", len(migrations))
	fmt.Println("first:", migrations[0].Description)
	fmt.Println("sorted:", migrations[0].Version < migrations[1].Version)
	// Output:
	// count: 2
	// first: create users table
	// sorted: true
}
