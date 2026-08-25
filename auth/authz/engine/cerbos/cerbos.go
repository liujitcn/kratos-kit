package cerbos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/liujitcn/kratos-kit/auth/authz/engine"
)

var (
	// ErrMissingEndpoint 表示没有配置 Cerbos PDP 地址。
	ErrMissingEndpoint = errors.New("cerbos: endpoint is required")
)

// AuthorizerFunc 执行一次 Cerbos 语义的鉴权。
type AuthorizerFunc func(context.Context, engine.Subject, engine.Action, engine.Resource, engine.Project) (bool, error)

// Option 配置 Cerbos 鉴权引擎。
type Option func(*options)

type options struct {
	endpoint       string
	httpClient     *http.Client
	principalRoles map[engine.Subject][]string
	projects       engine.Projects
	authorizer     AuthorizerFunc
}

// WithEndpoint 配置 Cerbos PDP HTTP 地址。
func WithEndpoint(endpoint string) Option {
	return func(options *options) {
		options.endpoint = strings.TrimSuffix(endpoint, "/")
	}
}

// WithHTTPClient 配置 Cerbos 请求使用的 HTTP 客户端。
func WithHTTPClient(httpClient *http.Client) Option {
	return func(options *options) {
		options.httpClient = httpClient
	}
}

// WithPrincipalRoles 配置主体对应的 Cerbos 角色。
func WithPrincipalRoles(roles map[engine.Subject][]string) Option {
	return func(options *options) {
		options.principalRoles = make(map[engine.Subject][]string, len(roles))
		for subject, values := range roles {
			options.principalRoles[subject] = slices.Clone(values)
		}
	}
}

// WithProjects 配置 FilterAuthorizedProjects 使用的候选项目。
func WithProjects(projects ...engine.Project) Option {
	return func(options *options) {
		options.projects = slices.Clone(projects)
	}
}

// WithAuthorizer 配置自定义 Cerbos 决策函数。
func WithAuthorizer(authorizer AuthorizerFunc) Option {
	return func(options *options) {
		options.authorizer = authorizer
	}
}

// State 是 Cerbos 鉴权引擎。
type State struct {
	options    *options
	httpClient *http.Client
}

var _ engine.Engine = (*State)(nil)

// NewEngine 创建 Cerbos 鉴权引擎。
func NewEngine(_ context.Context, opts ...Option) (*State, error) {
	options := &options{}
	for _, option := range opts {
		option(options)
	}
	if options.endpoint == "" && options.authorizer == nil {
		return nil, ErrMissingEndpoint
	}
	httpClient := options.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &State{options: options, httpClient: httpClient}, nil
}

// Name 返回鉴权引擎名称。
func (s *State) Name() string {
	return string(engine.Cerbos)
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

// FilterAuthorizedProjects 过滤配置中的候选项目。
func (s *State) FilterAuthorizedProjects(ctx context.Context, subjects engine.Subjects) (engine.Projects, error) {
	return s.ProjectsAuthorized(ctx, subjects, "*", "*", s.options.projects)
}

// IsAuthorized 判断主体是否可执行指定操作。
func (s *State) IsAuthorized(ctx context.Context, subject engine.Subject, action engine.Action, resource engine.Resource, project engine.Project) (bool, error) {
	if s.options.authorizer != nil {
		return s.options.authorizer(ctx, subject, action, resource, project)
	}
	return s.check(ctx, subject, action, resource, project)
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

// check 调用 Cerbos CheckResources API。
func (s *State) check(ctx context.Context, subject engine.Subject, action engine.Action, resource engine.Resource, project engine.Project) (bool, error) {
	kind, resourceID := splitResource(string(resource))
	attributes := make(map[string]any)
	if project != "" {
		attributes["project"] = project
	}
	body := map[string]any{
		"principal": map[string]any{
			"id":    subject,
			"roles": s.options.principalRoles[subject],
		},
		"resources": []map[string]any{{
			"resource": map[string]any{
				"kind": kind,
				"id":   resourceID,
				"attr": attributes,
			},
			"actions": []engine.Action{action},
		}},
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return false, fmt.Errorf("cerbos: encode request: %w", err)
	}
	var request *http.Request
	request, err = http.NewRequestWithContext(ctx, http.MethodPost, s.options.endpoint+"/api/check/resources", bytes.NewReader(encoded))
	if err != nil {
		return false, fmt.Errorf("cerbos: create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	var response *http.Response
	response, err = s.httpClient.Do(request)
	if err != nil {
		return false, fmt.Errorf("cerbos: check resources: %w", err)
	}
	var responseBody []byte
	responseBody, err = io.ReadAll(io.LimitReader(response.Body, 1<<20))
	closeErr := response.Body.Close()
	if err != nil {
		return false, fmt.Errorf("cerbos: read response: %w", err)
	}
	if closeErr != nil {
		return false, fmt.Errorf("cerbos: close response: %w", closeErr)
	}
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("cerbos: HTTP %d: %s", response.StatusCode, string(responseBody))
	}
	var result struct {
		Results []struct {
			Actions map[string]json.RawMessage `json:"actions"`
		} `json:"results"`
	}
	if err = json.Unmarshal(responseBody, &result); err != nil {
		return false, fmt.Errorf("cerbos: decode response: %w", err)
	}
	if len(result.Results) == 0 {
		return false, nil
	}
	return isAllowed(result.Results[0].Actions[string(action)])
}

// splitResource 把 kind:id 格式拆为 Cerbos 资源类型和 ID。
func splitResource(resource string) (string, string) {
	kind, resourceID, found := strings.Cut(resource, ":")
	if !found {
		return resource, resource
	}
	return kind, resourceID
}

// isAllowed 兼容 Cerbos 当前 effect 字符串和旧布尔响应。
func isAllowed(raw json.RawMessage) (bool, error) {
	var effect string
	err := json.Unmarshal(raw, &effect)
	if err == nil {
		return effect == "EFFECT_ALLOW", nil
	}
	var allowed bool
	if err = json.Unmarshal(raw, &allowed); err != nil {
		return false, fmt.Errorf("cerbos: decode action decision: %w", err)
	}
	return allowed, nil
}

// init 注册 Cerbos 鉴权引擎工厂。
func init() {
	err := engine.Register(engine.Cerbos, func(ctx context.Context, rawOptions ...any) (engine.Engine, error) {
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
