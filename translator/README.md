# Translator

`translator` 提供基于配置的统一机器翻译接口，底层复用
`github.com/liujitcn/go-utils/translator/*` 的厂商实现，内置 Google、百度、阿里云
和火山引擎 Provider。各 Provider 适配器分别位于
`translator/google`、`translator/baidu`、`translator/alibaba` 和 `translator/volc`
子目录。

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
