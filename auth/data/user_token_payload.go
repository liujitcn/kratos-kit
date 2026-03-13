package data

import (
	authnEngine "github.com/liujitcn/kratos-kit/auth/authn/engine"
)

const (
	ClaimFieldUserID   = "uid"
	ClaimFieldRoleID   = "rid"
	ClaimFieldRoleName = "rname"
	ClaimFieldRoleCode = "rcode"
	ClaimFieldDeptID   = "did"
	ClaimFieldDeptName = "dname"
	ClaimFieldOpenID   = "oid"
)

// UserTokenPayload 用户JWT令牌载荷
type UserTokenPayload struct {
	UserId   int64
	UserName string
	RoleId   int64
	RoleCode string
	RoleName string
	DeptId   int64
	DeptName string
	OpenId   string
}

func NewUserTokenPayloadWithClaims(claims *authnEngine.AuthClaims) (*UserTokenPayload, error) {
	userToken := &UserTokenPayload{}

	if err := userToken.ExtractAuthClaims(claims); err != nil {
		return nil, err
	}

	return userToken, nil
}

// MakeAuthClaims 构建认证声明
func (t *UserTokenPayload) MakeAuthClaims() *authnEngine.AuthClaims {
	return &authnEngine.AuthClaims{
		authnEngine.ClaimFieldSubject: t.UserName,
		ClaimFieldUserID:              t.UserId,
		ClaimFieldRoleID:              t.RoleId,
		ClaimFieldRoleCode:            t.RoleCode,
		ClaimFieldRoleName:            t.RoleName,
		ClaimFieldDeptID:              t.DeptId,
		ClaimFieldDeptName:            t.DeptName,
		ClaimFieldOpenID:              t.OpenId,
	}
}

// ExtractAuthClaims 解析认证声明
func (t *UserTokenPayload) ExtractAuthClaims(claims *authnEngine.AuthClaims) error {
	var err error
	t.UserName, err = claims.GetSubject()
	if err != nil {
		return err
	}
	t.UserId, err = claims.GetInt64(ClaimFieldUserID)
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
	t.OpenId, err = claims.GetString(ClaimFieldOpenID)
	if err != nil {
		return err
	}
	return nil
}
