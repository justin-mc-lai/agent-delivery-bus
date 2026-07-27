# Version Fleet 30 秒（v1.6.8）

```bash
beacon version fleet --json
beacon doctor init-dual-version \
  --previous-baseline release/v1.6.7 \
  --next-feature main \
  --hotfix-rule 'hotfix/*'
beacon doctor verify-dual-version --json
```
