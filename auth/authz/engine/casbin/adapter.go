package casbin

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/casbin/casbin/v2/model"
)

// Adapter 从项目策略映射向 Casbin 模型加载只读策略。
type Adapter struct {
	mu       sync.RWMutex
	policies map[string]interface{}
}

// newAdapter 创建空策略适配器。
func newAdapter() *Adapter {
	return &Adapter{
		policies: map[string]interface{}{},
	}
}

// LoadPolicy 将当前策略快照加载到 Casbin 模型。
func (sa *Adapter) LoadPolicy(casbinModel model.Model) error {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	policiesInterface, ok := sa.policies["policies"]
	if ok {
		policies, typeOK := policiesInterface.([]PolicyRule)
		if !typeOK {
			return fmt.Errorf("casbin policies must be []PolicyRule, got %T", policiesInterface)
		}
		var err error
		for _, line := range policies {
			err = line.LoadPolicyLine(casbinModel)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// SavePolicy 返回不支持写回完整策略的错误。
func (sa *Adapter) SavePolicy(_ model.Model) error {
	return errors.New("not implemented")
}

// AddPolicy 返回不支持增量新增策略的错误。
func (sa *Adapter) AddPolicy(_ string, _ string, _ []string) error {
	return errors.New("not implemented")
}

// RemovePolicy 返回不支持增量删除策略的错误。
func (sa *Adapter) RemovePolicy(_ string, _ string, _ []string) error {
	return errors.New("not implemented")
}

// RemoveFilteredPolicy 返回不支持过滤删除策略的错误。
func (sa *Adapter) RemoveFilteredPolicy(_ string, _ string, _ int, _ ...string) error {
	return errors.New("not implemented")
}

// SetPolicies 替换下一次加载使用的策略映射。
func (sa *Adapter) SetPolicies(policies map[string]interface{}) {
	policySnapshot := make(map[string]interface{}, len(policies))
	for key, value := range policies {
		if rules, ok := value.([]PolicyRule); ok {
			policySnapshot[key] = slices.Clone(rules)
			continue
		}
		policySnapshot[key] = value
	}
	sa.mu.Lock()
	defer sa.mu.Unlock()
	sa.policies = policySnapshot
}
