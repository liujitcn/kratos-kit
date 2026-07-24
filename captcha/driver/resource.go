package driver

import (
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/golang/freetype/truetype"
	assetchars "github.com/wenlng/go-captcha-assets/bindata/chars"
	assetfont "github.com/wenlng/go-captcha-assets/resources/fonts/fzshengsksjw"
	assetimages "github.com/wenlng/go-captcha-assets/resources/imagesv2"
	assettiles "github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/slide"
	"golang.org/x/image/font/gofont/goregular"
)

// defaultCommonCharsSource 是中文图形验证码和点击验证码共用的常用汉字题库。
// 题库按高频词语筛选，优先保留结构简单、容易辨认的汉字，避免默认资源中的生僻字干扰识别。
const defaultCommonCharsSource = "的一是了不人我在有他这为大来以个中上们到说时要就出会可也你对生能而子那得于着下自之年过发后作里用道行所然家种事成方多经日月水火山天文地人心手口目耳头面身长小高明白好看听读写学问工力生活世界中国人民我们大家朋友工作时间天气开心快乐平安顺利成功希望欢迎感谢请再见开始结束可以需要应该因为所以如果现在已经没有什么怎么这里那里东西地方问题方法结果信息内容情况关系帮助支持服务系统数据用户功能设置管理网络安全发展建设学习文化社会学校老师学生儿童父母男女左右前后上下内外开关真假正反多少新旧早晚快慢冷热轻重远近进退买卖吃喝穿住走坐站跑跳笑哭爱喜欢想做给让帮拿打找放"

// defaultBackground 创建默认验证码背景图。
func defaultBackground(width, height int) image.Image {
	bg := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 使用坐标扰动生成渐变纹理，避免依赖本地图片文件。
			r := uint8(36 + (x*37+y*11)%90)
			g := uint8(70 + (x*13+y*29)%100)
			b := uint8(130 + (x*17+y*7)%90)
			bg.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return bg
}

// defaultThumbBackground 创建白色提示图背景，方便多彩文字和干扰线形成对比。
func defaultThumbBackground(width, height int) image.Image {
	bg := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			shade := uint8(252 - (x*5+y*3)%8)
			bg.SetRGBA(x, y, color.RGBA{R: shade, G: shade, B: shade, A: 255})
		}
	}
	return bg
}

// defaultBackgrounds 返回 GoCaptcha 官方内嵌主图资源，加载失败时回退到本地生成背景。
func defaultBackgrounds(width, height int) []image.Image {
	images, err := assetimages.GetImages()
	// 官方素材不可用时使用本地生成背景，避免外部资源问题阻断验证码生成。
	if err != nil || len(images) == 0 {
		return []image.Image{defaultBackground(width, height)}
	}
	return images
}

// defaultThumbBackgrounds 返回低干扰缩略图背景，避免点击验证码提示文字被噪声盖住。
func defaultThumbBackgrounds(width, height int) []image.Image {
	return []image.Image{defaultThumbBackground(width, height)}
}

// defaultSlideGraphs 创建默认滑动拼图图块资源。
func defaultSlideGraphs() []*slide.GraphImage {
	graphs, err := assettiles.GetTiles()
	// 官方拼图素材加载成功时转换为 go-captcha 需要的资源类型。
	if err == nil && len(graphs) > 0 {
		newGraphs := make([]*slide.GraphImage, 0, len(graphs))
		for _, graph := range graphs {
			if graph == nil {
				continue
			}
			newGraphs = append(newGraphs, &slide.GraphImage{
				OverlayImage: graph.OverlayImage,
				ShadowImage:  graph.ShadowImage,
				MaskImage:    graph.MaskImage,
			})
		}
		if len(newGraphs) > 0 {
			return newGraphs
		}
	}
	return generatedSlideGraphs()
}

// generatedSlideGraphs 创建本地兜底滑动拼图图块资源。
func generatedSlideGraphs() []*slide.GraphImage {
	return []*slide.GraphImage{
		{
			OverlayImage: defaultPuzzleImage(color.RGBA{R: 255, G: 255, B: 255, A: 90}),
			ShadowImage:  defaultPuzzleImage(color.RGBA{R: 0, G: 0, B: 0, A: 120}),
			MaskImage:    defaultPuzzleImage(color.RGBA{R: 255, G: 255, B: 255, A: 255}),
		},
	}
}

// defaultPuzzleImage 创建默认滑动拼图遮罩图。
func defaultPuzzleImage(fill color.RGBA) image.Image {
	const size = 80
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	center := float64(size) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := math.Abs(float64(x) - center)
			dy := math.Abs(float64(y) - center)
			// 拼图块由中间矩形加两个圆形凸起组成，兼顾识别度和生成成本。
			inRect := dx < 24 && dy < 24
			inTopCircle := math.Hypot(float64(x)-center, float64(y)-18) < 12
			inRightCircle := math.Hypot(float64(x)-62, float64(y)-center) < 12
			if inRect || inTopCircle || inRightCircle {
				img.SetRGBA(x, y, fill)
			}
		}
	}
	return img
}

// defaultFonts 返回默认验证码字体。
func defaultFonts() []*truetype.Font {
	font, err := assetfont.GetFont()
	// 优先使用 GoCaptcha 官方字体，让点击文字验证码具备更好的中文展示效果。
	if err == nil && font != nil {
		return []*truetype.Font{font}
	}

	font, err = truetype.Parse(goregular.TTF)
	// 字体解析失败时交给 go-captcha 使用默认处理，避免生成流程 panic。
	if err != nil {
		return nil
	}
	return []*truetype.Font{font}
}

// defaultClickChars 返回点击验证码字符资源。
func defaultClickChars(language, source string) []string {
	// 调用方显式传入字符集时保留原始字符，不做 trim 或大小写改写。
	if source != "" {
		return splitChars(source)
	}

	// 英文模式使用官方字母数字资源，其他模式默认使用内置常用中文字符。
	if language == "en" {
		chars := assetchars.GetAlphaChars()
		if len(chars) > 0 {
			return chars
		}
	}

	return splitChars(defaultCommonCharsSource)
}

// defaultChineseSource 返回中文图形验证码字符源。
func defaultChineseSource(source string) string {
	// 调用方显式传入字符源时保留原始内容，不做隐式清洗或改写。
	if source != "" {
		return source
	}

	return strings.Join(splitChars(defaultCommonCharsSource), ",")
}

// splitChars 按 rune 拆分字符源，兼容中文和英文字符。
func splitChars(source string) []string {
	chars := make([]string, 0, len([]rune(source)))
	for _, ch := range source {
		chars = append(chars, string(ch))
	}
	return chars
}
