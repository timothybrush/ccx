# desktop 模块文档

[← 根目录](../CLAUDE.md)

## 模块职责

CCX 桌面后台 App，基于 Wails 3 提供系统托盘、配置向导、Agent 配置管理、健康检查与本地网关控制入口。

## 技术栈

- Go + Wails 3 桌面壳
- Vue 3 + TypeScript + Vite 桌面前端
- Bun 作为桌面前端主包管理器

## 常用命令

在 `desktop/` 目录执行：

```bash
wails3 dev      # 启动桌面开发模式
wails3 build    # 构建桌面应用
```

桌面前端命令参考 [frontend/CLAUDE.md](frontend/CLAUDE.md)。

## 目录说明

| 路径 | 职责 |
|------|------|
| `frontend/` | 桌面端 Vue 前端界面 |
| `bindings/` | Wails 生成的前后端绑定 |
| `internal/` | 桌面端 Go 服务实现 |

## 桌面前端包管理

`desktop/frontend/` 以 Bun 为主包管理器，`bun.lock` 是权威锁文件；`pnpm-lock.yaml` 仅用于兼容验证。

不要为了运行 `npm audit` 重新生成 `package-lock.json`。安全检查使用：

```bash
bun install --cwd frontend
bun audit --registry=https://registry.npmjs.org --cwd frontend
```

## 注意事项

- 默认假设用户可能已启动 Wails / Vite 开发进程，不要自动杀进程重启
- 修改 Go 服务导出接口后，确认 `bindings/` 是否需要重新生成
- 修改前端依赖优先进入 `frontend/` 使用 Bun 命令
- 桌面端配置管理相关逻辑集中在 `internal/configservice/`
