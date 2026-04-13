package abtesting_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/bizx/abtesting"
)

func ExampleManager_CreateExperiment() {
	store := abtesting.NewMemoryStore()
	mgr := abtesting.New(store)
	ctx := context.Background()

	exp := &abtesting.Experiment{
		ID:      "exp-001",
		Name:    "button-color",
		Enabled: true,
		Salt:    "salt123",
		Variants: []abtesting.Variant{
			{ID: "control", Name: "Blue Button", Weight: 50},
			{ID: "variant-a", Name: "Red Button", Weight: 50},
		},
	}

	err := mgr.CreateExperiment(ctx, exp)
	fmt.Println("create:", err)

	got, _ := mgr.GetExperiment(ctx, "exp-001")
	fmt.Println("name:", got.Name)
	fmt.Println("variants:", len(got.Variants))
	// Output:
	// create: <nil>
	// name: button-color
	// variants: 2
}

func ExampleManager_Assign() {
	store := abtesting.NewMemoryStore()
	mgr := abtesting.New(store)
	ctx := context.Background()

	_ = mgr.CreateExperiment(ctx, &abtesting.Experiment{
		ID:      "exp-001",
		Name:    "button-color",
		Enabled: true,
		Salt:    "salt123",
		Variants: []abtesting.Variant{
			{ID: "control", Name: "Blue", Weight: 50},
			{ID: "variant-a", Name: "Red", Weight: 50},
		},
	})

	// 分配用户（确定性哈希，同一用户始终分到同一组）.
	a1, _ := mgr.Assign(ctx, "exp-001", "user-42")
	a2, _ := mgr.Assign(ctx, "exp-001", "user-42")
	fmt.Println("consistent:", a1.VariantID == a2.VariantID)
	fmt.Println("experiment:", a1.ExperimentID)
	// Output:
	// consistent: true
	// experiment: exp-001
}
