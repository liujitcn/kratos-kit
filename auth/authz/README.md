# Kratos Authz

本模块定义统一鉴权契约、引擎工厂和 Kratos 中间件；具体实现按独立 Go
模块安装。

## 支持引擎

- `engine/casbin`：内置 ACL、RBAC、RESTful 及多租户模型
- `engine/opa`：本地 Rego 决策、批量过滤和策略数据热更新
- `engine/cerbos`：Cerbos PDP HTTP 决策；只提供 `Authorizer`，策略由外部 PDP 管理
- `engine/zanzibar`：面向 OpenFGA、Keto 等关系服务的
  `Checker`/`ProjectLister`/`PolicyWriter` 适配端口

`Engine` 只承诺 `Authorizer` 能力；Casbin、OPA 等可写实现会额外实现
`PolicyWriter`。Zanzibar 仅在显式配置写入器后通过 `State.PolicyWriter` 暴露该能力。
外部策略服务没有项目枚举能力时会返回明确错误，不会用空集合伪装成功。
