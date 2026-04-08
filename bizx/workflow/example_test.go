package workflow_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/bizx/workflow"
)

func ExampleEngine_StartWorkflow() {
	store := workflow.NewMemoryStore()
	engine := workflow.New(store)

	// 定义工作流: 开始 -> 验证 -> 结束.
	def := &workflow.Definition{
		ID:          "approval-flow",
		Name:        "Approval Workflow",
		Version:     "1.0",
		StartNodeID: "validate",
		Nodes: map[string]*workflow.Node{
			"validate": {
				ID:   "validate",
				Name: "Validate Request",
				Type: workflow.NodeTypeTask,
				Handler: func(ctx context.Context, inst *workflow.Instance) error {
					inst.Data["validated"] = true
					return nil
				},
				NextNodes: []string{"end"},
			},
			"end": {
				ID:   "end",
				Name: "End",
				Type: workflow.NodeTypeEnd,
			},
		},
	}

	_ = engine.RegisterDefinition(def)

	ctx := context.Background()
	inst, _ := engine.StartWorkflow(ctx, "approval-flow", map[string]any{
		"amount": 1000,
	})
	fmt.Println("status:", inst.Status)

	// 执行工作流.
	_ = engine.Execute(ctx, inst.ID)

	got, _ := engine.GetInstance(ctx, inst.ID)
	fmt.Println("final status:", got.Status)
	fmt.Println("validated:", got.Data["validated"])
	// Output:
	// status: pending
	// final status: completed
	// validated: true
}
