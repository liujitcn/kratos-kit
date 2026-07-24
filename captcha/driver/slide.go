package driver

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/liujitcn/go-utils/id"
	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/slide"
)

// slideDriver 负责滑动拼图验证码生成和横向偏移校验。
type slideDriver struct {
	config  *SlideConfig  // 滑动拼图配置
	captcha slide.Captcha // go-captcha 滑动验证码实例
}

// newSlideDriver 创建滑动拼图验证码驱动。
func newSlideDriver(config *SlideConfig) Driver {
	return &slideDriver{config: config}
}

// Generate 生成滑动拼图验证码。
func (d *slideDriver) Generate() (*Challenge, error) {
	d.init()

	captData, err := d.captcha.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate slide captcha: %w", err)
	}

	blockData := captData.GetData()
	if blockData == nil {
		return nil, fmt.Errorf("slide captcha data is nil")
	}

	var masterBase64 string
	masterBase64, err = captData.GetMasterImage().ToBase64()
	if err != nil {
		return nil, fmt.Errorf("failed to convert master image to base64: %w", err)
	}

	var tileBase64 string
	tileBase64, err = captData.GetTileImage().ToBase64()
	if err != nil {
		return nil, fmt.Errorf("failed to convert tile image to base64: %w", err)
	}

	captchaID := id.NewGUIDv4NoHyphen()
	slideCfg := d.config
	if slideCfg == nil {
		slideCfg = DefaultSlideConfig()
	}
	slideData := &SlideCaptchaData{
		Type:        string(DriverSlide),
		Image:       masterBase64,
		Thumb:       tileBase64,
		Width:       slideCfg.MasterWidth,
		Height:      slideCfg.MasterHeight,
		ThumbX:      blockData.DX,
		ThumbY:      blockData.DY,
		ThumbWidth:  blockData.Width,
		ThumbHeight: blockData.Height,
	}

	var jsonData []byte
	jsonData, err = json.Marshal(slideData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal slide data: %w", err)
	}

	return &Challenge{
		ID:      captchaID,
		Payload: string(jsonData),
		// 正确 X 坐标只写入缓存，不放入返回给前端的 JSON。
		Answer: strconv.Itoa(blockData.X),
	}, nil
}

// Verify 验证滑动验证码答案，允许少量像素误差。
func (d *slideDriver) Verify(expected, actual string) bool {
	expectedX, err := strconv.Atoi(expected)
	if err != nil {
		return false
	}

	var actualX int
	actualX, err = strconv.Atoi(actual)
	if err != nil {
		return false
	}

	diff := expectedX - actualX
	// 用户拖动可能存在少量像素误差，校验时转为绝对差值。
	if diff < 0 {
		diff = -diff
	}
	return diff <= 5
}

// init 初始化滑动验证码实例。
func (d *slideDriver) init() {
	// go-captcha 实例可复用，避免每次生成都重复构造资源。
	if d.captcha != nil {
		return
	}

	slideCfg := d.config
	// 未提供滑动配置时使用默认尺寸和资源参数。
	if slideCfg == nil {
		slideCfg = DefaultSlideConfig()
	}

	builder := slide.NewBuilder(
		slide.WithImageSize(option.Size{Width: slideCfg.MasterWidth, Height: slideCfg.MasterHeight}),
		slide.WithRangeGraphSize(option.RangeVal{Min: slideCfg.TileWidth, Max: slideCfg.TileWidth}),
	)
	builder.SetResources(
		slide.WithGraphImages(defaultSlideGraphs()),
		slide.WithBackgrounds(defaultBackgrounds(slideCfg.MasterWidth, slideCfg.MasterHeight)),
	)
	d.captcha = builder.Make()
}
