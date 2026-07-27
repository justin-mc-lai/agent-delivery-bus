# Longrun 默认 30 秒（v1.6.8）

新项目默认 `goal.longrun_default=true`：

```bash
beacon goal run "把订单域 FSM 做完并验收" -v v1.6.8
# 自动 attach driver/supervise（无需 --longrun）

# 强制关闭本轮
beacon goal run "小修文案" --no-longrun
```
