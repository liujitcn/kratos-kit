# project-docs

`project-docs` 是构建期文档收集命令，从项目根目录开始扫描相对路径
不超过三段的文件，只处理以下 Markdown：

- 文件名精确为 `README.md` 的文件。
- 任意父目录名为 `docs` 的 Markdown 文件。

命令只把无语言后缀的 Markdown 收集到默认目录；`README.en-US.md`、
`docs/guide.zh-TW.md` 等带语言后缀文件不参与默认目录。语言目录由下游翻译工具
根据 `docs.json` 生成 `docs.<locale>.json`。

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

生成命令不执行网络翻译，也不写入 `locale` 字段。默认目录与语言目录使用相同的
稳定文档路径，运行时可以据此按请求语言选择整份目录树和文档正文。

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
`docs.go`。生成的 Go 包导出默认目录 `DocsData`，并通过 `DocsFS` 嵌入构建时已经
存在的全部 `assets/docs*.json`。

可以通过 `--output` 或 `-o` 指定生成目录。相对路径以项目根目录为基准：

```bash
project-docs --output ./backend/internal/docs
project-docs -o ./build/projectdocs
```

从其他工作目录执行时，可以用 `--root` 指定待扫描的项目根目录：

```bash
project-docs --root /path/to/project --output /path/to/project/backend/internal/docs
```

多模块文档由各模块分别生成，并在运行时通过 Contributor 聚合。
