package data

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	authnEngine "github.com/liujitcn/kratos-kit/auth/authn/engine"
	"github.com/liujitcn/kratos-kit/cache"
	"github.com/redis/go-redis/v9"
)

type UserToken struct {
	cache         cache.Cache
	authenticator authnEngine.Authenticator

	accessTokenKeyPrefix  string
	refreshTokenKeyPrefix string

	accessTokenExpires  time.Duration
	refreshTokenExpires time.Duration
}

func NewUserToken(
	cache cache.Cache,
	authenticator authnEngine.Authenticator,
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
	if accessToken = r.createAccessJwtToken(userToken); accessToken == "" {
		err = errors.New("create access token failed")
		return
	}

	if err = r.setAccessTokenToRedis(userToken.UserId, accessToken, r.accessTokenExpires); err != nil {
		return
	}

	if refreshToken = r.createRefreshToken(); refreshToken == "" {
		err = errors.New("create refresh token failed")
		return
	}

	if err = r.setRefreshTokenToRedis(userToken.UserId, refreshToken, r.refreshTokenExpires); err != nil {
		return
	}

	return
}

// GenerateAccessToken 创建访问令牌
func (r *UserToken) GenerateAccessToken(userToken *UserTokenPayload) (accessToken string, err error) {
	if accessToken = r.createAccessJwtToken(userToken); accessToken == "" {
		err = errors.New("create access token failed")
		return
	}

	if err = r.setAccessTokenToRedis(userToken.UserId, accessToken, r.accessTokenExpires); err != nil {
		return
	}

	return
}

// GenerateRefreshToken 创建刷新令牌
func (r *UserToken) GenerateRefreshToken(userToken *UserTokenPayload) (refreshToken string, err error) {
	if refreshToken = r.createRefreshToken(); refreshToken == "" {
		err = errors.New("create refresh token failed")
		return
	}

	if err = r.setRefreshTokenToRedis(userToken.UserId, refreshToken, r.refreshTokenExpires); err != nil {
		return
	}

	return
}

// RemoveToken 移除所有令牌
func (r *UserToken) RemoveToken(userId int64) error {
	var err error
	if err = r.deleteAccessTokenFromRedis(userId); err != nil {
		log.Errorf("remove user access token failed: [%v]", err)
	}

	if err = r.deleteRefreshTokenFromRedis(userId); err != nil {
		log.Errorf("remove user refresh token failed: [%v]", err)
	}

	return err
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
	key := r.makeAccessTokenKey(userId)
	return r.cache.Exists(key)
}

// IsExistRefreshToken 刷新令牌是否存在
func (r *UserToken) IsExistRefreshToken(userId int64) bool {
	key := r.makeRefreshTokenKey(userId)
	return r.cache.Exists(key)
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
		if !errors.Is(err, redis.Nil) {
			log.Errorf("get redis user access token failed: %s", err.Error())
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
		if !errors.Is(err, redis.Nil) {
			log.Errorf("get redis user refresh token failed: %s", err.Error())
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

	signedToken, err := r.authenticator.CreateIdentity(*userToken.MakeAuthClaims())
	if err != nil {
		log.Error("create access token failed: [%v]", err)
	}

	return signedToken
}

// createRefreshToken 生成刷新令牌
func (r *UserToken) createRefreshToken() string {
	strUUID := uuid.New()
	return strUUID.String()
}

// makeAccessTokenKey 生成访问令牌键
func (r *UserToken) makeAccessTokenKey(userId int64) string {
	return fmt.Sprintf("%s%d", r.accessTokenKeyPrefix, userId)
}

// makeRefreshTokenKey 生成刷新令牌键
func (r *UserToken) makeRefreshTokenKey(userId int64) string {
	return fmt.Sprintf("%s%d", r.refreshTokenKeyPrefix, userId)
}
