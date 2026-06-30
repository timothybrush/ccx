# TODO

## 更新规范

### 提交问题

在本文档末尾添加新条目，格式：

```markdown
## [ ] 简短标题

问题描述，包含复现条件和预期行为。
```

如有对应 GitHub Issue，在标题中标注，如 `## [ ] 标题 (#issue号)`。

### 解决更新

问题修复后，将 `[ ]` 改为 `[x]`，并在描述下方追加：

```markdown
**关键提交：**
- `commit_hash` commit message
```

如涉及多文件变更，可补充 `**关键变更：**` 列出受影响文件。

---

## [ ] 管理后台使用报表导出 (#229)

来源：https://github.com/BenedictKing/ccx/issues/229

需求：在管理后台为渠道使用统计增加“导出使用报表”能力，支持按月份（默认当前月，后续可考虑自定义日期范围）导出当前渠道或全部渠道的使用数据。

建议范围：优先支持浏览器下载 CSV；JSON 可作为可选格式；XLSX 和定时邮件属于 nice-to-have，暂不作为首批必做项。

导出内容应覆盖当前仪表盘已有的核心指标：渠道名、服务类型、日期/小时桶、请求数、可用率、输入/输出 token、缓存读写、RPM、TPM。

备注：该需求目前不确定是否有更多用户需要，先记录为待评估项。

---

> **上游版本变更**

## [ ] Codex rust-v0.142.2 上游协议/工具变更评估

发现协议/工具/用法变更：MCP tool search 默认启用、服务端 safety-buffering 可见性与 faster-model 元数据、ResponseItem metadata 新增 turn_id。请评估对 ccx Responses 渠道的影响。

## [ ] Claude Code v2.1.193 上游协议/工具变更评估

发现协议/工具/用法变更：autoMode.classifyAllShell 扩大 Bash/PowerShell 权限分类范围、assistant_response OpenTelemetry 事件可能输出模型响应文本、MCP headersHelper 401/403 后自动重连。请评估对 ccx Messages 渠道和桌面端 Claude Code 集成的影响。

## [ ] Claude Code v2.1.196 上游协议/工具变更评估

发现协议/工具/用法变更：组织默认模型展示、MCP list/get 对仓库自批准 .mcp.json 的安全行为变化、所有 provider 默认启用 streaming idle watchdog、ANTHROPIC_BASE_URL 指向非 Anthropic 主机时禁用 Remote Control。请评估对 ccx Messages 渠道、流式响应保活策略和桌面端 Claude Code 集成的影响。

