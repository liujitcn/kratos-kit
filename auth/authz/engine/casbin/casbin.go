package casbin

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

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

type State struct {
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
			log.Errorf("casbin.authz.engine: failed to create casbin model: %v", err)
			return err
		}
	}

	s.enforcer, err = stdCasbin.NewSyncedEnforcer(s.model, s.policy)
	if err != nil {
		log.Errorf("casbin.authz.engine: failed to create casbin enforcer: %v", err)
		return err
	}

	return nil
}

// Name 返回引擎名称。
func (s *State) Name() string {
	return string(engine.Casbin)
}

// ProjectsAuthorized 返回在指定项目集合中具备权限的项目列表。
func (s *State) ProjectsAuthorized(_ context.Context, subjects engine.Subjects, action engine.Action, resource engine.Resource, projects engine.Projects) (engine.Projects, error) {
	result := make(engine.Projects, 0, len(projects))

	var err error
	var allowed bool
	for _, project := range projects {
		for _, subject := range subjects {
			if allowed, err = s.enforcer.Enforce(string(subject), string(resource), string(action), string(project)); err != nil {
				log.Errorf("casbin.authz.engine: failed to enforce policy for projects: %v", err)
				return nil, err
			} else if allowed {
				result = append(result, project)
			}
		}
	}

	return result, nil
}

// FilterAuthorizedPairs 过滤出主体具备权限的资源动作对。
func (s *State) FilterAuthorizedPairs(_ context.Context, subjects engine.Subjects, pairs engine.Pairs) (engine.Pairs, error) {
	result := make(engine.Pairs, 0, len(pairs))

	project := engine.Project(s.wildcardItem)

	var err error
	var allowed bool
	for _, p := range pairs {
		for _, subject := range subjects {
			if allowed, err = s.enforcer.Enforce(string(subject), string(p.Resource), string(p.Action), string(project)); err != nil {
				log.Errorf("casbin.authz.engine: failed to enforce policy for pair: %v", err)
				return nil, err
			} else if allowed {
				result = append(result, p)
			}
		}
	}
	return result, nil
}

// FilterAuthorizedProjects 过滤出主体具备访问权限的项目列表。
func (s *State) FilterAuthorizedProjects(_ context.Context, subjects engine.Subjects) (engine.Projects, error) {
	result := make(engine.Projects, 0, len(s.projects))

	resource := engine.Resource(s.wildcardItem)
	action := engine.Action(s.wildcardItem)

	var err error
	var allowed bool
	for _, project := range s.projects {
		for _, subject := range subjects {
			if allowed, err = s.enforcer.EnforceWithMatcher(s.authorizedProjectsMatcher, string(subject), string(resource), string(action), string(project)); err != nil {
				log.Errorf("casbin.authz.engine: failed to enforce policy with matcher: %v", err)
				return nil, err
			} else if allowed {
				result = append(result, project)
			}
		}
	}

	return result, nil
}

// IsAuthorized 判断主体是否具备指定资源动作权限。
func (s *State) IsAuthorized(_ context.Context, subject engine.Subject, action engine.Action, resource engine.Resource, project engine.Project) (bool, error) {
	if len(project) == 0 {
		// 未显式指定项目时，回退到通配项目规则进行匹配。
		project = engine.Project(s.wildcardItem)
	}

	var err error
	var allowed bool
	if allowed, err = s.enforcer.Enforce(string(subject), string(resource), string(action), string(project)); err != nil {
		log.Errorf("casbin.authz.engine: failed to enforce policy: %v", err)
		return false, err
	} else if allowed {
		return true, nil
	}
	return false, nil
}

// SetPolicies 更新策略并重新加载到 Casbin 执行器中。
func (s *State) SetPolicies(_ context.Context, policyMap engine.PolicyMap, _ engine.RoleMap) error {
	s.policy.SetPolicies(policyMap)

	if err := s.enforcer.LoadPolicy(); err != nil {
		log.Errorf("casbin.authz.engine: failed to load policy: %v", err)
		return err
	}

	//fmt.Println(err, s.enforcer.GetAllSubjects(), s.enforcer.GetAllRoles())

	projects, ok := policyMap["projects"]
	if ok {
		switch t := projects.(type) {
		case engine.Projects:
			s.projects = t
		}
	}

	return nil
}
