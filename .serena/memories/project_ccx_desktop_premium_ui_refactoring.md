---
name: project_ccx_desktop_premium_ui_refactoring
description: CCX 桌面端 (Wails 3 + Vue 3) 极致高级物理材质与双栏侧边栏 UI 架构重构特征与维护
metadata:
  type: project
---

CCX 桌面端于 2026/05/20 迎来了具有里程碑意义的极高水平 UI 视觉重塑。

### 核心设计特征

1. **双栏原生架构 (Sidebar Layout)**
   - 彻底移除了原来的 Web 顶部药丸导航与硬编码交通灯避让。
   - 引入左侧常驻侧边栏 `Sidebar.vue` + 右侧内容主视口 `App.vue`。
   - 顶栏保留 `data-wails-drag` 作为透明拖拽手柄，侧边栏高度集成“迷你状态守护卡片”（支持红、绿、黄脉冲霓虹呼吸灯实时监控网关）。
   - 采用 `v-show` 进行选项卡常驻缓存，完美保留了内嵌 Web UI 切换时的工作区 session 状态。

2. **物理拟态材质与霓虹状态 (Glow & Glassmorphism)**
   - 在 `index.css` 声明了 `@utility bg-glass` 毛玻璃拟态和 `@utility bg-glass-hover` 交互悬浮。
   - 声明了 `shadow-glow-green`/`shadow-glow-orange`/`shadow-glow-red` 等四类不同网关状态的霓虹发光脉冲。
   - 按钮引入了 `@utility btn-shimmer` 微光扫掠过渡动画，按钮按压具备 `active:scale-95` 物理微交互反馈。

3. **精密监控网格 (MetricsGrid.vue)**
   - 端口、版本、运行时间、上游可用渠道采用等宽字体 `font-mono`。
   - 运行时长进行了智能换算（秒、分、时自适应）。
   - 卡片应用 hover 平移、发光动画。

4. **高级高性能语法高亮终端 (LogViewer.vue)**
   - 不再采用全量重绘；使用 computed 属性预映射、解析。
   - 语法行解析器智能区分 `Scheduler`、`Config`、`Updater` 组件标签，高亮显示 ERROR/WARN/SUCCESS 级别。
   - 终端底部配备高科技闪烁命令提示符，顶栏集成了 **秒级日志搜索过滤框**、**自动滚动到底部锁定器**、**一键复制** 与 **清空本地日志缓存**。

### 未来维护提示
- 界面使用最新的 Tailwind CSS v4 标准，所有材质和实用类均写在 `index.css` 内，在任何新增卡片中引入 `bg-glass bg-glass-hover` 即可实现玻璃发光一致性。
- 由于 `App.vue` 采用了 `v-show` 缓存机制，若在 `App.vue` 下添加新的模块或卡片，确保其在 activeTab 下响应，且在不需要时不会产生严重的异步 API 轮询开销。
