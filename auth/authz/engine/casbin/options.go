package casbin

import (
	"github.com/casbin/casbin/v2/model"
	"github.com/liujitcn/kratos-kit/auth/authz/engine"
	"github.com/liujitcn/kratos-kit/auth/authz/engine/casbin/assets"
)

type OptFunc func(*State)

// WithModel 指定自定义 Casbin 模型。
func WithModel(model model.Model) OptFunc {
	return func(s *State) {
		s.model = model
	}
}

// WithStringModel 使用字符串内容创建 Casbin 模型。
func WithStringModel(str string) OptFunc {
	return func(s *State) {
		s.model, _ = model.NewModelFromString(str)
	}
}

// WithFileModel 使用模型文件创建 Casbin 模型。
func WithFileModel(path string) OptFunc {
	return func(s *State) {
		s.model, _ = model.NewModelFromFile(path)
	}
}

// WithDefaultModel 使用内置模型模板创建 Casbin 模型。
func WithDefaultModel(name string) OptFunc {
	return func(s *State) {
		var str string
		switch name {
		case "rbac":
			str = assets.DefaultRbacModel

		case "rbac_with_domains":
			str = assets.DefaultRbacWithDomainModel

		case "abac":
			str = assets.DefaultAbacModel

		case "acl":
			str = assets.DefaultAclModel

		case "restfull":
			str = assets.DefaultRestfullModel

		case "restfull_with_role":
			str = assets.DefaultRestfullWithRoleModel
		}

		s.model, _ = model.NewModelFromString(str)
	}
}

// WithPolicyAdapter 设置策略适配器。
func WithPolicyAdapter(policy *Adapter) OptFunc {
	return func(s *State) {
		s.policy = policy
	}
}

// WithPolices 设置初始策略集合。
func WithPolices(policies map[string]interface{}) OptFunc {
	return func(s *State) {
		if s.policy == nil {
			s.policy = newAdapter()
		}
		s.policy.SetPolicies(policies)
	}
}

// WithProjects 设置项目集合。
func WithProjects(projects engine.Projects) OptFunc {
	return func(s *State) {
		s.projects = projects
	}
}

// WithWildcardItem 设置通配项目占位符。
func WithWildcardItem(item string) OptFunc {
	return func(s *State) {
		s.wildcardItem = item
	}
}

// WithAuthorizedProjectsMatcher 设置项目授权匹配表达式。
func WithAuthorizedProjectsMatcher(matcher string) OptFunc {
	return func(s *State) {
		s.authorizedProjectsMatcher = matcher
	}
}
