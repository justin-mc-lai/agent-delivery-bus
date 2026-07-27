# Friction closeout 30s（v1.6.8）

```bash
# 1) 安装/版本真相
beacon doctor diagnose-runtime -p . --json

# 2) longrun 合同（项目默认 true，无需神秘 env）
beacon goal longrun-doctor -p . --json
beacon goal run "ship feature X" -p . -v v1.6.8 --json   # longrun 随 project default

# 3) 热修 release 门减负
beacon release check v1.6.8 -p . --profile hotfix --json

# 4) 主轴
beacon goal axis validate -v v1.6.8 --program beacon-friction-closeout-v168
```
