// Package zanzibar 提供适配 OpenFGA、Keto 等关系型鉴权服务的引擎。
package zanzibar

import (
	"context"
	"errors"

	"github.com/liujitcn/kratos-kit/auth/authz/engine"
)

var (
	// ErrMissingChecker 表示没有配置关系元组检查器。
	ErrMissingChecker = errors.New("zanzibar: checker is required")
	// ErrMissingProjectLister 表示没有配置项目枚举能力。
	ErrMissingProjectLister = errors.New("zanzibar: project lister is required")
)

// Tuple 表示一次 Zanzibar 关系检查。
type Tuple struct {
	// Subject 是待检查主体。
	Subject engine.Subject
	// Relation 是主体与对象之间的关系。
	Relation engine.Action
	// Object 是待检查对象。
	Object engine.Resource
	// Project 是可选的项目或命名空间。
	Project engine.Project
}

// Checker 检查关系元组是否成立。
type Checker interface {
	// Check 执行一次关系检查。
	Check(context.Context, Tuple) (bool, error)
}

// CheckerFunc 把函数适配为 Checker。
type CheckerFunc func(context.Context, Tuple) (bool, error)

// Check 执行一次关系检查。
func (f CheckerFunc) Check(ctx context.Context, tuple Tuple) (bool, error) {
	return f(ctx, tuple)
}

// ProjectLister 查询主体可访问的全部项目。
type ProjectLister interface {
	// ListProjects 返回主体可访问的项目。
	ListProjects(context.Context, engine.Subjects) (engine.Projects, error)
}

// ProjectListerFunc 把函数适配为 ProjectLister。
type ProjectListerFunc func(context.Context, engine.Subjects) (engine.Projects, error)

// ListProjects 返回主体可访问的项目。
func (f ProjectListerFunc) ListProjects(ctx context.Context, subjects engine.Subjects) (engine.Projects, error) {
	return f(ctx, subjects)
}

// PolicyWriter 是通用策略写入能力的兼容别名。
type PolicyWriter = engine.PolicyWriter

// PolicyWriterFunc 把函数适配为 PolicyWriter。
type PolicyWriterFunc func(context.Context, engine.PolicyMap, engine.RoleMap) error

// SetPolicies 更新关系策略。
func (f PolicyWriterFunc) SetPolicies(ctx context.Context, policies engine.PolicyMap, roles engine.RoleMap) error {
	return f(ctx, policies, roles)
}

// Option 配置 Zanzibar 鉴权引擎。
type Option func(*options)

type options struct {
	checker       Checker
	projectLister ProjectLister
	policyWriter  PolicyWriter
}

// WithChecker 配置关系元组检查器。
func WithChecker(checker Checker) Option {
	return func(options *options) {
		options.checker = checker
	}
}

// WithProjectLister 配置项目枚举器。
func WithProjectLister(projectLister ProjectLister) Option {
	return func(options *options) {
		options.projectLister = projectLister
	}
}

// WithPolicyWriter 配置关系策略写入器。
func WithPolicyWriter(policyWriter PolicyWriter) Option {
	return func(options *options) {
		options.policyWriter = policyWriter
	}
}

// State 是 Zanzibar 鉴权引擎。
type State struct {
	options *options
}

var _ engine.Engine = (*State)(nil)

// NewEngine 创建 Zanzibar 鉴权引擎。
func NewEngine(_ context.Context, opts ...Option) (*State, error) {
	options := &options{}
	for _, option := range opts {
		option(options)
	}
	if options.checker == nil {
		return nil, ErrMissingChecker
	}
	return &State{options: options}, nil
}

// Name 返回鉴权引擎名称。
func (s *State) Name() string {
	return string(engine.Zanzibar)
}

// ProjectsAuthorized 过滤指定候选项目。
func (s *State) ProjectsAuthorized(ctx context.Context, subjects engine.Subjects, action engine.Action, resource engine.Resource, projects engine.Projects) (engine.Projects, error) {
	authorized := make(engine.Projects, 0, len(projects))
	for _, project := range projects {
		allowed, err := s.authorizeAny(ctx, subjects, action, resource, project)
		if err != nil {
			return nil, err
		}
		if allowed {
			authorized = append(authorized, project)
		}
	}
	return authorized, nil
}

// FilterAuthorizedPairs 过滤指定资源操作对。
func (s *State) FilterAuthorizedPairs(ctx context.Context, subjects engine.Subjects, pairs engine.Pairs) (engine.Pairs, error) {
	authorized := make(engine.Pairs, 0, len(pairs))
	for _, pair := range pairs {
		allowed, err := s.authorizeAny(ctx, subjects, pair.Action, pair.Resource, "")
		if err != nil {
			return nil, err
		}
		if allowed {
			authorized = append(authorized, pair)
		}
	}
	return authorized, nil
}

// FilterAuthorizedProjects 查询主体可访问的全部项目。
func (s *State) FilterAuthorizedProjects(ctx context.Context, subjects engine.Subjects) (engine.Projects, error) {
	if s.options.projectLister == nil {
		return nil, ErrMissingProjectLister
	}
	return s.options.projectLister.ListProjects(ctx, subjects)
}

// IsAuthorized 检查关系元组是否成立。
func (s *State) IsAuthorized(ctx context.Context, subject engine.Subject, action engine.Action, resource engine.Resource, project engine.Project) (bool, error) {
	return s.options.checker.Check(ctx, Tuple{
		Subject:  subject,
		Relation: action,
		Object:   resource,
		Project:  project,
	})
}

// PolicyWriter 返回显式配置的可选关系策略写入器。
func (s *State) PolicyWriter() (engine.PolicyWriter, bool) {
	return s.options.policyWriter, s.options.policyWriter != nil
}

// authorizeAny 判断任一主体是否获得授权。
func (s *State) authorizeAny(ctx context.Context, subjects engine.Subjects, action engine.Action, resource engine.Resource, project engine.Project) (bool, error) {
	for _, subject := range subjects {
		allowed, err := s.IsAuthorized(ctx, subject, action, resource, project)
		if err != nil {
			return false, err
		}
		if allowed {
			return true, nil
		}
	}
	return false, nil
}

// init 注册 Zanzibar 鉴权引擎工厂。
func init() {
	err := engine.Register(engine.Zanzibar, func(ctx context.Context, rawOptions ...any) (engine.Engine, error) {
		options := make([]Option, 0, len(rawOptions))
		for _, rawOption := range rawOptions {
			if option, ok := rawOption.(Option); ok {
				options = append(options, option)
			}
		}
		return NewEngine(ctx, options...)
	})
	if err != nil {
		panic(err)
	}
}
