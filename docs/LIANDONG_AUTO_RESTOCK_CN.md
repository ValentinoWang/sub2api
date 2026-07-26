# 链动小铺自动补货

管理员可在“兑换码管理”页面查看链动未售库存、设置最低库存和每次补货数量，并启动或停止服务端自动补货。关闭浏览器不会停止任务；只有页面中的“停止自动补货”会持久关闭任务并取消当前链动请求。

## 运行配置

自动补货默认关闭。生产环境必须同时配置：

- `LIANDONG_RESTOCK_MERCHANT_TOKEN`：链动商户登录令牌。
- `LIANDONG_RESTOCK_CODE_SECRET`：至少 32 个字符的独立随机密钥。
- `LIANDONG_RESTOCK_PRODUCTS_JSON`：链动商品与 Sub2API 美元额度的固定映射。
- `LIANDONG_RESTOCK_INTERVAL_SECONDS`：检查周期，最小 30 秒，建议 300 秒。

商品映射示例只描述结构，不包含生产商品 ID：

```json
[
  {
    "cny_amount": 20,
    "usd_credit": 2.78,
    "goods_id": 12345,
    "threshold": 5,
    "restock_count": 20,
    "enabled": true
  }
]
```

`goods_id` 必须从链动商户后台确认，不能根据标题、价格或商品链接猜测。Token、卡密密钥和明文卡密不会通过管理接口回显。

## 故障与恢复

每批卡密由独立密钥和持久化批次 ID 确定性派生。Sub2API 会先幂等创建兑换码，再以 `remove_repeat=1` 上传。若上传超时、服务重启或链动返回错误，pending 批次会保留；下次启动或定时检查只重试同一批，不生成替代批次。

链动接口来自商户网页的登录态接口，不属于已确认的公开稳定 API。Token 过期或接口变更时，当前周期失败并在管理页显示错误，不会绕过失败继续生成新批次。修复 Token 或映射后，由管理员重新启动。
