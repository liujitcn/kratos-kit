package migration

import "fmt"

// Registry 保存已注册模块的迁移定义。
type Registry struct {
	// migrations 按模块名称保存迁移资源，一个模块可以包含多个资源。
	migrations map[string][]Migration
	// order 保存模块注册顺序，用于校验所有模块的依赖关系。
	order []string
}

// NewRegistry 创建迁移注册表，不会自动注入任何内置模块。
func NewRegistry(contributors AdditionalMigrations) (*Registry, error) {
	registry := &Registry{
		migrations: make(map[string][]Migration),
		order:      make([]string, 0, len(contributors)),
	}
	var err error
	err = registry.Register(contributors...)
	if err != nil {
		return nil, err
	}
	return registry, nil
}

// Register 将迁移贡献者注册到当前注册表。
func (r *Registry) Register(contributors ...Contributor) error {
	for _, contributor := range contributors {
		if contributor == nil {
			continue
		}
		migrations := contributor.Migrations()
		if len(migrations) == 0 {
			continue
		}
		name := contributor.Name()
		if name == "" {
			return fmt.Errorf("迁移模块名称不能为空")
		}
		if _, exists := r.migrations[name]; exists {
			return fmt.Errorf("迁移模块重复注册: %s", name)
		}
		for _, migration := range migrations {
			if migration.FS == nil {
				return fmt.Errorf("迁移模块 %s 未提供文件系统", name)
			}
			if migration.Path == "" {
				return fmt.Errorf("迁移模块 %s 未提供资源路径", name)
			}
			migration.Dependencies = append([]string(nil), migration.Dependencies...)
			r.migrations[name] = append(r.migrations[name], migration)
		}
		r.order = append(r.order, name)
	}
	return r.validateDependencies()
}

// validateDependencies 校验迁移依赖是否存在且无循环。
func (r *Registry) validateDependencies() error {
	for _, name := range r.order {
		for _, migration := range r.migrations[name] {
			for _, dependency := range migration.Dependencies {
				if _, exists := r.migrations[dependency]; !exists {
					return fmt.Errorf("迁移模块 %s 依赖未注册模块 %s", name, dependency)
				}
			}
		}
	}
	visiting := make(map[string]bool)
	visited := make(map[string]bool)
	var err error
	var visit func(string) error
	visit = func(name string) error {
		if visiting[name] {
			return fmt.Errorf("迁移模块依赖存在循环: %s", name)
		}
		if visited[name] {
			return nil
		}
		visiting[name] = true
		for _, dependency := range r.migrations[name][0].Dependencies {
			err = visit(dependency)
			if err != nil {
				return err
			}
		}
		visiting[name] = false
		visited[name] = true
		return nil
	}
	for _, name := range r.order {
		err = visit(name)
		if err != nil {
			return err
		}
	}
	return nil
}
