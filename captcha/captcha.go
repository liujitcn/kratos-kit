package captcha

import (
	"context"
	"fmt"

	cachekit "github.com/liujitcn/kratos-kit/cache"
	"github.com/liujitcn/kratos-kit/captcha/driver"
)

var _ Service = (*Captcha)(nil)

// Captcha 提供验证码生成和校验能力。
type Captcha struct {
	store  *cacheStore // 答案缓存适配器
	config *Config     // 验证码配置
	driver driver.Driver
}

// NewCaptcha 创建验证码实例（使用 Options 模式）。
func NewCaptcha(cache cachekit.Cache, opts ...Option) *Captcha {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(config)
	}
	return NewCaptchaWithConfig(cache, config)
}

// NewCaptchaWithConfig 使用自定义配置创建实例。
func NewCaptchaWithConfig(cache cachekit.Cache, config *Config) *Captcha {
	// 未传配置时使用默认配置，避免调用方只想快速启用时需要补全所有字段。
	if config == nil {
		config = DefaultConfig()
	}
	return &Captcha{
		store:  newCacheStore(cache),
		config: config,
		driver: driver.New(config),
	}
}

// Generate 生成验证码并自动保存答案。
func (c *Captcha) Generate(ctx context.Context) (*Challenge, error) {
	var challenge *driver.Challenge
	var err error
	challenge, err = c.driver.Generate()
	if err != nil {
		return nil, err
	}
	// 答案只写入缓存，避免业务接口把服务端答案返回给前端。
	err = c.store.Set(c.cacheKey(challenge.ID), challenge.Answer, c.config.Expire)
	if err != nil {
		return nil, err
	}
	return &Challenge{ID: challenge.ID, Payload: challenge.Payload}, nil
}

// Verify 从缓存读取并校验验证码，验证成功自动删除。
func (c *Captcha) Verify(ctx context.Context, captchaID, userInput string) (bool, error) {
	ans := c.store.Get(c.cacheKey(captchaID))
	// 缓存不存在通常表示验证码过期、已验证或 ID 无效。
	if ans == "" {
		return false, nil
	}

	match := c.driver.Verify(ans, userInput)
	// 验证成功后删除答案，降低重复提交和重放风险。
	if match {
		_ = c.store.Delete(c.cacheKey(captchaID))
	}
	return match, nil
}

// cacheKey 生成验证码缓存键。
func (c *Captcha) cacheKey(captchaID string) string {
	return fmt.Sprintf("%s:%s", c.config.KeyPrefix, captchaID)
}
