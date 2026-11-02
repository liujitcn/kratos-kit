package data

import (
	"strconv"
	"testing"
	"time"

	"github.com/liujitcn/kratos-kit/auth/authn/engine"
	kitcache "github.com/liujitcn/kratos-kit/cache"
	"github.com/liujitcn/kratos-kit/cache/memory"
)

type stubIdentityCreator struct{ seq int }

// CreateIdentity 创建测试用伪令牌。
func (s *stubIdentityCreator) CreateIdentity(claims engine.AuthClaims) (string, error) {
	s.seq++
	return "stub-token-" + time.Now().Format("150405.000000000") + "-" + strconv.Itoa(s.seq), nil
}

// TestTokenSessionGracePeriod 验证轮换后旧访问令牌在宽限期内可用。
func TestTokenSessionGracePeriod(t *testing.T) {
	memoryCache, cleanup, err := memory.NewMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	var _ kitcache.Cache = memoryCache
	userToken := NewUserToken(memoryCache, &stubIdentityCreator{}, "test:access:", "test:refresh:", time.Hour, 30*24*time.Hour)
	payload := &UserTokenPayload{UserId: 1, TenantCode: "0000", RoleCode: "super"}

	access1, _, err := userToken.GenerateTokenForSession(payload, "session-1")
	if err != nil {
		t.Fatal(err)
	}
	if !userToken.IsAccessTokenValid(1, access1) {
		t.Fatal("轮换前旧令牌应有效")
	}
	access2, err := userToken.GenerateAccessTokenForSession(payload, "session-1")
	if err != nil {
		t.Fatal(err)
	}
	if access1 == access2 {
		t.Fatal("轮换应生成新令牌")
	}
	if !userToken.IsAccessTokenValid(1, access2) {
		t.Fatal("新令牌应有效")
	}
	if !userToken.IsAccessTokenValid(1, access1) {
		t.Fatal("轮换后旧令牌在宽限期内应有效")
	}
	sessions, err := userToken.GetTokenSessions(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].PreviousAccessToken != access1 {
		t.Fatalf("会话应保留旧令牌快照: %+v", sessions)
	}
	if sessions[0].PreviousAccessExpiresAt > time.Now().Add(accessTokenGracePeriod).UnixNano() {
		t.Fatal("宽限期不应超过上限")
	}
}
