package engine

import (
	"context"
)

// Engine 定义所有鉴权引擎必须提供的只读决策能力。
type Engine interface {
	Authorizer
}

// Authorizer 定义鉴权决策与批量过滤能力。
type Authorizer interface {
	Name() string

	ProjectsAuthorized(ctx context.Context, subjects Subjects, action Action, resource Resource, projects Projects) (Projects, error)

	FilterAuthorizedPairs(ctx context.Context, subjects Subjects, pairs Pairs) (Pairs, error)

	FilterAuthorizedProjects(ctx context.Context, subjects Subjects) (Projects, error)

	IsAuthorized(ctx context.Context, subjects Subject, action Action, resource Resource, project Project) (bool, error)
}

// PolicyWriter 定义可选的策略更新能力。
type PolicyWriter interface {
	SetPolicies(ctx context.Context, policies PolicyMap, roles RoleMap) error
}

// Writer 是 PolicyWriter 的兼容别名。
type Writer = PolicyWriter
