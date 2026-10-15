# normalize-go-imports

`normalize-go-imports` 扫描 Go module，把包路径末级目录与 import 别名统一，减少同一仓库中别名风格不一致的问题。

## 使用

```bash
normalize-go-imports -root /path/to/project
normalize-go-imports -root /path/to/project -write
normalize-go-imports -root /path/to/project -modules backend,api -write
normalize-go-imports -root /path/to/project -files internal/data/data.go
```

默认只输出变更预览；指定 `-write` 才会写回文件。`-modules` 接收相对根目录的 module 目录，未指定时自动发现所有 `go.mod`。`-files` 用于限制待处理文件。

命令只修改 import 别名，不改包路径、代码逻辑或未识别的语法。
