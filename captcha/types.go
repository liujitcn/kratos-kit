package captcha

import (
	"context"

	"github.com/liujitcn/kratos-kit/captcha/driver"
)

// Service 定义验证码对外服务能力。
type Service interface {
	// Generate 生成验证码，自动保存服务端答案，并只返回前端需要的 ID 与图片数据。
	Generate(ctx context.Context) (*Challenge, error)
	// Verify 校验用户输入，校验成功后自动删除缓存中的答案。
	Verify(ctx context.Context, captchaID, userInput string) (bool, error)
}

// Challenge 表示一次验证码生成结果。
type Challenge struct {
	ID      string `json:"id"`      // 验证码 ID
	Payload string `json:"payload"` // 图片 base64 或行为验证码 JSON(base64)
}

// DriverType 表示验证码驱动类型。
type DriverType = driver.DriverType

const (
	// DriverDigit 表示数字验证码。
	DriverDigit = driver.DriverDigit
	// DriverString 表示字符串验证码。
	DriverString = driver.DriverString
	// DriverMath 表示算术验证码。
	DriverMath = driver.DriverMath
	// DriverChinese 表示中文验证码。
	DriverChinese = driver.DriverChinese
	// DriverSlide 表示滑动拼图验证码。
	DriverSlide = driver.DriverSlide
	// DriverClick 表示点击文字验证码。
	DriverClick = driver.DriverClick
	// DriverRotate 表示旋转验证码。
	DriverRotate = driver.DriverRotate
)

// DigitConfig 数字验证码配置。
type DigitConfig = driver.DigitConfig

// StringConfig 字符串验证码配置。
type StringConfig = driver.StringConfig

// MathConfig 算术验证码配置。
type MathConfig = driver.MathConfig

// ChineseConfig 中文验证码配置。
type ChineseConfig = driver.ChineseConfig

// SlideConfig 滑动拼图验证码配置。
type SlideConfig = driver.SlideConfig

// ClickConfig 点击文字验证码配置。
type ClickConfig = driver.ClickConfig

// RotateConfig 旋转验证码配置。
type RotateConfig = driver.RotateConfig

// Config 验证码总配置。
type Config = driver.Config

// Option 表示验证码配置选项函数。
type Option = driver.Option

// SlideCaptchaData 滑动验证码数据结构。
type SlideCaptchaData = driver.SlideCaptchaData

// ClickCaptchaData 点击验证码数据结构。
type ClickCaptchaData = driver.ClickCaptchaData

// RotateCaptchaData 旋转验证码数据结构。
type RotateCaptchaData = driver.RotateCaptchaData

var (
	// DefaultDigitConfig 返回默认数字验证码配置。
	DefaultDigitConfig = driver.DefaultDigitConfig
	// DefaultStringConfig 返回默认字符串验证码配置。
	DefaultStringConfig = driver.DefaultStringConfig
	// DefaultMathConfig 返回默认算术验证码配置。
	DefaultMathConfig = driver.DefaultMathConfig
	// DefaultChineseConfig 返回默认中文验证码配置。
	DefaultChineseConfig = driver.DefaultChineseConfig
	// DefaultSlideConfig 返回默认滑动拼图验证码配置。
	DefaultSlideConfig = driver.DefaultSlideConfig
	// DefaultClickConfig 返回默认点击文字验证码配置。
	DefaultClickConfig = driver.DefaultClickConfig
	// DefaultRotateConfig 返回默认旋转验证码配置。
	DefaultRotateConfig = driver.DefaultRotateConfig
	// DefaultConfig 返回默认验证码总配置。
	DefaultConfig = driver.DefaultConfig
	// WithDriverType 设置驱动类型。
	WithDriverType = driver.WithDriverType
	// WithExpire 设置过期时间。
	WithExpire = driver.WithExpire
	// WithKeyPrefix 设置缓存 key 前缀。
	WithKeyPrefix = driver.WithKeyPrefix
	// WithDigitConfig 设置数字验证码配置。
	WithDigitConfig = driver.WithDigitConfig
	// WithStringConfig 设置字符串验证码配置。
	WithStringConfig = driver.WithStringConfig
	// WithMathConfig 设置算术验证码配置。
	WithMathConfig = driver.WithMathConfig
	// WithChineseConfig 设置中文验证码配置。
	WithChineseConfig = driver.WithChineseConfig
	// WithDigitHeight 设置数字验证码高度。
	WithDigitHeight = driver.WithDigitHeight
	// WithDigitWidth 设置数字验证码宽度。
	WithDigitWidth = driver.WithDigitWidth
	// WithDigitCount 设置数字验证码字符数量。
	WithDigitCount = driver.WithDigitCount
	// WithDigitMaxSkew 设置数字验证码最大倾斜度。
	WithDigitMaxSkew = driver.WithDigitMaxSkew
	// WithDigitDotCount 设置数字验证码干扰点数量。
	WithDigitDotCount = driver.WithDigitDotCount
	// WithStringHeight 设置字符串验证码高度。
	WithStringHeight = driver.WithStringHeight
	// WithStringWidth 设置字符串验证码宽度。
	WithStringWidth = driver.WithStringWidth
	// WithStringCount 设置字符串验证码字符数量。
	WithStringCount = driver.WithStringCount
	// WithStringSource 设置字符串验证码字符源。
	WithStringSource = driver.WithStringSource
	// WithStringDotCount 设置字符串验证码干扰点数量。
	WithStringDotCount = driver.WithStringDotCount
	// WithMathHeight 设置算术验证码高度。
	WithMathHeight = driver.WithMathHeight
	// WithMathWidth 设置算术验证码宽度。
	WithMathWidth = driver.WithMathWidth
	// WithMathDotCount 设置算术验证码干扰点数量。
	WithMathDotCount = driver.WithMathDotCount
	// WithChineseHeight 设置中文验证码高度。
	WithChineseHeight = driver.WithChineseHeight
	// WithChineseWidth 设置中文验证码宽度。
	WithChineseWidth = driver.WithChineseWidth
	// WithChineseCount 设置中文验证码字符数量。
	WithChineseCount = driver.WithChineseCount
	// WithChineseSource 设置中文验证码字符源。
	WithChineseSource = driver.WithChineseSource
	// WithChineseChars 设置中文验证码字符列表。
	WithChineseChars = driver.WithChineseChars
	// WithChineseDotCount 设置中文验证码干扰点数量。
	WithChineseDotCount = driver.WithChineseDotCount
	// WithSlideConfig 设置滑动拼图验证码配置。
	WithSlideConfig = driver.WithSlideConfig
	// WithSlideMasterSize 设置滑动拼图主图尺寸。
	WithSlideMasterSize = driver.WithSlideMasterSize
	// WithSlideTileSize 设置滑动拼图滑块尺寸。
	WithSlideTileSize = driver.WithSlideTileSize
	// WithSlideTileRadius 设置滑动拼图滑块圆角半径。
	WithSlideTileRadius = driver.WithSlideTileRadius
	// WithSlideJigsawRadius 设置滑动拼图缺口圆角半径。
	WithSlideJigsawRadius = driver.WithSlideJigsawRadius
	// WithSlideShadow 设置滑动拼图阴影效果。
	WithSlideShadow = driver.WithSlideShadow
	// WithClickConfig 设置点击文字验证码配置。
	WithClickConfig = driver.WithClickConfig
	// WithClickMasterSize 设置点击验证码主图尺寸。
	WithClickMasterSize = driver.WithClickMasterSize
	// WithClickThumbSize 设置点击验证码缩略图尺寸。
	WithClickThumbSize = driver.WithClickThumbSize
	// WithClickCaptchaCount 设置点击验证码主图字符数量。
	WithClickCaptchaCount = driver.WithClickCaptchaCount
	// WithClickVerifyCount 设置点击验证码验证字符数量。
	WithClickVerifyCount = driver.WithClickVerifyCount
	// WithClickChars 设置点击验证码字符集。
	WithClickChars = driver.WithClickChars
	// WithClickLanguage 设置点击验证码语言。
	WithClickLanguage = driver.WithClickLanguage
	// WithClickShadow 设置点击验证码阴影效果。
	WithClickShadow = driver.WithClickShadow
	// WithRotateConfig 设置旋转验证码配置。
	WithRotateConfig = driver.WithRotateConfig
	// WithRotateMasterSize 设置旋转验证码主图尺寸。
	WithRotateMasterSize = driver.WithRotateMasterSize
	// WithRotateThumbSize 设置旋转验证码缩略图尺寸。
	WithRotateThumbSize = driver.WithRotateThumbSize
)
