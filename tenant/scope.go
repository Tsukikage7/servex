package tenant

import "context"

const defaultColumn = "tenant_id"

// IDFromContext 从 context 提取 tenant ID，无租户返回空字符串.
// 等同于 ID(ctx)，语义更明确.
func IDFromContext(ctx context.Context) string {
	return ID(ctx)
}

// WhereClause 返回 SQL WHERE 子句和参数.
// 无租户时返回空字符串（不过滤）.
// 示例:
//
//	clause, args := tenant.WhereClause(ctx)
//	// → ("tenant_id = ?", ["abc123"])
//	clause, args := tenant.WhereClause(ctx, "t.tenant_id")
//	// → ("t.tenant_id = ?", ["abc123"])
func WhereClause(ctx context.Context, column ...string) (clause string, args []any) {
	id := ID(ctx)
	if id == "" {
		return "", nil
	}

	col := defaultColumn
	if len(column) > 0 && column[0] != "" {
		col = column[0]
	}

	return col + " = ?", []any{id}
}
