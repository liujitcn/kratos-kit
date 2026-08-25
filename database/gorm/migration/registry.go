package migration

import (
	"fmt"
	"slices"
)

// Registry 保存已注册模块的迁移定义。
type Registry struct {
	// migrations 按模块名称保存迁移资源，一个模块可以包含多个资源。
	migrations map[ModuleName][]Migration
	// order 保存模块注册顺序，用于校验所有模块的依赖关系。
	order []ModuleName
}

// NewRegistry 创建迁移注册表，不会自动注入任何内置模块。
func NewRegistry(contributors AdditionalMigrations) (*Registry, error) {
	registry := &Registry{
		migrations: make(map[ModuleName][]Migration),
		order:      make([]ModuleName, 0, len(contributors)),
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
		name := contributor.Name()
		if err := name.Validate(); err != nil {
			return err
		}
		if _, exists := r.migrations[name]; exists {
			return fmt.Errorf("迁移模块重复注册: %s", name)
		}
		migrations := contributor.Migrations()
		if len(migrations) == 0 {
			// 记录空模块名称，避免后续同名贡献者绕过唯一性校验。
			r.migrations[name] = nil
			r.order = append(r.order, name)
			continue
		}
		for _, migration := range migrations {
			if migration.FS == nil {
				return fmt.Errorf("迁移模块 %s 未提供文件系统", name)
			}
			if migration.Path == "" {
				return fmt.Errorf("迁移模块 %s 未提供资源路径", name)
			}
			migration.Dependencies = slices.Clone(migration.Dependencies)
			for _, dependency := range migration.Dependencies {
				if err := dependency.Validate(); err != nil {
					return fmt.Errorf("迁移模块 %s 依赖无效: %w", name, err)
				}
			}
			r.migrations[name] = append(r.migrations[name], migration)
		}
		r.order = append(r.order, name)
	}
	return r.validateDependencies()
}

// validateDependencies 校验迁移依赖是否存在且无循环。
func (r *Registry) validateDependencies() error {
	for _, name := range r.order {
		for _, dependency := range migrationDependencies(r.migrations[name]) {
			if _, exists := r.migrations[dependency]; !exists {
				return fmt.Errorf("迁移模块 %s 依赖未注册模块 %s", name, dependency)
			}
		}
	}
	visiting := make(map[ModuleName]bool)
	visited := make(map[ModuleName]bool)
	var err error
	var visit func(ModuleName) error
	visit = func(name ModuleName) error {
		if visiting[name] {
			return fmt.Errorf("迁移模块依赖存在循环: %s", name)
		}
		if visited[name] {
			return nil
		}
		visiting[name] = true
		for _, dependency := range migrationDependencies(r.migrations[name]) {
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

// migrationDependencies 返回迁移模块声明的去重依赖名称。
func migrationDependencies(migrations []Migration) []ModuleName {
	dependencies := make([]ModuleName, 0)
	visited := make(map[ModuleName]bool)
	for _, migration := range migrations {
		for _, dependency := range migration.Dependencies {
			if visited[dependency] {
				continue
			}
			visited[dependency] = true
			dependencies = append(dependencies, dependency)
		}
	}
	return dependencies
}
