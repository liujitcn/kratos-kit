package ent

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"entgo.io/ent"
	"github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/liujitcn/kratos-kit/auth"
)

var auditExcludeTypes = []string{
	"BaseLog",
	"base_log",
}

var auditExcludeTypesMu sync.RWMutex

// AuditHook 返回用于回填 Ent 审计字段的 mutation hook。
func AuditHook() ent.Hook {
	return func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
			if shouldSkipAudit(mutation) {
				return next.Mutate(ctx, mutation)
			}

			if mutation.Op().Is(ent.OpCreate) {
				err := fillCreatedFields(ctx, mutation)
				if err != nil {
					return nil, err
				}
			}
			if mutation.Op().Is(ent.OpUpdate | ent.OpUpdateOne) {
				err := fillUpdatedFields(ctx, mutation)
				if err != nil {
					return nil, err
				}
			}
			return next.Mutate(ctx, mutation)
		})
	}
}

// SetAuditExcludeTypes 设置跳过审计字段回填的 Ent 类型名称。
func SetAuditExcludeTypes(types ...string) {
	auditExcludeTypesMu.Lock()
	defer auditExcludeTypesMu.Unlock()
	auditExcludeTypes = types
}

// shouldSkipAudit 判断当前 mutation 是否需要跳过审计字段回填。
func shouldSkipAudit(mutation ent.Mutation) bool {
	if mutation == nil {
		return true
	}
	auditExcludeTypesMu.RLock()
	defer auditExcludeTypesMu.RUnlock()
	return slices.Contains(auditExcludeTypes, mutation.Type())
}

// fillCreatedFields 在创建时回填审计字段。
func fillCreatedFields(ctx context.Context, mutation ent.Mutation) error {
	now := time.Now()
	var createdBySet bool
	createdBySet, err := setFieldIfUnset(mutation, "created_by", int64(0))
	if err != nil {
		return err
	}
	var updatedBySet bool
	updatedBySet, err = setFieldIfUnset(mutation, "updated_by", int64(0))
	if err != nil {
		return err
	}
	if createdBySet || updatedBySet {
		userID := getUserIDFromContext(ctx)
		if createdBySet {
			err = mutation.SetField("created_by", userID)
			if err != nil {
				return err
			}
		}
		if updatedBySet {
			err = mutation.SetField("updated_by", userID)
			if err != nil {
				return err
			}
		}
	}

	_, err = setFieldIfUnset(mutation, "created_at", now)
	if err != nil {
		return err
	}
	_, err = setFieldIfUnset(mutation, "updated_at", now)
	if err != nil {
		return err
	}
	return nil
}

// fillUpdatedFields 在更新时回填审计字段。
func fillUpdatedFields(ctx context.Context, mutation ent.Mutation) error {
	var updatedBySet bool
	updatedBySet, err := setFieldIfUnset(mutation, "updated_by", int64(0))
	if err != nil {
		return err
	}
	if updatedBySet {
		err = mutation.SetField("updated_by", getUserIDFromContext(ctx))
		if err != nil {
			return err
		}
	}

	_, err = setFieldIfUnset(mutation, "updated_at", time.Now())
	if err != nil {
		return err
	}
	return nil
}

// setFieldIfUnset 在字段未被调用方显式设置时回填默认值。
func setFieldIfUnset(mutation ent.Mutation, fieldName string, value ent.Value) (bool, error) {
	if _, ok := mutation.Field(fieldName); ok {
		return false, nil
	}
	err := mutation.SetField(fieldName, value)
	if err != nil {
		// Ent 生成的 mutation 会在字段不存在时返回 unknown xxx field name。
		if isUnknownFieldError(err, fieldName) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// isUnknownFieldError 判断错误是否表示 Ent schema 未声明目标字段。
func isUnknownFieldError(err error, fieldName string) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.HasPrefix(message, "unknown ") && strings.HasSuffix(message, " field "+fieldName)
}

// getUserIDFromContext 从上下文中解析当前用户ID。
func getUserIDFromContext(ctx context.Context) int64 {
	if ctx == nil || ctx == context.Background() || ctx == context.TODO() || isAppLifecycleContext(ctx) {
		return 0
	}

	userInfo, err := auth.FromContext(ctx)
	if err != nil {
		log.Warn("context has no user info, use default user id")
		return 0
	}
	if userInfo == nil {
		log.Error("get user id failed, use default user id")
		return 0
	}

	return userInfo.UserId
}

// isAppLifecycleContext 判断当前上下文是否为 Kratos 应用生命周期上下文。
func isAppLifecycleContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}

	// 请求上下文会额外挂载 transport 信息，不能按应用生命周期上下文处理。
	if _, ok := transport.FromServerContext(ctx); ok {
		return false
	}

	_, ok := kratos.FromContext(ctx)
	return ok
}
