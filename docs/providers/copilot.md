# GitHub Copilot

## 获取 GitHub OAuth Token

1. 打开 CCX 管理界面，进入 **Responses** 入口
2. 点击「添加渠道」
3. 选择上游类型 `GitHub Copilot`
4. 填写 `Base URL`（默认 `https://api.githubcopilot.com`，通常不需要修改）
5. 在「身份认证」区域点击「使用 GitHub Copilot 登录」
6. 记录页面显示的授权码，在打开的 GitHub 授权页确认授权
7. 授权成功后，系统会自动把 GitHub OAuth token 写入渠道 `API Keys`

## 在 CCX 中添加渠道

### Responses 入口（推荐）

| 字段 | 值 |
|------|-----|
| 名称 | `GitHub Copilot`（自定义） |
| 服务类型 | `copilot` |
| Base URL | `https://api.githubcopilot.com` |
| API Keys | 上一步授权生成的 GitHub OAuth token（`gho_...`） |

### 配置建议

- 启用 `Codex 原生工具透传`
- 如模型端点不支持图片工具，可开启 `移除 image_generation 工具`
- 按需配置 `Model Mapping`，将 Codex 客户端常用别名映射到 Copilot 实际模型名

## 可用端点

| 协议 | 端点 |
|------|------|
| Responses | `POST /responses` |
| Models | `GET /models` |

说明：`serviceType: "copilot"` 会跳过默认 `/v1` 前缀，最终请求地址为 `https://api.githubcopilot.com/responses`。

## 注意事项

- 渠道中保存的 `API Key` 是 **GitHub OAuth token**，不是短期 Copilot API token
- CCX 会在请求前自动把 GitHub OAuth token 换成短期 Copilot token，并注入 `Authorization: Bearer <copilot_token>`
- 不要手动把 GitHub OAuth token 当作 `Authorization: Bearer` 调用 `api.githubcopilot.com`
- OAuth 相关接口默认要求管理员鉴权，避免对外暴露 GitHub 登录流程
- 如需代理访问，请在渠道「代理地址」字段填写 HTTP/HTTPS/SOCKS5 代理 URL
