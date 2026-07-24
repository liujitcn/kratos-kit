package casbin

import (
	"strings"

	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
)

type PolicyRule struct {
	PType string `json:"p_type,omitempty"`
	V0    string `json:"v0,omitempty"`
	V1    string `json:"v1,omitempty"`
	V2    string `json:"v2,omitempty"`
	V3    string `json:"v3,omitempty"`
	V4    string `json:"v4,omitempty"`
	V5    string `json:"v5,omitempty"`
}

// LoadPolicyLine 将策略行按当前 Casbin 模型格式加载到内存模型。
func (line PolicyRule) LoadPolicyLine(model model.Model) error {
	lineText := line.PType
	values := []string{line.V0, line.V1, line.V2, line.V3, line.V4, line.V5}
	for len(values) > 0 && values[len(values)-1] == "" {
		values = values[:len(values)-1]
	}
	if len(values) > 0 {
		lineText += ", " + strings.Join(values, ", ")
	}
	return persist.LoadPolicyLine(lineText, model)
}
