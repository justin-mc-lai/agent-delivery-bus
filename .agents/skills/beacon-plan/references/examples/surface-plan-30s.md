# Surface 30s: plan

## 对外入口（Scheme A）

```text
$beacon-plan
只做规划拆海/审查，先不实现
项目：.
版本：auto
```

## 渐进加载

内部会按 `public-surface-progressive-map.v1.json` 加载细粒度 skill，**能力不降级**。  
用户无需背 `beacon-gov-*` 名；框架自愈见 goal `self_heal` 字段。

## Loom

`$loom-plan` — 同语义，Rust CLI 轨。
