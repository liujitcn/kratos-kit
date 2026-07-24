package driver

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/liujitcn/go-utils/id"
	"github.com/wenlng/go-captcha/v2/rotate"
)

const rotateVerifyPadding = 5

// rotateDriver 负责旋转图片验证码生成和角度校验。
type rotateDriver struct {
	config  *RotateConfig  // 旋转验证码配置
	captcha rotate.Captcha // go-captcha 旋转验证码实例
}

// newRotateDriver 创建旋转验证码驱动。
func newRotateDriver(config *RotateConfig) Driver {
	return &rotateDriver{config: config}
}

// Generate 生成旋转验证码。
func (d *rotateDriver) Generate() (*Challenge, error) {
	d.init()

	captData, err := d.captcha.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate rotate captcha: %w", err)
	}

	angleData := captData.GetData()
	if angleData == nil {
		return nil, fmt.Errorf("rotate captcha data is nil")
	}

	var masterBase64 string
	masterBase64, err = captData.GetMasterImage().ToBase64()
	if err != nil {
		return nil, fmt.Errorf("failed to convert master image to base64: %w", err)
	}

	var thumbBase64 string
	thumbBase64, err = captData.GetThumbImage().ToBase64()
	if err != nil {
		return nil, fmt.Errorf("failed to convert thumb image to base64: %w", err)
	}

	captchaID := id.NewGUIDv4NoHyphen()
	imageSize := d.captcha.GetOptions().GetImageSize()
	rotateData := &RotateCaptchaData{
		Type:      string(DriverRotate),
		Image:     masterBase64,
		Thumb:     thumbBase64,
		Width:     imageSize,
		Height:    imageSize,
		ThumbSize: angleData.Width,
	}

	var jsonData []byte
	jsonData, err = json.Marshal(rotateData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal rotate data: %w", err)
	}

	return &Challenge{
		ID:      captchaID,
		Payload: string(jsonData),
		// 正确角度只写入缓存，不放入返回给前端的 JSON。
		Answer: strconv.Itoa(angleData.Angle),
	}, nil
}

// Verify 验证旋转验证码答案，角度允许少量误差。
func (d *rotateDriver) Verify(expected, actual string) bool {
	expectedAngle, err := strconv.Atoi(expected)
	if err != nil {
		return false
	}

	var actualAngle int
	actualAngle, err = strconv.Atoi(actual)
	if err != nil {
		return false
	}

	// GoCaptcha 旋转验证码的校验语义是用户旋转角度加上初始随机角度接近 360 度。
	return rotate.Validate(actualAngle, expectedAngle, rotateVerifyPadding)
}

// init 初始化旋转验证码实例。
func (d *rotateDriver) init() {
	// go-captcha 实例可复用，避免每次生成都重复构造资源。
	if d.captcha != nil {
		return
	}
	rotateCfg := d.config
	// 未提供旋转配置时使用默认图片尺寸。
	if rotateCfg == nil {
		rotateCfg = DefaultRotateConfig()
	}

	masterSize := rotateCfg.MasterWidth
	// GoCaptcha 旋转验证码使用正方形主图，宽度无效时兼容高度配置。
	if masterSize <= 0 {
		masterSize = rotateCfg.MasterHeight
	}
	if masterSize <= 0 {
		masterSize = DefaultRotateConfig().MasterWidth
	}

	thumbSize := rotateCfg.ThumbWidth
	// 缩略图同样是正方形，宽度无效时兼容高度配置。
	if thumbSize <= 0 {
		thumbSize = rotateCfg.ThumbHeight
	}
	if thumbSize <= 0 {
		thumbSize = DefaultRotateConfig().ThumbWidth
	}

	builder := rotate.NewBuilder(
		rotate.WithImageSquareSize(masterSize),
		rotate.WithRangeThumbImageSquareSize([]int{thumbSize}),
	)
	builder.SetResources(rotate.WithImages(defaultBackgrounds(masterSize, masterSize)))
	d.captcha = builder.Make()
}
