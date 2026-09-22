# 成本研究与本地原生接入

`costlab.py` 与 `capture_radar.py` 提供参考资料研究原型；`start_local_native.py` 启动原生 Go 成本开发服务并初始化独立 PostgreSQL 成本库。真实账号与用量来源只读，不执行购买、充值、用卡或改价。

本地页面、原生功能、操作说明和验收边界见 [本地原生成本工作台](../../docs/cost-workbench/LOCAL_NATIVE.md)。

## 运行

Python 3.11+；`costlab.py` 只用标准库。输出目录请置于代码仓库外，严禁提交真实账号/网络/采购记录。

```bash
python -m unittest discover -s tools/cost_research -p 'test_*.py' -v

# 静态公开页：先检查 robots，失败即停止；不会绕过限制。
python tools/cost_research/costlab.py radar --output /tmp/cost-private/radar.json

# 动态公开页：可选 Playwright，匿名新会话，仅允许 Radar 域名普通 GET。
python -m pip install playwright
python -m playwright install chromium
python tools/cost_research/capture_radar.py --output-dir /tmp/cost-private/radar
python tools/cost_research/costlab.py radar --html /tmp/cost-private/radar/radar.html --output /tmp/cost-private/radar-parsed.json

# 已授权的本系统管理员 JWT，由环境注入，不写命令行或仓库。
# SUB2API_ADMIN_TOKEN 必须事先安全注入。
python tools/cost_research/costlab.py account --authorized --origin https://YOUR-OWN-HOST --account-id 1 --kind quota --output /tmp/cost-private/quota-1.json
python tools/cost_research/costlab.py account --authorized --origin https://YOUR-OWN-HOST --account-id 1 --kind rates --output /tmp/cost-private/rates-1.json

# 宿主机已安装并运行 vnstatd，显式选择云厂商计费网卡。
python tools/cost_research/costlab.py traffic --interface eth0 --output /tmp/cost-private/traffic.json

# 公开目录 API；需要时安全注入 OPENROUTER_API_KEY。不会调用付费推理。
python tools/cost_research/costlab.py market --model openai/gpt-5.6-sol --output /tmp/cost-private/market-sol.json
```

## 当前交付边界

`public_snapshot.json` 是通过网页检索获得后整理的公开初步信息，不冒充本地脚本端到端采集结果。当前原生入口已验证 Radar 公开页面读取及独立成本库保存。原型爬取仍要求有效 robots 策略；robots.txt 返回 HTML 时拒绝将其视为允许采集。没有完成真实服务器 vnStat 采样或上游重置操作验收。

静态解析器仅把已明确标注的档位周额度提升为字段；GPT-6 分模型额度、七天平均、两个月事件率没有证据时保持 null。动态页面的 JSON 被捕获为候选证据，仍需校验字段、来源日期、档位、预算池及价格版本后接入，不能随便找一个数字叫额度。没有编造未公开的历史 API 或调用验证码绕过。

`forecast` 需要显式配置已完整观察的自身事件数、曝光周数、逐事件实际净增比例、基础容量和利用率。先验形状参数/率参数使用每周单位；不同类型的事件必须分开建模。

```json
{
  "own_exposure_complete": true,
  "weekly_reference_capacity": 1000,
  "billing_days": 30,
  "cash_and_allocated_cost": 200,
  "prior_event_shape": 1,
  "prior_exposure_weeks": 1,
  "own_observed_weeks": 4,
  "own_event_count": 4,
  "verified_net_refill_fractions": [0.2, 0.4, 0.6, 0.8],
  "useful_utilization": 0.5,
  "seed": 7
}
```

上面全部是测试假设，不是你的实测值。保存为仓库外的 config.json 后：

```bash
python tools/cost_research/costlab.py forecast --config /tmp/cost-private/config.json --output /tmp/cost-private/forecast.json
```

算法实现为 Gamma-Poisson 事件数后验预测 + 净增比例 Bayesian bootstrap。基础容量、利用率暂为条件输入，所以输出是条件情景区间，不包含全部不确定性，也不是已发生账单。原生 Go 的双窗口回放、观察留存与页面后端已经实现，具体边界以原生说明和当前机器记录为准；完整容量校准、真实上游重置验收和自动定价不由这个 Python 预测器证明。

复用现有后台额度查询可能刷新本地显示缓存/有效令牌，这是原业务 GET 的行为；本工具不主动兑换重置卡。原有自动重置若已开启，其既有任务不在本研究工具控制范围内。
