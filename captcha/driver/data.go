package driver

// SlideCaptchaData 滑动验证码数据结构。
type SlideCaptchaData struct {
	Type        string `json:"type"`        // 验证码类型
	Image       string `json:"image"`       // 主图 base64
	Thumb       string `json:"thumb"`       // 滑块图 base64
	Width       int    `json:"width"`       // 主图展示宽度
	Height      int    `json:"height"`      // 主图展示高度
	ThumbX      int    `json:"thumbX"`      // 滑块初始展示横坐标
	ThumbY      int    `json:"thumbY"`      // 滑块初始展示纵坐标
	ThumbWidth  int    `json:"thumbWidth"`  // 滑块展示宽度
	ThumbHeight int    `json:"thumbHeight"` // 滑块展示高度
}

// ClickCaptchaData 点击验证码数据结构。
type ClickCaptchaData struct {
	Type        string `json:"type"`        // 验证码类型
	Image       string `json:"image"`       // 主图 base64
	Thumb       string `json:"thumb"`       // 缩略图 base64
	Width       int    `json:"width"`       // 主图展示宽度
	Height      int    `json:"height"`      // 主图展示高度
	ThumbWidth  int    `json:"thumbWidth"`  // 缩略图展示宽度
	ThumbHeight int    `json:"thumbHeight"` // 缩略图展示高度
}

// RotateCaptchaData 旋转验证码数据结构。
type RotateCaptchaData struct {
	Type      string `json:"type"`      // 验证码类型
	Image     string `json:"image"`     // 主图 base64（需要旋转的图片）
	Thumb     string `json:"thumb"`     // 缩略图 base64（提示目标方向）
	Width     int    `json:"width"`     // 主图展示宽度
	Height    int    `json:"height"`    // 主图展示高度
	ThumbSize int    `json:"thumbSize"` // 内圈展示尺寸
}
