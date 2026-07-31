# project-docs

`project-docs` 是零参数的构建期文档收集命令，从项目根目录开始扫描相对路径
不超过三段的文件，只处理以下 Markdown：

- 文件名精确为 `README.md` 的文件。
- 任意父目录名为 `docs` 的 Markdown 文件。

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

## 安装

```bash
go install github.com/liujitcn/kratos-kit/cmd/project-docs@latest
```

## 使用

```bash
project-docs
```

普通项目输出到 `internal/projectdocs`。当前目录包含 `backend` 时，输出到
`backend/internal/projectdocs`，用于 `kratos-admin` 这类前后端一体仓库。命令
不接受任何参数；多模块文档由各模块分别生成，并在运行时通过 Contributor 聚合。
