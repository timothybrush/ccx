# desktop/frontend 模块文档

[← 根目录](../../CLAUDE.md)

## 模块职责

Wails 3 桌面端前端界面，负责桌面壳内的 Vue 3 UI、配置向导、Agent 配置管理与本地网关交互入口。

## 包管理器

本模块以 Bun 为主包管理器，`bun.lock` 是依赖锁文件的权威来源；`pnpm-lock.yaml` 仅用于 pnpm 兼容验证。

不要为了运行 `npm audit` 重新生成 `package-lock.json`，避免重复锁文件和旧锁文件误报。

## 启动与构建命令

```bash
bun install
bun run dev
bun run build
bun run build:dev
bun run preview
```

pnpm 仅用于兼容验证：

```bash
pnpm install
```

新增或升级依赖优先使用：

```bash
bun add <package>
bun add --dev <package>
bun update
```

## 安全扫描

项目配置了 `bunfig.toml`，在 `bun install` 时自动运行 `@socketsecurity/bun-security-scanner` 拦截恶意依赖。

```bash
bun install                                      # 安装依赖并触发 Socket 安全扫描器
bun audit --registry=https://registry.npmjs.org # 使用 npm 官方 registry 执行漏洞审计
```

说明：如果本地 registry 指向 `npmmirror`，直接运行 `bun audit` 可能因镜像源不支持 audit 接口而返回 404；审计时显式指定 npm 官方 registry。

## 技术栈

- Vue 3
- TypeScript
- Vite
- Wails 3 runtime

## 注意事项

- 桌面前端依赖 Wails runtime 绑定，修改接口调用前先确认 `desktop/bindings/` 与后端服务定义
- 不要自动杀掉或重启用户已启动的 Wails / Vite 开发进程
- 生产构建由桌面端构建流程消费，避免手动移动 `dist/` 产物
