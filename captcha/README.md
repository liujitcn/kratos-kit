# Captcha 验证码模块

`captcha` 提供图形验证码和行为验证码生成能力，存储层统一依赖 `github.com/liujitcn/kratos-kit/cache.Cache`。调用方只需要生成和验证两步：生成时模块自动保存答案，验证成功后模块自动删除缓存。

## 特性

- 支持数字、字符串、中文、算术验证码。
- 支持滑动拼图、点击文字、旋转图片验证码。
- 对外只返回验证码 ID 和图片数据，不返回答案。
- 图片数据只通过 base64 或 JSON(base64) 返回，不写本地文件。
- 答案只存入 `cache.Cache`，可接入内存缓存、Redis 缓存或其他缓存实现。
- 普通图形验证码基于 `github.com/mojocn/base64Captcha` 内嵌字体和绘制能力。
- 行为验证码默认使用 `github.com/wenlng/go-captcha-assets` 内嵌背景、字体、缩略图和拼图贴图资源，资源加载失败时自动降级到本地生成资源。

## 支持的验证码类型

| 类型 | 驱动常量 | 生成方式 | `Challenge.Payload` | 校验输入 |
| --- | --- | --- | --- | --- |
| 数字验证码 | `DriverDigit` | `base64Captcha.DriverDigit` | base64 图片 | 图片中的数字 |
| 字符串验证码 | `DriverString` | `base64Captcha.DriverString` | base64 图片 | 图片中的字符串 |
| 中文验证码 | `DriverChinese` | `base64Captcha.DriverChinese` | base64 图片 | 图片中的中文文字 |
| 算术验证码 | `DriverMath` | `base64Captcha.DriverMath` | base64 图片 | 算术表达式结果 |
| 滑动拼图验证码 | `DriverSlide` | GoCaptcha 拼图资源 | JSON(base64) | 滑块最终横坐标 |
| 点击文字验证码 | `DriverClick` | GoCaptcha 点选资源 | JSON(base64) | 待点击文字坐标数组 |
| 旋转图片验证码 | `DriverRotate` | GoCaptcha 旋转资源 | JSON(base64) | 用户旋转角度 |

普通图形验证码的 `Payload` 直接返回 base64 图片；行为验证码的 `Payload` 返回 JSON 字符串，JSON 中只包含前端展示所需的 base64 图片和辅助位置信息，正确答案只写入服务端缓存。

中文验证码和点击文字验证码默认使用模块内置的常用汉字题库，优先选择来自高频词语、结构简单且容易辨认的汉字。可通过 `WithChineseChars`、`WithChineseSource` 或 `WithClickChars` 覆盖默认题库。

## 目录结构

| 目录或文件 | 说明 |
| --- | --- |
| `captcha.go` | 对外服务门面，负责生成、缓存答案和验证 |
| `types.go` | 根包类型、配置和选项别名，保持 `captcha.With*` 等入口 |
| `store.go` | `cache.Cache` 存储适配 |
| `driver` | 数字、字符串、中文、算术、滑动、点击和旋转验证码驱动实现 |

## 快速开始

```go
package main

import (
	"context"
	"time"

	"github.com/liujitcn/kratos-kit/cache"
	"github.com/liujitcn/kratos-kit/captcha"
)

func main() {
	store, cleanup, err := cache.NewCache(nil)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	cap := captcha.NewCaptcha(store,
		captcha.WithDriverType(captcha.DriverString),
		captcha.WithExpire(5*time.Minute),
		captcha.WithKeyPrefix("admin:captcha"),
		captcha.WithStringCount(6),
		captcha.WithStringSource("ABCDEFGHJKLMNPQRSTUVWXYZ23456789"),
	)

	ctx := context.Background()
	challenge, err := cap.Generate(ctx)
	if err != nil {
		panic(err)
	}

	matched, err := cap.Verify(ctx, challenge.ID, "USER_INPUT")
	if err != nil {
		panic(err)
	}

	_, _ = challenge.Payload, matched
}
```

## 行为验证码

滑动拼图、点击文字和旋转图片验证码的 `Payload` 是 JSON 字符串，JSON 内只包含验证码类型、base64 图片和前端展示尺寸/位置信息，正确坐标或角度只保存在服务端缓存中。

行为验证码的默认素材参考 GoCaptcha 官方资源包，调用方无需准备本地图片或字体文件。若只使用数字、字符串、中文或算术验证码，则仍由 `base64Captcha` 生成普通图形验证码。

点击文字验证码默认展示 6 个主图文字，并要求用户按顺序点击缩略图中的 4 个目标文字；默认字符池使用常用且易辨认的中文字符，缩略图使用白底、多彩文字和适量干扰线。

## 中文验证码

中文验证码基于 `base64Captcha.DriverChinese` 生成，默认使用模块内置的常用汉字题库。调用方可直接使用默认题库：

```go
cap := captcha.NewCaptcha(store,
	captcha.WithDriverType(captcha.DriverChinese),
	captcha.WithChineseCount(4),
)
```

也可以显式指定中文字符列表：

```go
cap := captcha.NewCaptcha(store,
	captcha.WithDriverType(captcha.DriverChinese),
	captcha.WithChineseChars([]string{"中", "国", "人", "民", "你", "我", "他", "她"}),
)
```

滑动拼图示例：

```go
cap := captcha.NewCaptcha(store,
	captcha.WithDriverType(captcha.DriverSlide),
	captcha.WithSlideMasterSize(300, 220),
	captcha.WithSlideTileSize(60, 60),
)

challenge, err := cap.Generate(context.Background())
if err != nil {
	panic(err)
}

var data captcha.SlideCaptchaData
if err = json.Unmarshal([]byte(challenge.Payload), &data); err != nil {
	panic(err)
}
```

`SlideCaptchaData` 包含：

| 字段 | 说明 |
| --- | --- |
| `type` | 验证码类型，固定为 `slide` |
| `image` | 带缺口的主图 base64 |
| `thumb` | 滑块图 base64 |
| `width` | 主图展示宽度 |
| `height` | 主图展示高度 |
| `thumbX` | 滑块初始展示横坐标 |
| `thumbY` | 滑块初始展示纵坐标 |
| `thumbWidth` | 滑块展示宽度 |
| `thumbHeight` | 滑块展示高度 |

`ClickCaptchaData` 包含：

| 字段 | 说明 |
| --- | --- |
| `type` | 验证码类型，固定为 `click` |
| `image` | 主图 base64 |
| `thumb` | 缩略图 base64 |
| `width` | 主图展示宽度 |
| `height` | 主图展示高度 |
| `thumbWidth` | 缩略图展示宽度 |
| `thumbHeight` | 缩略图展示高度 |

`RotateCaptchaData` 包含：

| 字段 | 说明 |
| --- | --- |
| `type` | 验证码类型，固定为 `rotate` |
| `image` | 需要旋转的图片 base64 |
| `thumb` | 目标方向提示图 base64 |
| `width` | 主图展示宽度 |
| `height` | 主图展示高度 |
| `thumbSize` | 可旋转内圈展示尺寸 |

## 主要 API

| 方法 | 说明 |
| --- | --- |
| `NewCaptcha(cache.Cache, ...Option)` | 使用选项创建验证码实例 |
| `NewCaptchaWithConfig(cache.Cache, *Config)` | 使用完整配置创建验证码实例 |
| `Generate(ctx)` | 生成验证码并自动保存答案，返回 `Challenge{ID, Payload}` |
| `Verify(ctx, id, input)` | 校验用户输入，成功后自动删除缓存 |

## 配置说明

| 配置 | 说明 |
| --- | --- |
| `WithDriverType` | 设置验证码类型 |
| `WithExpire` | 设置答案缓存过期时间 |
| `WithKeyPrefix` | 设置缓存 key 前缀 |
| `WithDigit*` | 设置数字验证码参数 |
| `WithString*` | 设置字符串验证码参数 |
| `WithChinese*` | 设置中文验证码参数 |
| `WithMath*` | 设置算术验证码参数 |
| `WithSlide*` | 设置滑动拼图验证码参数 |
| `WithClick*` | 设置点击文字验证码参数 |
| `WithRotate*` | 设置旋转验证码参数 |

## 注意事项

1. 业务接口只应把 `Challenge.ID` 和 `Challenge.Payload` 返回给前端。
2. `Verify` 成功后会删除缓存，可降低重放风险。
3. 普通图形验证码的 `Payload` 是 base64 图片；行为验证码的 `Payload` 是 JSON(base64)。
4. 行为验证码默认资源来自 Go 依赖内嵌文件，不会在运行时访问网络或读取外部目录。
5. 高并发或多实例场景建议使用 Redis 类型的 `cache.Cache` 实现。
