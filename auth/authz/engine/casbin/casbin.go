package casbin

import (
	"context"
	"sync"

	"github.com/go-kratos/kratos/v3/log"

	stdCasbin "github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"

	"github.com/liujitcn/kratos-kit/auth/authz/engine"
	"github.com/liujitcn/kratos-kit/auth/authz/engine/casbin/assets"
)

func init() {
	_ = engine.Register(engine.Casbin, func(ctx context.Context, options ...any) (engine.Engine, error) {
		var opts []OptFunc
		if len(options) > 0 {
			for _, o := range options {
				if opt, ok := o.(OptFunc); ok {
					opts = append(opts, opt)
				}
			}
		}

		return NewEngine(ctx, opts...)
	})
}

var _ engine.Engine = (*State)(nil)
var _ engine.PolicyWriter = (*State)(nil)

// State 保存 Casbin 模型、策略执行器和项目策略快照。
type State struct {
	policyMu sync.RWMutex

	model    model.Model
	policy   *Adapter
	enforcer *stdCasbin.SyncedEnforcer

	projects                  engine.Projects
	wildcardItem              string
	authorizedProjectsMatcher string
}

// NewEngine 创建 Casbin 鉴权引擎实例。
func NewEngine(_ context.Context, opts ...OptFunc) (*State, error) {
	s := State{
		policy:                    newAdapter(),
		projects:                  engine.Projects{},
		wildcardItem:              DefaultWildcardItem,
		authorizedProjectsMatcher: DefaultAuthorizedProjectsMatcher,
	}

	if err := s.init(opts...); err != nil {
		return nil, err
	}

	return &s, nil
}

// init 初始化 Casbin 模型与执行器。
func (s *State) init(opts ...OptFunc) error {
	for _, opt := range opts {
		opt(s)
	}

	var err error

	if s.model == nil {
		s.model, err = model.NewModelFromString(assets.DefaultRestfullWithRoleModel)
		if err != nil {
			log.Error("casbin.authz.engine: failed to create casbin model", "error", err)
			return err
		}
	}

	s.enforcer, err = stdCasbin.NewSyncedEnforcer(s.model, s.policy)
	if err != nil {
		log.Error("casbin.authz.engine: failed to create casbin enforcer", "error", err)
		return err
	}

	return nil
}

// Name 返回引擎名称。
func (s *State) Name() string {
	return string(engine.Casbin)
}

// ProjectsAuthorized 返回在指定项目集合中具备权限的项目列表。
func (s *State) ProjectsAuthorized(ctx context.Context, subjects engine.Subjects, action engine.Action, resource engine.Resource, projects engine.Projects) (engine.Projects, error) {
	result := make(engine.Projects, 0, len(projects))
	tenant := tenantFromContext(ctx)

	var err error
	var allowed bool
	for _, project := range projects {
		for _, subject := range subjects {
			if allowed, err = s.enforcer.Enforce(string(tenant), string(subject), string(resource), string(action), string(project)); err != nil {
				log.Error("casbin.authz.engine: failed to enforce policy for projects", "error", err)
				return nil, err
			} else if allowed {
				result = append(result, project)
				break
			}
		}
	}

	return result, nil
}

// FilterAuthorizedPairs 过滤出主体具备权限的资源动作对。
func (s *State) FilterAuthorizedPairs(ctx context.Context, subjects engine.Subjects, pairs engine.Pairs) (engine.Pairs, error) {
	result := make(engine.Pairs, 0, len(pairs))

	tenant := tenantFromContext(ctx)
	project := engine.Project(s.wildcardItem)

	var err error
	var allowed bool
	for _, p := range pairs {
		for _, subject := range subjects {
			if allowed, err = s.enforcer.Enforce(string(tenant), string(subject), string(p.Resource), string(p.Action), string(project)); err != nil {
				log.Error("casbin.authz.engine: failed to enforce policy for pair", "error", err)
				return nil, err
			} else if allowed {
				result = append(result, p)
				break
			}
		}
	}
	return result, nil
}

// FilterAuthorizedProjects 过滤出主体具备访问权限的项目列表。
func (s *State) FilterAuthorizedProjects(ctx context.Context, subjects engine.Subjects) (engine.Projects, error) {
	s.policyMu.RLock()
	projects := append(engine.Projects(nil), s.projects...)
	s.policyMu.RUnlock()
	result := make(engine.Projects, 0, len(projects))

	tenant := tenantFromContext(ctx)
	resource := engine.Resource(s.wildcardItem)
	action := engine.Action(s.wildcardItem)

	var err error
	var allowed bool
	for _, project := range projects {
		for _, subject := range subjects {
			if allowed, err = s.enforcer.EnforceWithMatcher(s.authorizedProjectsMatcher, string(tenant), string(subject), string(resource), string(action), string(project)); err != nil {
				log.Error("casbin.authz.engine: failed to enforce policy with matcher", "error", err)
				return nil, err
			} else if allowed {
				result = append(result, project)
				break
			}
		}
	}

	return result, nil
}

// IsAuthorized 判断主体是否具备指定资源动作权限。
func (s *State) IsAuthorized(ctx context.Context, subject engine.Subject, action engine.Action, resource engine.Resource, project engine.Project) (bool, error) {
	if len(project) == 0 {
		// 未显式指定项目时，回退到通配项目规则进行匹配。
		project = engine.Project(s.wildcardItem)
	}

	tenant := tenantFromContext(ctx)

	var err error
	var allowed bool
	if allowed, err = s.enforcer.Enforce(string(tenant), string(subject), string(resource), string(action), string(project)); err != nil {
		log.Error("casbin.authz.engine: failed to enforce policy", "error", err)
		return false, err
	} else if allowed {
		return true, nil
	}
	return false, nil
}

// SetPolicies 更新策略并重新加载到 Casbin 执行器中。
func (s *State) SetPolicies(_ context.Context, policyMap engine.PolicyMap, _ engine.RoleMap) error {
	s.policyMu.Lock()
	defer s.policyMu.Unlock()

	s.policy.SetPolicies(policyMap)

	var err error
	err = s.enforcer.LoadPolicy()
	if err != nil {
		log.Error("casbin.authz.engine: failed to load policy", "error", err)
		return err
	}

	//fmt.Println(err, s.enforcer.GetAllSubjects(), s.enforcer.GetAllRoles())

	projects, ok := policyMap["projects"]
	if ok {
		switch t := projects.(type) {
		case engine.Projects:
			s.projects = append(engine.Projects(nil), t...)
		}
	}

	return nil
}

// tenantFromContext 从授权声明中读取租户编码，未设置时回退默认租户。
func tenantFromContext(ctx context.Context) string {
	claims, ok := engine.AuthClaimsFromContext(ctx)
	if !ok || claims.Tenant == nil || *claims.Tenant == "" {
		return DefaultTenant
	}
	return string(*claims.Tenant)
}
