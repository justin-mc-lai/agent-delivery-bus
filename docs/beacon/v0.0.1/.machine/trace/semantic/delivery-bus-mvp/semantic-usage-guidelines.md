# Semantic Usage Guidelines: delivery-bus-mvp

## 使用规则

- 先引用 semantic-contract.json，再进入 prototype 或 implement，不要在阶段内临时翻译需求。
- 页面名、状态名、模块名优先复用官方词表，避免 implementation / verify / delivery 这类旧阶段词汇出现在前台。
- 允许 taste-skill 做设计增强，但不得覆盖 requirement truth 或反向改写 semantic contract。
- 发现语义漂移时，优先重建 semantic layer，再继续 prototype freeze 或 implement delivery。

## 默认设计桥接

- repo: https://github.com/Leonxlnx/taste-skill
- skill: taste-skill
- detected: no
- install: npx skills add https://github.com/Leonxlnx/taste-skill --skill taste-skill
