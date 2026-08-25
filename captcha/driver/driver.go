package driver

import (
	"strings"
	"uuid"
)

// Challenge 表示驱动内部生成结果，Answer 只允许写入服务端缓存。
type Challenge struct {
	ID      string // 验证码 ID
	Payload string // 图片 base64 或行为验证码 JSON(base64)
	Answer  string // 服务端答案，不对外返回
}

// Driver 定义单类验证码的生成与校验能力。
type Driver interface {
	Generate() (*Challenge, error)
	Verify(expected, actual string) bool
}

// newUUIDNoHyphen 生成不带连字符的随机 UUID。
func newUUIDNoHyphen() string {
	return strings.ReplaceAll(uuid.NewV4().String(), "-", "")
}

// New 根据配置创建对应的验证码驱动。
func New(config *Config) Driver {
	// 内部兜底默认配置，保证直接调用 New 的测试路径也可用。
	if config == nil {
		config = DefaultConfig()
	}

	// 行为验证码需要专用生成和容差校验逻辑。
	switch config.DriverType {
	case DriverSlide:
		return newSlideDriver(config.SlideConfig)
	case DriverClick:
		return newClickDriver(config.ClickConfig)
	case DriverRotate:
		return newRotateDriver(config.RotateConfig)
	default:
		return newImageDriver(config)
	}
}
