package driver

import (
	"encoding/json"
	"fmt"

	"github.com/liujitcn/go-utils/id"
	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/click"
)

const clickVerifyPadding = 8

// clickDriver 负责点击文字验证码生成和坐标序列校验。
type clickDriver struct {
	config  *ClickConfig  // 点击文字验证码配置
	captcha click.Captcha // go-captcha 点击验证码实例
}

// clickAnswerDot 表示服务端缓存中的正确点击坐标。
type clickAnswerDot struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
	Size   int `json:"size"`
}

// clickInputDot 表示前端提交的点击坐标。
type clickInputDot struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// newClickDriver 创建点击文字验证码驱动。
func newClickDriver(config *ClickConfig) Driver {
	return &clickDriver{config: config}
}

// Generate 生成点击文字验证码。
func (d *clickDriver) Generate() (*Challenge, error) {
	d.init()

	captData, err := d.captcha.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate click captcha: %w", err)
	}

	dotData := captData.GetData()
	if dotData == nil {
		return nil, fmt.Errorf("click captcha data is nil")
	}

	var masterBase64 string
	masterBase64, err = captData.GetMasterImage().ToBase64()
	if err != nil {
		return nil, fmt.Errorf("failed to get master image: %w", err)
	}

	var thumbBase64 string
	thumbBase64, err = captData.GetThumbImage().ToBase64()
	if err != nil {
		return nil, fmt.Errorf("failed to get thumb image: %w", err)
	}

	answerDots := make([]clickAnswerDot, 0, len(dotData))
	// 正确点击坐标只作为答案写入缓存，Payload 里只返回图片。
	for i := 0; i < len(dotData); i++ {
		dot := dotData[i]
		if dot == nil {
			return nil, fmt.Errorf("click captcha dot data is nil")
		}
		answerDots = append(answerDots, clickAnswerDot{
			X:      dot.X,
			Y:      dot.Y,
			Width:  dot.Width,
			Height: dot.Height,
			Size:   dot.Size,
		})
	}

	captchaID := id.NewGUIDv4NoHyphen()
	clickCfg := d.config
	if clickCfg == nil {
		clickCfg = DefaultClickConfig()
	}
	clickData := &ClickCaptchaData{
		Type:        string(DriverClick),
		Image:       masterBase64,
		Thumb:       thumbBase64,
		Width:       clickCfg.MasterWidth,
		Height:      clickCfg.MasterHeight,
		ThumbWidth:  clickCfg.ThumbWidth,
		ThumbHeight: clickCfg.ThumbHeight,
	}

	var jsonData []byte
	jsonData, err = json.Marshal(clickData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal click data: %w", err)
	}

	var answerBytes []byte
	answerBytes, err = json.Marshal(answerDots)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal click answer: %w", err)
	}

	return &Challenge{
		ID:      captchaID,
		Payload: string(jsonData),
		Answer:  string(answerBytes),
	}, nil
}

// Verify 验证点击文字验证码答案，坐标允许少量误差。
func (d *clickDriver) Verify(expected, actual string) bool {
	var expectedDots []clickAnswerDot
	err := json.Unmarshal([]byte(expected), &expectedDots)
	if err != nil {
		return false
	}

	var actualDots []clickInputDot
	err = json.Unmarshal([]byte(actual), &actualDots)
	if err != nil {
		return false
	}

	if len(expectedDots) != len(actualDots) {
		return false
	}

	for i, expectedDot := range expectedDots {
		actualDot := actualDots[i]
		width := expectedDot.Width
		height := expectedDot.Height
		// 兼容历史缓存或极端资源数据未写入宽高的情况，使用字符尺寸兜底。
		if width <= 0 {
			width = expectedDot.Size
		}
		if height <= 0 {
			height = expectedDot.Size
		}
		// 用户点击字符区域即可通过，不要求刚好点到字符左上角或中心点。
		if !click.Validate(int(actualDot.X), int(actualDot.Y), expectedDot.X, expectedDot.Y, width, height, clickVerifyPadding) {
			return false
		}
	}
	return true
}

// init 初始化点击文字验证码实例。
func (d *clickDriver) init() {
	// go-captcha 实例可复用，避免每次生成都重复构造资源。
	if d.captcha != nil {
		return
	}

	clickCfg := d.config
	// 未提供点击配置时使用默认字符集、尺寸和阴影参数。
	if clickCfg == nil {
		clickCfg = DefaultClickConfig()
	}

	builder := click.NewBuilder(
		click.WithImageSize(option.Size{Width: clickCfg.MasterWidth, Height: clickCfg.MasterHeight}),
		click.WithRangeLen(option.RangeVal{Min: clickCfg.CaptchaCount, Max: clickCfg.CaptchaCount}),
		click.WithRangeVerifyLen(option.RangeVal{Min: clickCfg.VerifyCount, Max: clickCfg.VerifyCount}),
		click.WithRangeThumbImageSize(option.Size{Width: clickCfg.ThumbWidth, Height: clickCfg.ThumbHeight}),
		click.WithRangeThumbSize(option.RangeVal{Min: 28, Max: 34}),
		click.WithRangeThumbColors([]string{"#6d28d9", "#7c2d12", "#166534", "#be123c", "#1d4ed8", "#a16207"}),
		click.WithRangeThumbBgColors([]string{"#7e22ce", "#92400e", "#15803d", "#b91c1c", "#0f766e", "#c2410c"}),
		// 提示图保留多彩干扰线，但不再扭曲文字本身，避免四个目标字过难识别。
		click.WithRangeThumbBgDistort(option.DistortLevel4),
		click.WithRangeThumbBgCirclesNum(16),
		click.WithRangeThumbBgSlimLineNum(4),
		click.WithIsThumbNonDeformAbility(true),
		click.WithThumbDisturbAlpha(0.75),
		click.WithDisplayShadow(clickCfg.DisplayShadow),
		click.WithShadowColor(clickCfg.ShadowColor),
		click.WithShadowPoint(option.Point{X: clickCfg.ShadowOffsetX, Y: clickCfg.ShadowOffsetY}),
	)

	builder.SetResources(
		click.WithChars(defaultClickChars(clickCfg.Language, clickCfg.Chars)),
		click.WithFonts(defaultFonts()),
		click.WithBackgrounds(defaultBackgrounds(clickCfg.MasterWidth, clickCfg.MasterHeight)),
		click.WithThumbBackgrounds(defaultThumbBackgrounds(clickCfg.ThumbWidth, clickCfg.ThumbHeight)),
	)
	d.captcha = builder.Make()
}
