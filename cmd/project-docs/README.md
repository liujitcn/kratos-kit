# project-docs

`project-docs` 是构建期文档收集命令，从项目根目录开始扫描相对路径
不超过三段的文件，只处理以下 Markdown：

- 文件名精确为 `README.md` 的文件。
- 任意父目录名为 `docs` 的 Markdown 文件。

同一路径可以通过文件名语言后缀提供翻译，例如 `README.en-US.md`、
`docs/guide.zh-TW.md`。无后缀文件是默认正文，语言版本会聚合到同一个文档节点，
不会在目录中重复显示。

例如会收集 `README.md`、`backend/core/README.md`、
`docs/guide/install.md` 和 `backend/docs/api.md`，不会收集路径超过三段的
`backend/internal/agent/README.md` 或 `docs/guide/install/linux.md`。

JSON 按文件目录保存为树形结构，项目身份不写入构建产物：

```json
{
  "documents": [
    {
      "path": "README.md",
      "content": "# kratos-admin",
      "locale": {
        "en-US": "# kratos-admin"
      },
      "updated_at": "2026-07-31T08:00:00Z"
    }
  ],
  "directories": [
    {
      "name": "docs",
      "path": "docs",
      "documents": [],
      "directories": []
    }
  ]
}
```

文档节点只记录项目内相对路径、源 Markdown 文件的 RFC3339 更新时间和正文。
服务加载生成物后，使用 `AppInfo.Project` 和 `AppInfo.Name` 生成项目身份与稳定
文档 ID。

生成时只扫描和聚合项目中显式存在的语言 Markdown 文件，不执行网络翻译。
上一次生成结果中，源文未变化的 `locale` 内容会被保留；源文变化后由独立的
i18n 脚本重新补充。这样 Go 命令只负责收集文档，适合通过 `go install` 分发。

## 安装

```bash
go install github.com/liujitcn/kratos-kit/cmd/project-docs@latest
```

## 使用

```bash
project-docs
```

普通项目输出到 `internal/projectdocs`；当前目录包含 `backend` 时，默认输出到
`backend/internal/docs`。输出目录下会生成 `assets/docs.json` 和
`docs.go`，其中导出 `DocsData` 作为嵌入 JSON。

可以通过 `--output` 或 `-o` 指定生成目录。相对路径以项目根目录为基准：

```bash
project-docs --output ./backend/internal/docs
project-docs -o ./build/projectdocs
```

从其他工作目录执行时，可以用 `--root` 指定待扫描的项目根目录：

```bash
project-docs --root /path/to/project --output /path/to/project/backend/internal/docs
```

多语言补充脚本位于 `cmd/i18n/project_docs.py`，读取已经生成的
`assets/docs.json`，只为缺少的语言字段补充翻译，不重新扫描 Markdown，也不生成
`docs.go`。它支持 Google V1、OpenCC、Markdown 代码和占位符保护；翻译端点可通过
`I18N_TRANSLATE_ENDPOINT` 覆盖，离线模式使用 `--offline` 或 `I18N_OFFLINE=1`。

```bash
project-docs --output ./backend/internal/docs
python3 ./cmd/i18n/project_docs.py \
  --root . \
  --output ./backend/internal/docs \
  --source-locale zh-CN \
  --locales en-US,zh-TW,ja-JP
```

多模块文档由各模块分别生成，并在运行时通过 Contributor 聚合。
