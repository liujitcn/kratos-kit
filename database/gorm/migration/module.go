package migration

import (
	"fmt"
	"regexp"
)

// ModuleName 表示迁移执行器使用的模块名称标识。
type ModuleName string

var moduleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// String 返回模块名称的字符串值。
func (name ModuleName) String() string {
	return string(name)
}

// Validate 校验模块名称格式。
func (name ModuleName) Validate() error {
	if !moduleNamePattern.MatchString(string(name)) {
		return fmt.Errorf("迁移模块名称无效: %q，必须使用 1-64 位小写字母、数字或连字符，且以字母开头", name)
	}
	return nil
}
