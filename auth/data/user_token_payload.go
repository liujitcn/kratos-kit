package data

import "github.com/liujitcn/kratos-kit/auth/authn/engine"

const (
	ClaimFieldTenantID   = "tid"
	ClaimFieldTenantCode = "tcode"
	ClaimFieldUserID     = "uid"
	ClaimFieldUserCode   = "ucode"
	ClaimFieldRoleID     = "rid"
	ClaimFieldRoleName   = "rname"
	ClaimFieldRoleCode   = "rcode"
	ClaimFieldDeptID     = "did"
	ClaimFieldDeptName   = "dname"
	ClaimFieldDataScope  = "ds"
)

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
	return &engine.AuthClaims{
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
	return nil
}
