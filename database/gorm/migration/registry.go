package migration

import "fmt"

// Registry 保存已注册模块的迁移定义。
type Registry struct {
	// specs 按模块名称保存迁移资源，一个模块可以包含多个目标资源。
	specs map[string][]MigrationSpec
	// order 保存模块注册顺序，用于校验所有模块的依赖关系。
	order []string
}

// NewRegistry 创建迁移注册表，不会自动注入任何内置模块。
func NewRegistry(contributors AdditionalMigrations) (*Registry, error) {
	registry := &Registry{
		specs: make(map[string][]MigrationSpec),
		order: make([]string, 0, len(contributors)),
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
		specs := contributor.Migrations()
		if len(specs) == 0 {
			continue
		}
		name := specs[0].Name
		if name == "" {
			return fmt.Errorf("迁移模块名称不能为空")
		}
		if _, exists := r.specs[name]; exists {
			return fmt.Errorf("迁移模块重复注册: %s", name)
		}
		for _, spec := range specs {
			if spec.Name != name {
				return fmt.Errorf("迁移模块包含多个名称: %s", name)
			}
			if spec.FS == nil {
				return fmt.Errorf("迁移模块 %s 未提供文件系统", name)
			}
			if spec.Path == "" {
				return fmt.Errorf("迁移模块 %s 未提供资源路径", name)
			}
			if spec.Target == "" {
				spec.Target = DefaultTarget
			}
			spec.Dependencies = append([]string(nil), spec.Dependencies...)
			r.specs[name] = append(r.specs[name], spec)
		}
		r.order = append(r.order, name)
	}
	return r.validateDependencies()
}

// validateDependencies 校验迁移依赖是否存在且无循环。
func (r *Registry) validateDependencies() error {
	for _, name := range r.order {
		for _, spec := range r.specs[name] {
			for _, dependency := range spec.Dependencies {
				if _, exists := r.specs[dependency]; !exists {
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
		for _, dependency := range r.specs[name][0].Dependencies {
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
