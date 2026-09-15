package data

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/liujitcn/kratos-kit/auth/authn/engine"
	"github.com/liujitcn/kratos-kit/cache/memory"
)

// claimsIdentityCreator 保留签发声明以验证不同会话的令牌唯一性。
type claimsIdentityCreator struct{}

// CreateIdentity 序列化真实签发声明作为测试令牌。
func (claimsIdentityCreator) CreateIdentity(claims engine.AuthClaims) (string, error) {
	payload, err := json.Marshal(claims)
	return string(payload), err
}

// TestSessionRotationAndRevocation 验证两个设备轮换令牌后仍独立有效，单设备退出不影响另一台。
func TestSessionRotationAndRevocation(t *testing.T) {
	store, cleanup, err := memory.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	manager := NewUserToken(store, claimsIdentityCreator{}, "access:", "refresh:", time.Hour, 24*time.Hour)
	payload := &UserTokenPayload{UserId: 1, UserName: "alice"}
	var firstAccess, firstRefresh string
	firstAccess, firstRefresh, err = manager.GenerateTokenForSession(payload, "first")
	if err != nil {
		t.Fatal(err)
	}
	var secondAccess, secondRefresh string
	secondAccess, secondRefresh, err = manager.GenerateTokenForSession(payload, "second")
	if err != nil {
		t.Fatal(err)
	}
	if !manager.IsExistAccessToken(1) || !manager.IsExistRefreshToken(1) {
		t.Fatal("Core 的用户级存在性检查必须识别独立会话")
	}
	if firstAccess == secondAccess {
		t.Fatal("不同会话不得共享访问令牌")
	}
	oldAccess, oldRefresh := firstAccess, firstRefresh
	firstAccess, err = manager.GenerateAccessTokenForSession(payload, "first")
	if err != nil {
		t.Fatal(err)
	}
	firstRefresh, err = manager.GenerateRefreshTokenForSession(payload, "first")
	if err != nil {
		t.Fatal(err)
	}
	secondAccess, err = manager.GenerateAccessTokenForSession(payload, "second")
	if err != nil {
		t.Fatal(err)
	}
	secondRefresh, err = manager.GenerateRefreshTokenForSession(payload, "second")
	if err != nil {
		t.Fatal(err)
	}
	if !manager.IsAccessTokenValid(1, firstAccess) || !manager.IsRefreshTokenValid(1, firstRefresh) {
		t.Fatal("第二台设备刷新后，第一台设备的轮换令牌应仍有效")
	}
	if manager.IsAccessTokenValid(1, oldAccess) || manager.IsRefreshTokenValid(1, oldRefresh) {
		t.Fatal("轮换前的令牌必须失效")
	}
	if err = manager.RemoveTokenForSession(1, "second", secondAccess, secondRefresh); err != nil {
		t.Fatal(err)
	}
	if !manager.IsAccessTokenValid(1, firstAccess) || manager.IsAccessTokenValid(1, secondAccess) {
		t.Fatal("单设备撤销必须只影响指定会话")
	}
	if err = manager.RemoveToken(1); err != nil {
		t.Fatal(err)
	}
	if manager.IsAccessTokenValid(1, firstAccess) || manager.IsRefreshTokenValid(1, firstRefresh) {
		t.Fatal("撤销全部会话后不得继续刷新")
	}
}

// TestUserTokenPayloadTenantProjects 验证租户项目范围可以在认证声明中完整往返。
func TestUserTokenPayloadTenantProjects(t *testing.T) {
	payload := &UserTokenPayload{
		TenantId: 1,
		UserId:   7,
		TenantProjects: []TenantProjectScope{
			{TenantId: 1, ProjectId: []int64{101, 102}},
			{TenantId: 2, ProjectId: []int64{0}},
		},
	}
	claims := payload.MakeAuthClaims()
	decoded, err := NewUserTokenPayloadWithClaims(claims)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.TenantProjects, payload.TenantProjects) {
		t.Fatalf("租户项目范围往返失败：got=%v want=%v", decoded.TenantProjects, payload.TenantProjects)
	}
	encoded, err := json.Marshal(*claims)
	if err != nil {
		t.Fatal(err)
	}
	var decodedClaims engine.AuthClaims
	if err = json.Unmarshal(encoded, &decodedClaims); err != nil {
		t.Fatal(err)
	}
	decoded, err = NewUserTokenPayloadWithClaims(&decodedClaims)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.TenantProjects, payload.TenantProjects) {
		t.Fatalf("JSON Claims 租户项目范围往返失败：got=%v want=%v", decoded.TenantProjects, payload.TenantProjects)
	}
}

// TestUserTokenPayloadTenantProjectsInvalid 验证非法全部标记和重复项目不会进入认证载荷。
func TestUserTokenPayloadTenantProjectsInvalid(t *testing.T) {
	for _, value := range []interface{}{
		[]TenantProjectScope{{TenantId: 1, ProjectId: []int64{0, 101}}},
		[]TenantProjectScope{{TenantId: 1, ProjectId: []int64{101, 101}}},
		[]TenantProjectScope{{TenantId: 0, ProjectId: []int64{101}}},
	} {
		claims := engine.AuthClaims{ClaimFieldTenantProjects: value}
		_, err := NewUserTokenPayloadWithClaims(&claims)
		if err == nil {
			t.Fatalf("应拒绝非法项目范围：%v", value)
		}
	}
}
