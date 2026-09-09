package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/liujitcn/kratos-kit/auth/authn/engine"
	"github.com/liujitcn/kratos-kit/cache"
	"github.com/redis/go-redis/v9"
)

// UserToken 管理用户访问令牌、刷新令牌和独立登录会话。
type UserToken struct {
	indexMu       sync.Mutex
	cache         cache.Cache
	authenticator engine.IdentityCreator

	accessTokenKeyPrefix  string
	refreshTokenKeyPrefix string

	accessTokenExpires  time.Duration
	refreshTokenExpires time.Duration
}

// TokenSession 描述同一用户的一组访问令牌和刷新令牌。
type TokenSession struct {
	SessionID        string `json:"session_id"`
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	AccessExpiresAt  int64  `json:"access_expires_at"`  // 访问令牌到期时间，Unix 纳秒。
	RefreshExpiresAt int64  `json:"refresh_expires_at"` // 刷新令牌到期时间，Unix 纳秒。
}

// NewUserToken 创建使用指定缓存和签发器的令牌管理器。
func NewUserToken(
	cache cache.Cache,
	authenticator engine.IdentityCreator,
	accessTokenKeyPrefix,
	refreshTokenKeyPrefix string,
	accessTokenExpires,
	refreshTokenExpires time.Duration,
) *UserToken {
	return &UserToken{
		cache:                 cache,
		authenticator:         authenticator,
		accessTokenKeyPrefix:  accessTokenKeyPrefix,
		refreshTokenKeyPrefix: refreshTokenKeyPrefix,
		accessTokenExpires:    accessTokenExpires,
		refreshTokenExpires:   refreshTokenExpires,
	}
}

// GenerateToken 创建令牌
func (r *UserToken) GenerateToken(userToken *UserTokenPayload) (accessToken string, refreshToken string, err error) {
	return r.GenerateTokenForSession(userToken, "")
}

// GenerateTokenForSession 创建指定会话的访问令牌和刷新令牌。
func (r *UserToken) GenerateTokenForSession(userToken *UserTokenPayload, sessionID string) (accessToken string, refreshToken string, err error) {
	accessToken = r.createAccessJwtToken(userToken)
	if accessToken == "" {
		return "", "", errors.New("create access token failed")
	}
	refreshToken = r.createRefreshToken()
	if sessionID == "" {
		if err = r.setAccessTokenToRedis(userToken.UserId, accessToken, r.accessTokenExpires); err != nil {
			return
		}
		err = r.setRefreshTokenToRedis(userToken.UserId, refreshToken, r.refreshTokenExpires)
		return
	}
	now := time.Now()
	session := TokenSession{SessionID: sessionID, AccessToken: accessToken, RefreshToken: refreshToken, AccessExpiresAt: now.Add(r.accessTokenExpires).UnixNano(), RefreshExpiresAt: now.Add(r.refreshTokenExpires).UnixNano()}
	if err = r.cache.Set(r.makeSessionKey(userToken.UserId, sessionID), "1", max(r.accessTokenExpires, r.refreshTokenExpires)); err != nil {
		return
	}
	err = r.saveTokenSession(userToken.UserId, session)
	return
}

// GenerateAccessToken 创建访问令牌
func (r *UserToken) GenerateAccessToken(userToken *UserTokenPayload) (accessToken string, err error) {
	return r.GenerateAccessTokenForSession(userToken, "")
}

// GenerateAccessTokenForSession 为指定会话创建访问令牌。
func (r *UserToken) GenerateAccessTokenForSession(userToken *UserTokenPayload, sessionID string) (accessToken string, err error) {
	var session TokenSession
	if sessionID != "" {
		session, err = r.getTokenSession(userToken.UserId, sessionID)
		if err != nil {
			return "", err
		}
	}
	accessToken = r.createAccessJwtToken(userToken)
	if accessToken == "" {
		return "", errors.New("create access token failed")
	}
	if sessionID == "" {
		return accessToken, r.setAccessTokenToRedis(userToken.UserId, accessToken, r.accessTokenExpires)
	}
	session.AccessToken = accessToken
	session.AccessExpiresAt = time.Now().Add(r.accessTokenExpires).UnixNano()
	if err = r.saveTokenSession(userToken.UserId, session); err != nil {
		return "", err
	}
	// 只延长仍存在的会话，不得在并发撤销后重新创建有效标记。
	err = r.cache.Expire(r.makeSessionKey(userToken.UserId, sessionID), time.Until(time.Unix(0, max(session.AccessExpiresAt, session.RefreshExpiresAt))))
	return
}

// GenerateRefreshToken 创建刷新令牌
func (r *UserToken) GenerateRefreshToken(userToken *UserTokenPayload) (refreshToken string, err error) {
	return r.GenerateRefreshTokenForSession(userToken, "")
}

// GenerateRefreshTokenForSession 为指定会话创建刷新令牌。
func (r *UserToken) GenerateRefreshTokenForSession(userToken *UserTokenPayload, sessionID string) (refreshToken string, err error) {
	var session TokenSession
	if sessionID != "" {
		session, err = r.getTokenSession(userToken.UserId, sessionID)
		if err != nil {
			return "", err
		}
	}
	refreshToken = r.createRefreshToken()
	if refreshToken == "" {
		return "", errors.New("create refresh token failed")
	}
	if sessionID == "" {
		return refreshToken, r.setRefreshTokenToRedis(userToken.UserId, refreshToken, r.refreshTokenExpires)
	}
	session.RefreshToken = refreshToken
	session.RefreshExpiresAt = time.Now().Add(r.refreshTokenExpires).UnixNano()
	if err = r.saveTokenSession(userToken.UserId, session); err != nil {
		return "", err
	}
	// 只延长仍存在的会话，不得在并发撤销后重新创建有效标记。
	err = r.cache.Expire(r.makeSessionKey(userToken.UserId, sessionID), time.Until(time.Unix(0, max(session.AccessExpiresAt, session.RefreshExpiresAt))))
	return
}

// RemoveToken 移除所有令牌
func (r *UserToken) RemoveToken(userId int64) error {
	sessions, err := r.getTokenSessions(userId)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if err = r.RemoveTokenForSession(userId, session.SessionID, session.AccessToken, session.RefreshToken); err != nil {
			return err
		}
	}
	return errors.Join(r.deleteAccessTokenFromRedis(userId), r.deleteRefreshTokenFromRedis(userId))
}

// RemoveTokenForSession 移除指定会话的访问令牌和刷新令牌。
func (r *UserToken) RemoveTokenForSession(userId int64, sessionID, accessToken, refreshToken string) error {
	if sessionID == "" {
		return r.RemoveToken(userId)
	}
	if err := r.cache.Del(r.makeSessionKey(userId, sessionID)); err != nil {
		return err
	}
	r.indexMu.Lock()
	defer r.indexMu.Unlock()
	err := r.cache.HDel(r.makeSessionIndexKey(userId), sessionID)
	if isTokenCacheMiss(err) {
		return nil
	}
	return err
}

// GetTokenSessions 返回用户当前缓存中的会话令牌列表。
func (r *UserToken) GetTokenSessions(userId int64) ([]TokenSession, error) {
	return r.getTokenSessions(userId)
}

// IsAccessTokenValid 校验访问令牌是否属于用户的有效会话。
func (r *UserToken) IsAccessTokenValid(userId int64, accessToken string) bool {
	if accessToken == "" {
		return false
	}
	if r.GetAccessToken(userId) == accessToken {
		return r.IsExistAccessToken(userId)
	}
	sessions, err := r.getTokenSessions(userId)
	if err != nil {
		return false
	}
	for _, session := range sessions {
		if session.AccessToken == accessToken && time.Now().UnixNano() < session.AccessExpiresAt {
			return true
		}
	}
	return false
}

// IsRefreshTokenValid 校验刷新令牌是否属于用户的有效会话。
func (r *UserToken) IsRefreshTokenValid(userId int64, refreshToken string) bool {
	if refreshToken == "" {
		return false
	}
	if r.GetRefreshToken(userId) == refreshToken {
		return r.IsExistRefreshToken(userId)
	}
	sessions, err := r.getTokenSessions(userId)
	if err != nil {
		return false
	}
	for _, session := range sessions {
		if session.RefreshToken == refreshToken && time.Now().UnixNano() < session.RefreshExpiresAt {
			return true
		}
	}
	return false
}

// GetAccessToken 获取访问令牌
func (r *UserToken) GetAccessToken(userId int64) string {
	return r.getAccessTokenFromRedis(userId)
}

// GetRefreshToken 获取刷新令牌
func (r *UserToken) GetRefreshToken(userId int64) string {
	return r.getRefreshTokenFromRedis(userId)
}

// IsExistAccessToken 访问令牌是否存在
func (r *UserToken) IsExistAccessToken(userId int64) bool {
	if r.cache.Exists(r.makeAccessTokenKey(userId)) {
		return true
	}
	sessions, err := r.getTokenSessions(userId)
	if err != nil {
		return false
	}
	for _, session := range sessions {
		if time.Now().UnixNano() < session.AccessExpiresAt {
			return true
		}
	}
	return false
}

// IsExistRefreshToken 刷新令牌是否存在
func (r *UserToken) IsExistRefreshToken(userId int64) bool {
	if r.cache.Exists(r.makeRefreshTokenKey(userId)) {
		return true
	}
	sessions, err := r.getTokenSessions(userId)
	if err != nil {
		return false
	}
	for _, session := range sessions {
		if time.Now().UnixNano() < session.RefreshExpiresAt {
			return true
		}
	}
	return false
}

// GetAccessTokenExpires 获取token 有效期，单位秒
func (r *UserToken) GetAccessTokenExpires() int64 {
	return int64(r.accessTokenExpires.Seconds())
}

// GetRefreshTokenExpires 获取token 有效期，单位秒
func (r *UserToken) GetRefreshTokenExpires() int64 {
	return int64(r.refreshTokenExpires.Seconds())
}

// setAccessTokenToRedis 设置访问令牌
func (r *UserToken) setAccessTokenToRedis(userId int64, token string, expires time.Duration) error {
	key := r.makeAccessTokenKey(userId)
	return r.cache.Set(key, token, expires)
}

// getAccessTokenFromRedis 获取访问令牌
func (r *UserToken) getAccessTokenFromRedis(userId int64) string {
	key := r.makeAccessTokenKey(userId)
	result, err := r.cache.Get(key)
	if err != nil {
		if !isTokenCacheMiss(err) {
			log.Error("get redis user access token failed", "error", err)
		}
		return ""
	}
	return result
}

// deleteAccessTokenFromRedis 删除访问令牌
func (r *UserToken) deleteAccessTokenFromRedis(userId int64) error {
	key := r.makeAccessTokenKey(userId)
	return r.cache.Del(key)
}

// setRefreshTokenToRedis 设置刷新令牌
func (r *UserToken) setRefreshTokenToRedis(userId int64, token string, expires time.Duration) error {
	key := r.makeRefreshTokenKey(userId)
	return r.cache.Set(key, token, expires)
}

// getRefreshTokenFromRedis 获取刷新令牌
func (r *UserToken) getRefreshTokenFromRedis(userId int64) string {
	key := r.makeRefreshTokenKey(userId)
	result, err := r.cache.Get(key)
	if err != nil {
		if !isTokenCacheMiss(err) {
			log.Error("get redis user refresh token failed", "error", err)
		}
		return ""
	}
	return result
}

// deleteRefreshTokenFromRedis 删除刷新令牌
func (r *UserToken) deleteRefreshTokenFromRedis(userId int64) error {
	key := r.makeRefreshTokenKey(userId)
	return r.cache.Del(key)
}

// createAccessJwtToken 生成JWT访问令牌
func (r *UserToken) createAccessJwtToken(userToken *UserTokenPayload) string {

	claims := userToken.MakeAuthClaims()
	(*claims)[engine.ClaimFieldJwtID] = uuid.NewV4().String()
	(*claims)[engine.ClaimFieldIssuedAt] = time.Now().Unix()
	(*claims)[engine.ClaimFieldExpirationTime] = time.Now().Add(r.accessTokenExpires).Unix()
	signedToken, err := r.authenticator.CreateIdentity(*claims)
	if err != nil {
		log.Error("create access token failed", "error", err)
	}

	return signedToken
}

// createRefreshToken 生成刷新令牌
func (r *UserToken) createRefreshToken() string {
	return uuid.NewV4().String()
}

// makeAccessTokenKey 生成访问令牌键
func (r *UserToken) makeAccessTokenKey(userId int64) string {
	return fmt.Sprintf("%s%d", r.accessTokenKeyPrefix, userId)
}

// makeRefreshTokenKey 生成刷新令牌键
func (r *UserToken) makeRefreshTokenKey(userId int64) string {
	return fmt.Sprintf("%s%d", r.refreshTokenKeyPrefix, userId)
}

// getTokenSession 读取仍有效的指定会话，撤销后不能通过刷新重新建立会话。
func (r *UserToken) getTokenSession(userID int64, sessionID string) (TokenSession, error) {
	if !r.cache.Exists(r.makeSessionKey(userID, sessionID)) {
		return TokenSession{}, errors.New("token session not found")
	}
	raw, err := r.cache.HGet(r.makeSessionIndexKey(userID), sessionID)
	if err != nil {
		return TokenSession{}, err
	}
	var session TokenSession
	err = json.Unmarshal([]byte(raw), &session)
	return session, err
}

// getTokenSessions 读取用户的会话快照，忽略已到期或撤销的会话。
func (r *UserToken) getTokenSessions(userID int64) ([]TokenSession, error) {
	r.indexMu.Lock()
	defer r.indexMu.Unlock()
	entries, err := r.cache.HGetAll(r.makeSessionIndexKey(userID))
	if err != nil {
		if isTokenCacheMiss(err) {
			return nil, nil
		}
		return nil, err
	}
	sessions := make([]TokenSession, 0, len(entries))
	for sessionID, raw := range entries {
		if !r.cache.Exists(r.makeSessionKey(userID, sessionID)) {
			if err = r.cache.HDel(r.makeSessionIndexKey(userID), sessionID); err != nil {
				return nil, err
			}
			continue
		}
		var session TokenSession
		if err = json.Unmarshal([]byte(raw), &session); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// saveTokenSession 原子替换指定会话字段，不覆盖其他设备的会话。
func (r *UserToken) saveTokenSession(userID int64, session TokenSession) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	r.indexMu.Lock()
	defer r.indexMu.Unlock()
	return r.cache.HSet(r.makeSessionIndexKey(userID), session.SessionID, string(payload))
}

// makeSessionIndexKey 返回用户的独立会话索引键。
func (r *UserToken) makeSessionIndexKey(userID int64) string {
	return fmt.Sprintf("%s:session-index:%d", r.accessTokenKeyPrefix, userID)
}

// makeSessionKey 返回会话的有效标记键，独立于令牌轮换保存。
func (r *UserToken) makeSessionKey(userID int64, sessionID string) string {
	return fmt.Sprintf("%s:session:%d:%s", r.accessTokenKeyPrefix, userID, sessionID)
}

// isTokenCacheMiss 兼容 Redis 与内存缓存的缺失键错误。
func isTokenCacheMiss(err error) bool {
	return errors.Is(err, redis.Nil) || err != nil && (strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "key expired"))
}
