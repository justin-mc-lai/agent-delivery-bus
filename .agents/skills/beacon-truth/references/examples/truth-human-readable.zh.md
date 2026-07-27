# 示例：带人话层的需求真相（中文）

```markdown
---
slug: demo-order-pay
language: zh
---

# 订单支付

## 人话

卖家要能收一笔已创建订单的款。没订单不能付。付完订单变成已支付，才能发货。

- 能做：对 `created` 订单发起支付；失败可重试
- 不能做：跳过创建直接支付；支付成功后当没付过
- 怎样算完：合法路径测过；非法跳转被拒绝

## User Intent

创建订单后完成支付。

## Acceptance Criteria

| AC ID | Description |
|-------|-------------|
| AC-PAY-001 | created → paying → paid |
| AC-PAY-002 | * → paid 无支付记录 → 拒绝 |
```
