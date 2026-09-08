package main

import (
	"fmt"
	"regexp"
	"strings"
)

// parseBusinessModules 校验业务模块清单，未指定时仅生成 system 模块。
func parseBusinessModules(value string) ([]string, error) {
	if value == "" {
		return []string{"system"}, nil
	}
	modules := strings.Split(value, ",")
	pattern := regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	seen := make(map[string]bool)
	for _, name := range modules {
		if !pattern.MatchString(name) || name == "core" {
			return nil, fmt.Errorf("无效的业务模块名 %q：使用小写字母、数字或连字符，不能使用 core", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("业务模块名重复: %s", name)
		}
		seen[name] = true
	}
	return modules, nil
}
