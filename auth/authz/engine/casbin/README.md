# auth/authz/engine/casbin

该模块把 Casbin 接入 `auth/authz/engine.Authorizer`，提供租户、主体、资源、动作和项目维度的权限判断，以及按权限过滤项目和资源动作对。

## 创建引擎

```go
state, err := casbin.NewEngine(ctx,
	casbin.WithDefaultModel("restfull_with_role"),
	casbin.WithProjects(engine.Projects{"project-a", "project-b"}),
)
if err != nil {
	return err
}
```

支持的内置模型名称：`acl`、`rbac`、`rbac_with_domains`、`abac`、`restfull` 和 `restfull_with_role`。也可以使用 `WithModel`、`WithStringModel` 或 `WithFileModel` 提供自定义模型。

未指定项目时使用通配项目值；`WithWildcardItem` 可以覆盖该值。项目授权列表使用 `WithAuthorizedProjectsMatcher` 指定匹配表达式。

## 策略

`WithPolices`（兼容现有拼写）或 `WithPolicyAdapter` 设置策略快照。`PolicyRule` 支持 `p_type` 和最多六个策略值，适配器在创建 Enforcer 时加载快照。

引擎实现 `PolicyWriter`，调用 `SetPolicies` 会替换下一次决策使用的策略快照。适配器不支持 Casbin 的增量写回方法，`SavePolicy`、`AddPolicy`、`RemovePolicy` 和过滤删除会返回未实现错误。

## 决策接口

- `IsAuthorized`：判断单个主体对资源、动作和项目是否有权限；
- `ProjectsAuthorized`：从项目集合中过滤允许项；
- `FilterAuthorizedPairs`：过滤资源动作对；
- `FilterAuthorizedProjects`：从初始化的项目快照中返回允许项。

引擎通过 `auth/authz/engine` 工厂以 `casbin` 名称注册，创建时匿名导入该包即可启用。
