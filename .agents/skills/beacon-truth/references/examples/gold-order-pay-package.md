# Gold example: 订单支付 / 关单 / 退款 + Intent Matrix（v1.6.10 R3）

> Twin: `docs/beacon/v1.6.10/features/truth-gold-order-pay-v1610/`  
> 示范 **Truth Review Gate Check A**（Intent Coverage Matrix）

```markdown
---
slug: demo-order-pay
language: zh
revision_id: R3
domain_required: true
---

## 人话

买家下单后付款。可能成功、失败重试，或超时关单。付成功后未发货可退款；已发货要拦住。重复回调只算一次。

## Intent Coverage Matrix

| intent_id | strength | landing | status |
|-----------|----------|---------|--------|
| INT-PAY-01 发起支付 | must | AC-PAY-001 | covered |
| INT-PAY-03 成功幂等 | must | AC-PAY-003 | covered |
| INT-PAY-05 超时关单 | must | AC-PAY-022 | covered |
| INT-RF-03 shipped 拦截 | must | AC-RF-003 | covered |
| INT-EDGE-partial-refund | should | Non-goal | deferred |

## FSM 五元组（摘要）

created + PayRequested → paying  
paying + PaymentSucceeded → paid  
paying + PayTimeout → closed  
paid + RefundRequested[not_shipped] → refunding  
refunding + RefundSucceeded → refunded  

## Truth Review Gate

Check A/B/C：见 docs package 示范结论 → review_gate_pass 形态为 true  
Humanizer：人话短句，无「标志着/赋能」腔
```
