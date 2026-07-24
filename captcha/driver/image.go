package driver

import (
	"fmt"
	"strings"

	"github.com/mojocn/base64Captcha"
)

const chineseCaptchaFont = "wqy-microhei.ttc"

// imageDriver 负责数字、字符串、中文和算术等普通图形验证码。
type imageDriver struct {
	config *Config // 普通图形验证码配置
}

// newImageDriver 创建普通图形验证码驱动。
func newImageDriver(config *Config) Driver {
	return &imageDriver{config: config}
}

// Generate 生成数字、字符串、中文或算术图形验证码。
func (d *imageDriver) Generate() (*Challenge, error) {
	base64Driver := d.base64Driver()
	id, question, answer := base64Driver.GenerateIdQuestionAnswer()
	item, err := base64Driver.DrawCaptcha(question)
	if err != nil {
		return nil, fmt.Errorf("failed to draw captcha: %w", err)
	}
	return &Challenge{
		ID:      id,
		Payload: item.EncodeB64string(),
		Answer:  answer,
	}, nil
}

// Verify 校验普通图形验证码答案。
func (d *imageDriver) Verify(expected, actual string) bool {
	// 普通图形验证码沿用 base64Captcha 的校验习惯，避免大小写和前后空格导致用户看图正确却失败。
	return strings.EqualFold(strings.TrimSpace(expected), strings.TrimSpace(actual))
}

// base64Driver 根据配置创建 base64Captcha 驱动。
func (d *imageDriver) base64Driver() base64Captcha.Driver {
	config := d.config
	// 允许驱动单独测试或独立构造时不传配置。
	if config == nil {
		config = DefaultConfig()
	}

	// 不同图形验证码类型使用不同的 base64Captcha driver。
	switch config.DriverType {
	case DriverString:
		stringCfg := config.StringConfig
		if stringCfg == nil {
			stringCfg = DefaultStringConfig()
		}
		return base64Captcha.NewDriverString(
			stringCfg.Height,
			stringCfg.Width,
			stringCfg.DotCount,
			0,
			stringCfg.CaptchaCount,
			stringCfg.Source,
			nil,
			nil,
			nil,
		)
	case DriverMath:
		mathCfg := config.MathConfig
		if mathCfg == nil {
			mathCfg = DefaultMathConfig()
		}
		return base64Captcha.NewDriverMath(
			mathCfg.Height,
			mathCfg.Width,
			mathCfg.DotCount,
			0,
			nil,
			nil,
			nil,
		)
	case DriverChinese:
		chineseCfg := config.ChineseConfig
		if chineseCfg == nil {
			chineseCfg = DefaultChineseConfig()
		}
		// 中文验证码固定使用中文字体，避免默认随机字体缺少中文字形时显示方框。
		return base64Captcha.NewDriverChinese(
			chineseCfg.Height,
			chineseCfg.Width,
			chineseCfg.DotCount,
			0,
			chineseCfg.CaptchaCount,
			defaultChineseSource(chineseCfg.Source),
			nil,
			nil,
			[]string{chineseCaptchaFont},
		)
	default:
		digitCfg := config.DigitConfig
		if digitCfg == nil {
			digitCfg = DefaultDigitConfig()
		}
		return base64Captcha.NewDriverDigit(
			digitCfg.Height,
			digitCfg.Width,
			digitCfg.CaptchaCount,
			digitCfg.MaxSkew,
			digitCfg.DotCount,
		)
	}
}
