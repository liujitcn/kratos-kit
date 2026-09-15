package data

import (
	"encoding/json"

	"github.com/liujitcn/kratos-kit/auth/authn/engine"
)

const (
	ClaimFieldTenantID       = "tid"
	ClaimFieldTenantCode     = "tcode"
	ClaimFieldUserID         = "uid"
	ClaimFieldUserCode       = "ucode"
	ClaimFieldRoleID         = "rid"
	ClaimFieldRoleName       = "rname"
	ClaimFieldRoleCode       = "rcode"
	ClaimFieldDeptID         = "did"
	ClaimFieldDeptName       = "dname"
	ClaimFieldDataScope      = "ds"
	ClaimFieldTenantProjects = "tprojects"
)

// TenantProjectScope 表示用户在一个目标租户下的项目范围，项目 ID 为 0 时表示全部项目。
type TenantProjectScope struct {
	TenantId  int64   `json:"tenant_id"`
	ProjectId []int64 `json:"project_id"`
}

// UserTokenPayload 用户JWT令牌载荷
type UserTokenPayload struct {
	TenantId   int64
	TenantCode string
	UserId     int64
	UserCode   string
	UserName   string
	RoleId     int64
	RoleCode   string
	RoleName   string
	DeptId     int64
	DeptName   string
	DataScope  int32
	// TenantProjects 保存登录用户按目标租户分组的项目范围。
	TenantProjects []TenantProjectScope
}

func NewUserTokenPayloadWithClaims(claims *engine.AuthClaims) (*UserTokenPayload, error) {
	userToken := &UserTokenPayload{}

	if err := userToken.ExtractAuthClaims(claims); err != nil {
		return nil, err
	}

	return userToken, nil
}

// MakeAuthClaims 构建认证声明
func (t *UserTokenPayload) MakeAuthClaims() *engine.AuthClaims {
	claims := &engine.AuthClaims{
		engine.ClaimFieldSubject: t.UserName,
		ClaimFieldTenantID:       t.TenantId,
		ClaimFieldTenantCode:     t.TenantCode,
		ClaimFieldUserID:         t.UserId,
		ClaimFieldUserCode:       t.UserCode,
		ClaimFieldRoleID:         t.RoleId,
		ClaimFieldRoleCode:       t.RoleCode,
		ClaimFieldRoleName:       t.RoleName,
		ClaimFieldDeptID:         t.DeptId,
		ClaimFieldDeptName:       t.DeptName,
		ClaimFieldDataScope:      t.DataScope,
	}
	if t.TenantProjects != nil {
		(*claims)[ClaimFieldTenantProjects] = t.TenantProjects
	}
	return claims
}

// ExtractAuthClaims 解析认证声明
func (t *UserTokenPayload) ExtractAuthClaims(claims *engine.AuthClaims) error {
	var err error
	t.UserName, err = claims.GetSubject()
	if err != nil {
		return err
	}
	t.TenantId, err = claims.GetInt64(ClaimFieldTenantID)
	if err != nil {
		return err
	}
	t.TenantCode, err = claims.GetString(ClaimFieldTenantCode)
	if err != nil {
		return err
	}
	t.UserId, err = claims.GetInt64(ClaimFieldUserID)
	if err != nil {
		return err
	}
	t.UserCode, err = claims.GetString(ClaimFieldUserCode)
	if err != nil {
		return err
	}
	t.RoleId, err = claims.GetInt64(ClaimFieldRoleID)
	if err != nil {
		return err
	}
	t.RoleName, err = claims.GetString(ClaimFieldRoleName)
	if err != nil {
		return err
	}
	t.RoleCode, err = claims.GetString(ClaimFieldRoleCode)
	if err != nil {
		return err
	}
	t.DeptId, err = claims.GetInt64(ClaimFieldDeptID)
	if err != nil {
		return err
	}
	t.DeptName, err = claims.GetString(ClaimFieldDeptName)
	if err != nil {
		return err
	}
	t.DataScope, err = claims.GetInt32(ClaimFieldDataScope)
	if err != nil {
		return err
	}
	value, exists := (*claims)[ClaimFieldTenantProjects]
	if exists {
		var encoded []byte
		encoded, err = json.Marshal(value)
		if err != nil {
			return err
		}
		err = json.Unmarshal(encoded, &t.TenantProjects)
		if err != nil {
			return err
		}
		if err = validateTenantProjects(t.TenantProjects); err != nil {
			return err
		}
	}
	return nil
}

// validateTenantProjects 校验租户项目范围的结构和全部项目标记。
func validateTenantProjects(scopes []TenantProjectScope) error {
	for _, scope := range scopes {
		if scope.TenantId <= 0 || scope.ProjectId == nil {
			return engine.ErrorInvalidType
		}
		seen := make(map[int64]struct{}, len(scope.ProjectId))
		for _, projectID := range scope.ProjectId {
			if projectID < 0 {
				return engine.ErrorInvalidType
			}
			if _, exists := seen[projectID]; exists {
				return engine.ErrorInvalidType
			}
			seen[projectID] = struct{}{}
			if projectID == 0 && len(scope.ProjectId) != 1 {
				return engine.ErrorInvalidType
			}
		}
	}
	return nil
}
