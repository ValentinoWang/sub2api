# 原生成本工作台

实现、启用方式、数据口径、验收证据和未完成边界统一见 [V04_NATIVE_LEDGER.md](V04_NATIVE_LEDGER.md)。

管理员入口：`/admin/cost-center`。前端在应用启动的首次导航前注册；后端在原有路由设置完成后注册，沿用现有管理员认证、审计、合规与限流。

三档固定为 `plus`、`pro5x`、`pro20x`。账号费用、实际可用容量与模型能力分别输入，不用宣传倍率替代实测。原生采购账和条件比较器分开；单档缺数据就显示缺数据，不因另一档已知而补造结果。

静态核查命令：

```bash
python tools/cost_research/check_native_cost_wiring.py
```

接线已体现在源码中。完整仓库编译、PostgreSQL真实集成、Vue浏览器验收和人工验收状态以当前机器记录为准；独立Go包与SQL替身通过不等于以上项目通过。
