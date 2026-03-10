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
	var sub string
	sub, err = claims.GetSubject()
	if err == nil {
		t.UserName = sub
	}

	var uid int64
	uid, err = claims.GetInt64(ClaimFieldUserID)
	if err == nil {
		t.UserId = uid
	}

	var rid int64
	rid, err = claims.GetInt64(ClaimFieldRoleID)
	if err == nil {
		t.RoleId = rid
	}

	var rname string
	rname, err = claims.GetString(ClaimFieldRoleName)
	if err == nil {
		t.RoleName = rname
	}

	var rcode string
	rcode, err = claims.GetString(ClaimFieldRoleCode)
	if err == nil {
		t.RoleCode = rcode
	}

	var did int64
	did, err = claims.GetInt64(ClaimFieldDeptID)
	if err == nil {
		t.DeptId = did
	}

	var dname string
	dname, err = claims.GetString(ClaimFieldDeptName)
	if err == nil {
		t.DeptName = dname
	}

	var oid string
	oid, err = claims.GetString(ClaimFieldOpenID)
	if err == nil {
		t.OpenId = oid
	}
	return nil
}
