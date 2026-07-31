# project-docs

`project-docs` 在构建期收集一个或多个项目根目录的 `README.md`、
`docs/**/*.md`，以及 `frontend/admin/README.md` 和
`frontend/app/README.md`，生成可嵌入服务二进制的稳定 JSON 目录和 Go
嵌入文件。其他层级的 `README.md` 和 `docs` 目录不会被收集。

JSON 目录按项目和文件目录保存为树形结构。每个项目节点分别包含根目录文档和
`directories`，目录节点通过同名字段递归保存子目录：

```json
{
  "projects": [
    {
      "key": "admin",
      "name": "系统管理",
      "documents": [],
      "directories": [
        {
          "name": "docs",
          "path": "docs",
          "documents": [],
          "directories": []
        }
      ]
    }
  ]
}
```

## 安装

```bash
go install github.com/liujitcn/kratos-kit/cmd/project-docs@latest
```

## 使用

每个 `--source` 使用 `OpenAPI-key:项目名称=项目根目录` 格式，并可重复传入：

```bash
project-docs \
  --source 'admin:系统管理=..' \
  --source 'shop:商城=../../shop'
```

默认输出到当前执行目录下的 `internal/projectdocs/assets/catalog.json`，并自动生成
`internal/projectdocs/catalog_gen.go`。特殊场景可通过 `--output` 覆盖 JSON
输出路径；非标准 JSON 路径默认不生成 Go 文件：

```bash
project-docs \
  --source 'shop:商城=.' \
  --output ./build/project-documents.json
```

自定义目录仍需生成 Go 嵌入文件时，同时使用 `--go-output`。JSON 文件必须位于
Go 文件所在目录或其子目录，满足 `go:embed` 的路径限制：

```bash
project-docs \
  --source 'shop:商城=.' \
  --output ./internal/projectdocs/assets/shop.json \
  --go-output ./internal/projectdocs/catalog_gen.go
```

文档 ID 由 OpenAPI 项目标识和项目内相对路径生成，修改文档内容不会改变 ID。
