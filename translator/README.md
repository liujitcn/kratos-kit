# Translator

`translator` 提供基于配置的统一机器翻译接口，底层复用
`github.com/liujitcn/go-utils/translator`，内置 Google、百度、阿里云和火山引擎
Provider，也支持应用注册自定义 Provider。

## 配置

```yaml
translator:
  enabled: true
  type: google
  timeout: 8s
  google:
    version: v1
```

配置 `type` 选择 Provider。Google、百度、阿里云和火山引擎分别使用
`google`、`baidu`、`alibaba`、`volc` 配置段；Google V1 可省略 `google` 配置段，
其他 Provider 缺少对应配置段时会在启动阶段返回配置错误。`timeout` 同时用于
翻译请求的 HTTP Client 和业务调用截止时间。

## 自定义 Provider

业务项目可以注册任意实现 `go-utils/translator.Translator` 的 Provider：

```go
translator.MustRegisterProvider("custom", func(
	_ *configv1.Translator,
	httpClient *http.Client,
) (translator.Translator, error) {
	return custom.New(httpClient), nil
})
```

注册应在创建翻译器前完成；配置中的 `type: custom` 即可选择该实现。
