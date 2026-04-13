package alerting_test

import (
	"fmt"
	"time"

	"github.com/Tsukikage7/servex/v2/observability/alerting"
)

func ExampleRule() {
	rule := &alerting.Rule{
		ID:   "high-cpu",
		Name: "High CPU Usage",
		Type: alerting.RuleThreshold,
		Condition: alerting.Condition{
			Metric:    "cpu_usage_percent",
			Operator:  alerting.OpGT,
			Threshold: 90.0,
			Duration:  5 * time.Minute,
		},
		Labels:       map[string]string{"severity": "critical"},
		EvalInterval: 30 * time.Second,
		For:          2 * time.Minute,
	}
	fmt.Println(rule.Name)
	fmt.Println(rule.Type)
	fmt.Println(rule.Condition.Operator)
	fmt.Println(rule.Condition.Threshold)
	// Output:
	// High CPU Usage
	// threshold
	// >
	// 90
}
