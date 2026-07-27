# Hotfix 30s（主线热修 + 分支登记）

```bash
# 1) 计划（可不写 --from-version → 默认当前发布线）
beacon goal hotfix-plan "生产：支付重复扣款" -p . --json
# 查看 hotfix_branch / recommended_commands

# 2) 建分支（示例）
# git switch -c hotfix/v1.6.8-... main

# 3) goal
beacon goal run "生产：支付重复扣款" --mode hotfix -p . \
  --forward-port main \
  --rollback-notes "revert <sha> / flag off"

# 4) 最小发布门
beacon release check <baseline> --profile hotfix -p .
# 人工确认 release；再 forward-port 到开发线
```
