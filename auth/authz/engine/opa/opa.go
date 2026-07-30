// Package opa 提供基于 Open Policy Agent Rego 的本地鉴权引擎。
package opa

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/liujitcn/kratos-kit/auth/authz/engine"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage/inmem"
)

const (
	defaultDecisionQuery = "data.authz.allow"
	defaultProjectsQuery = "data.authz.authorized_projects"
)

var (
	// ErrMissingModules 表示没有配置任何 Rego 模块。
	ErrMissingModules = errors.New("opa: at least one Rego module is required")
	// ErrUnexpectedDecision 表示 Rego 决策结果不是单个布尔值。
	ErrUnexpectedDecision = errors.New("opa: decision query must return one boolean value")
	// ErrUnexpectedProjects 表示项目查询结果不是字符串集合。
	ErrUnexpectedProjects = errors.New("opa: projects query must return a string collection")
)

// Option 配置 OPA 鉴权引擎。
type Option func(*options)

type options struct {
	modules       map[string]string
	decisionQuery string
	projectsQuery string
}

// WithModule 添加一个 Rego 模块。
func WithModule(name string, module string) Option {
	return func(options *options) {
		if options.modules == nil {
			options.modules = make(map[string]string)
		}
		options.modules[name] = module
	}
}

// WithModules 配置 Rego 模块集合。
func WithModules(modules map[string]string) Option {
	return func(options *options) {
		options.modules = make(map[string]string, len(modules))
		for name, module := range modules {
			options.modules[name] = module
		}
	}
}

// WithDecisionQuery 配置单次鉴权决策查询。
func WithDecisionQuery(query string) Option {
	return func(options *options) {
		options.decisionQuery = query
	}
}

// WithProjectsQuery 配置可访问项目集合查询。
func WithProjectsQuery(query string) Option {
	return func(options *options) {
		options.projectsQuery = query
	}
}

// State 是 OPA 鉴权引擎。
type State struct {
	mu            sync.RWMutex
	options       *options
	policies      engine.PolicyMap
	roles         engine.RoleMap
	decisionQuery rego.PreparedEvalQuery
	projectsQuery rego.PreparedEvalQuery
}

var _ engine.Engine = (*State)(nil)
var _ engine.PolicyWriter = (*State)(nil)

// NewEngine 创建 OPA 鉴权引擎。
func NewEngine(ctx context.Context, opts ...Option) (*State, error) {
	options := &options{
		decisionQuery: defaultDecisionQuery,
		projectsQuery: defaultProjectsQuery,
	}
	for _, option := range opts {
		option(options)
	}
	if len(options.modules) == 0 {
		return nil, ErrMissingModules
	}
	state := &State{
		options:  options,
		policies: make(engine.PolicyMap),
		roles:    make(engine.RoleMap),
	}
	if err := state.prepare(ctx); err != nil {
		return nil, err
	}
	return state, nil
}

// Name 返回鉴权引擎名称。
func (s *State) Name() string {
	return string(engine.Opa)
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
	s.mu.RLock()
	prepared := s.projectsQuery
	s.mu.RUnlock()
	resultSet, err := prepared.Eval(ctx, rego.EvalInput(map[string]any{
		"subjects": subjects,
	}))
	if err != nil {
		return nil, fmt.Errorf("opa: evaluate projects: %w", err)
	}
	return projectsFromResult(resultSet)
}

// IsAuthorized 判断主体是否可执行指定操作。
func (s *State) IsAuthorized(ctx context.Context, subject engine.Subject, action engine.Action, resource engine.Resource, project engine.Project) (bool, error) {
	s.mu.RLock()
	prepared := s.decisionQuery
	s.mu.RUnlock()
	resultSet, err := prepared.Eval(ctx, rego.EvalInput(map[string]any{
		"subject":  subject,
		"subjects": engine.MakeSubjects(subject),
		"action":   action,
		"resource": resource,
		"project":  project,
	}))
	if err != nil {
		return false, fmt.Errorf("opa: evaluate decision: %w", err)
	}
	return decisionFromResult(resultSet)
}

// SetPolicies 原子替换策略和角色数据并重新准备查询。
func (s *State) SetPolicies(ctx context.Context, policies engine.PolicyMap, roles engine.RoleMap) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies = cloneMap(policies)
	s.roles = cloneMap(roles)
	return s.prepareLocked(ctx)
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

// prepare 准备决策查询和项目查询。
func (s *State) prepare(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prepareLocked(ctx)
}

// prepareLocked 在持有写锁时准备查询。
func (s *State) prepareLocked(ctx context.Context) error {
	data := map[string]any{
		"policies": s.policies,
		"roles":    s.roles,
	}
	queryOptions := make([]func(*rego.Rego), 0, len(s.options.modules)+2)
	queryOptions = append(queryOptions, rego.Store(inmem.NewFromObject(data)))
	for name, module := range s.options.modules {
		queryOptions = append(queryOptions, rego.Module(name, module))
	}
	decisionOptions := append([]func(*rego.Rego){rego.Query(s.options.decisionQuery)}, queryOptions...)
	decision, err := rego.New(decisionOptions...).PrepareForEval(ctx)
	if err != nil {
		return fmt.Errorf("opa: prepare decision query: %w", err)
	}
	projectOptions := append([]func(*rego.Rego){rego.Query(s.options.projectsQuery)}, queryOptions...)
	var projects rego.PreparedEvalQuery
	projects, err = rego.New(projectOptions...).PrepareForEval(ctx)
	if err != nil {
		return fmt.Errorf("opa: prepare projects query: %w", err)
	}
	s.decisionQuery = decision
	s.projectsQuery = projects
	return nil
}

// decisionFromResult 解析单个布尔决策。
func decisionFromResult(resultSet rego.ResultSet) (bool, error) {
	if len(resultSet) != 1 || len(resultSet[0].Expressions) != 1 {
		return false, ErrUnexpectedDecision
	}
	allowed, ok := resultSet[0].Expressions[0].Value.(bool)
	if !ok {
		return false, ErrUnexpectedDecision
	}
	return allowed, nil
}

// projectsFromResult 解析项目集合。
func projectsFromResult(resultSet rego.ResultSet) (engine.Projects, error) {
	if len(resultSet) != 1 || len(resultSet[0].Expressions) != 1 {
		return nil, ErrUnexpectedProjects
	}
	values, ok := resultSet[0].Expressions[0].Value.([]any)
	if !ok {
		return nil, ErrUnexpectedProjects
	}
	projects := make(engine.Projects, 0, len(values))
	for _, value := range values {
		project, matched := value.(string)
		if !matched {
			return nil, ErrUnexpectedProjects
		}
		projects = append(projects, engine.Project(project))
	}
	return projects, nil
}

// cloneMap 浅复制策略或角色映射。
func cloneMap[T ~map[string]any](value T) T {
	cloned := make(T, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}

// init 注册 OPA 鉴权引擎工厂。
func init() {
	err := engine.Register(engine.Opa, func(ctx context.Context, rawOptions ...any) (engine.Engine, error) {
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
