package engine

// Type 表示鉴权引擎类型。
type Type string

const (
	// Noop 表示不执行鉴权的空实现。
	Noop Type = "noop"
	// Casbin 表示 Casbin 鉴权引擎。
	Casbin Type = "casbin"
	// Cerbos 表示 Cerbos PDP 鉴权引擎。
	Cerbos Type = "cerbos"
	// Opa 表示 Open Policy Agent 鉴权引擎。
	Opa Type = "opa"
	// Zanzibar 表示 Zanzibar 关系型鉴权引擎。
	Zanzibar Type = "zanzibar"
)
