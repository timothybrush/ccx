# 火山套餐 Key 保活与渠道恢复交互修复计划

> 状态：提案。本文定义实现边界、验证路径与上线恢复步骤；不在计划阶段修改运行配置或恢复现有密钥。

**目标：** 修复火山 Agent Plan/Coding Plan 混合 Key 渠道的跨套餐误探测与 `/models` 误判，并在渠道全部 Key 不可用时直接提供“恢复”操作。

**范围：** `backend-go/internal/healthcheck/`、火山数据面探针共享实现、`backend-go/main.go`、`frontend/src/components/ChannelOrchestration.vue`、`frontend/src/utils/channelApiKeys.ts` 及相关测试。

**非目标：** 修改火山 AK/SK 管控面识别、改变套餐计费规则、自动清除所有历史认证失败、扩大保活频率、停止或重启 3688 端口进程。

## 目录

1. [背景与问题证据](#1-背景与问题证据)
2. [目标、非目标与设计不变量](#2-目标非目标与设计不变量)
3. [后端修复设计](#3-后端修复设计)
4. [前端恢复交互设计](#4-前端恢复交互设计)
5. [文件影响与实施任务](#5-文件影响与实施任务)
6. [测试矩阵与验收标准](#6-测试矩阵与验收标准)
7. [上线、状态恢复与回滚](#7-上线状态恢复与回滚)
8. [风险与提交建议](#8-风险与提交建议)

## 1. 背景与问题证据

### 1.1 用户可见现象

同一火山自动托管账号内同时存在 Agent Plan 与 Coding Plan 推理 Key。Agent Plan Key 已通过 AK/SK 管控面识别，界面能正确展示 `Agent Plan · Medium` 和套餐窗口余量，但后台保活仍将其标记为“认证失败”。当四个 Key 全部因认证或额度原因进入禁用列表时，渠道行仍显示“暂停”按钮，用户必须先暂停渠道，才能看到“恢复”按钮。

这两个现象分别来自后端探测路由和前端动作状态判断，但共享同一个业务事实：渠道配置状态仍是 `active`，实际可用凭证数已经为零。

### 1.2 后端误判链路

当前保活在 `healthcheck.Manager` 中先取得渠道级 `u.GetAllBaseURLs()`，随后把同一 URL 列表传给每个 API Key。混合套餐渠道的地址顺序可能是：

1. `https://ark.cn-beijing.volces.com/api/coding`
2. `https://ark.cn-beijing.volces.com/api/plan`

即使 Agent Plan Key 的 `APIKeyConfig.BaseURL` 已绑定到 `/api/plan`，L1 仍会先使用渠道首个 `/api/coding` 地址。L1 又复用通用 `GetChannelModels`，最终形成类似 `GET /api/coding/v1/models` 的请求。上游返回 401/403 后，当前逻辑立即终止候选遍历、记录 `auth_failed` 并调用黑名单回调，因此不会再尝试该 Key 自己绑定的 Agent Plan 地址。

本地只读核查已经确认：

- 被误判的 Key 管控面计划为 `agent_plan`，状态为 `Running`；
- 该凭证绑定的数据面地址为 `/api/plan`；
- 同一渠道的默认地址为 `/api/coding`；
- L1 记录在同一时刻出现连续三次 `auth_failed`，错误内容来自火山数据面的无效凭证响应。

### 1.3 `/models` 本身不适用于火山套餐 Key

`builtin_models_manifest.go` 已明确声明火山 Agent/Coding Plan 的模型发现依赖 AK/SK 管控面签名接口，普通推理 Key 不能通过 `/v1/models` 探测，因此相关清单均为 `DisableProbe=true`。Autopilot 新增渠道时也已经存在专用数据面探针：

- Agent Plan 使用 `/api/plan`，模型为 `auto`；
- Coding Plan 使用 `/api/coding`，模型为 `ark-code-latest`；
- Claude 协议探针补齐 Claude Code 请求头、system 身份和 session metadata。

因此，即使先修复 per-key BaseURL，继续使用通用 `/models` 仍然可能制造假阴性。正确修复必须同时解决“端点归属”和“探针协议”两个问题。

### 1.4 前端动作误判链路

`ChannelOrchestration.vue` 当前仅在渠道状态为 `suspended` 或熔断器状态为 `open` 时显示恢复按钮；否则一律显示暂停按钮。它没有使用已经存在的 `availableChannelApiKeyCount()` 与 `disabledChannelApiKeyCount()` 判断渠道是否已经没有可用 Key。

后端渠道恢复接口本身没有该限制：`ResumeChannelWithKind` 会恢复全部禁用 Key、重置熔断指标并返回 `restoredKeys`，不要求渠道必须先处于暂停状态。因此“先暂停、再恢复”完全是前端动作选择造成的额外用户步骤。

## 2. 目标、非目标与设计不变量

### 2.1 完成目标

| 领域 | 完成定义 |
|---|---|
| Key 路由 | 已绑定 `APIKeyConfig.BaseURL` 的 Key 只在自己的端点上执行 L1/L2，不参与渠道级 BaseURL 笛卡尔积。 |
| 火山探针 | Agent/Coding Plan 不再调用通用 `/models`；使用与套餐入口、协议和请求特征匹配的数据面探针。 |
| 错误分类 | 只有 Key 自己绑定的正确端点明确返回可识别的 401/403 时，才记录 `auth_failed` 并拉黑。 |
| 模型来源 | 火山专用探针成功后使用管控面发现结果或内置套餐清单为后续选模提供模型集合，不从探针响应臆造模型。 |
| 恢复交互 | `可用 Key=0` 且 `禁用 Key>0` 时，渠道行直接显示恢复按钮，一次点击恢复全部禁用 Key。 |
| 兼容性 | 历史手填且没有 per-key BaseURL 的渠道保持当前多 BaseURL 回退行为。 |

### 2.2 非目标

- 不改变 `GetPersonalPlan` 对 Agent/Coding Plan 的识别流程。
- 不改变 Agent Plan 的 `GetAFPUsage` 或 Coding Plan 的 `GetCodingPlanUsage`。
- 不把 AK/SK 校验结果当作推理 Key 的数据面可用性证明。
- 不为所有 `DisableProbe=true` 的供应商一次性设计通用插件系统；首版只抽取足够复用的火山探针边界。
- 不自动恢复所有历史 `authentication_error`，因为其中可能包含真实失效凭证。
- 不改变额度耗尽 Key 的自动恢复时间，也不绕过用户手动暂停的 `enabled=false` 状态。
- 不增加后台探测频率，不让一次调度周期对同一火山 Key 重复执行等价真实调用。

### 2.3 设计不变量

1. **绑定优先：** per-key BaseURL 是经过接入探针确认的凭证归属，优先级高于渠道默认 BaseURL。
2. **历史兼容：** `BoundBaseURLForKey` 返回空时才使用 `GetAllBaseURLs()`，旧配置继续工作。
3. **协议严格：** 火山套餐探针必须根据 BaseURL 选择 Agent/Coding 模型，并根据 `serviceType` 选择 Messages 或 OpenAI 兼容请求。
4. **错误保守：** 网络错误、5xx、404、模型/参数校验错误不能被升级为认证失败；只有明确且可识别的认证响应触发拉黑。
5. **单次事实：** 同一探测结果同时承担可达性判断和模型集合解锁，不在 L1 成功后无条件重复发送同类探针。
6. **恢复可解释：** “恢复”按钮只表示存在后端 `RestoreAllKeys` 能恢复的禁用 Key；没有 Key 或只有手动暂停 Key 时不显示。
7. **不泄密：** 日志、测试夹具和文档只保留掩码、计划类型、URL 路径与错误类别，不记录明文 Key、AK/SK 或完整上游请求 ID。

### 2.4 原则应用

- **KISS：** 复用现有 `BoundBaseURLForKey`、恢复接口和 Key 计数工具，不新增平行配置字段。
- **DRY：** 火山新增渠道验证与后台保活共享同一个数据面探针实现，避免请求特征再次漂移。
- **SOLID：** BaseURL 解析、上游探针、健康状态处置和 UI 动作选择分别保持单一职责。
- **YAGNI：** 不在本次引入供应商探针注册中心、数据库迁移或自动清洗历史黑名单。

## 3. 后端修复设计

### 3.1 统一 per-key BaseURL 解析

在 `config.UpstreamConfig` 上新增只读辅助方法，或在 `healthcheck` 内封装等价纯函数：

```go
func BaseURLsForKey(upstream *config.UpstreamConfig, apiKey string) []string {
    if bound := upstream.BoundBaseURLForKey(apiKey); bound != "" {
        return []string{bound}
    }
    return upstream.GetAllBaseURLs()
}
```

该函数用于 L1、L2 和失败指标归因。不得直接修改原 `UpstreamConfig`，避免并发保活任务污染共享配置快照。

L1 调度从“渠道先算一次 BaseURLs”调整为“每个 Key 单独解析 BaseURLs”。绑定 Key 遇到 401/403 后仍然立即停止，因为此时响应来自该 Key 自己确认过的端点；未绑定历史 Key 保留当前候选遍历语义。

### 3.2 抽取共享火山数据面探针

将 `autopilot/verify_endpoint.go` 中火山专用请求构建与判定提取到不依赖 Autopilot 状态的共享位置。优先选择职责中性的 `internal/upstreamprobe/`；如果现有包边界更适合，也可放入 `internal/handlers/` 的探针模块，但不得让 `healthcheck` 反向依赖 `autopilot`。

建议接口：

```go
type Result struct {
    OK         bool
    StatusCode int
    AuthFailed bool
    Body       []byte
    Err        error
}

func ProbeVolcenginePlan(
    ctx context.Context,
    serviceType, baseURL, apiKey, authHeader string,
) Result
```

探针行为固定为：

| 套餐入口 | Claude/Messages | OpenAI 兼容 |
|---|---|---|
| Agent Plan | `POST /api/plan/v1/messages`，`model=auto`，Claude Code 特征 | `POST /api/plan/v3/chat/completions`，`model=auto` |
| Coding Plan | `POST /api/coding/v1/messages`，`model=ark-code-latest`，Claude Code 特征 | `POST /api/coding/v3/chat/completions`，`model=ark-code-latest` |

探针只接受真实 2xx 为成功。401/403 标记 `AuthFailed=true`；其他 4xx、5xx 与网络错误保留原状态，不推断 Key 无效。请求体保持最小输出限制，避免不必要的额度消耗。

Autopilot 新增渠道验证改为调用该共享函数，删除原有重复火山分支；已有通用 Claude/OpenAI/Kimi 验证保持不变。

### 3.3 火山 L1 策略

L1Fetcher 在收到火山官方套餐 BaseURL 时，不再进入 `GetChannelModels`：

1. 调用共享 `ProbeVolcenginePlan` 验证数据面 Key 与入口组合。
2. 成功后查询 `config.LookupBuiltinManifest(baseURL, normalizedServiceType)`。
3. 将 `ModelIDs` 编码为标准 OpenAI models 列表响应，复用现有 `countModels` 与 `extractModelIDs`。
4. 失败时原样返回状态码和受限响应体，让现有错误分类逻辑处理。

若自动托管凭证已完成 AK/SK 模型发现，可在后续实现中优先使用绑定模型清单；本次以现有内置清单作为稳定回退，不把该扩展作为阻塞项。

### 3.4 L2 使用同一 Key 绑定端点

当前 L2 只把 `probeChannel.APIKeys` 裁剪为单个 Key，仍保留渠道默认 `BaseURL/BaseURLs`。修复时应同时设置：

```go
probeChannel.APIKeys = []string{apiKey}
probeChannel.BaseURL = resolvedBaseURLs[0]
probeChannel.BaseURLs = append([]string(nil), resolvedBaseURLs...)
probeChannel.DisabledAPIKeys = nil
probeChannel.APIKeyConfigs = config.NormalizeAPIKeyConfigsForView(probeChannel)
```

实际实现应只保留当前 Key 的配置，不能把其他凭证配置带进探针副本。失败指标 `recordFailure` 也必须记录本次真实使用的 bound BaseURL，而不是渠道首个地址。

对于火山官方 `first` tier 渠道，默认会执行 L2。由于火山 L1 已经发送真实最小调用，需要避免同周期重复等价请求。建议让 L1 结果携带 `realCallVerified=true`，调度器据此跳过该 Key 的 L2；其他 provider 维持原行为。若首版不扩展 outcome DTO，则可以在明确识别火山专用 L1 时直接跳过 L2，并记录结构化日志说明原因。

### 3.5 错误分类与黑名单边界

保持 `ShouldBlacklistKey` 为唯一业务分类入口，但调用它之前必须满足：

- 请求发送到了该 Key 的 bound BaseURL，或该 Key 没有绑定且当前为历史兼容路径；
- 状态码为 401/403；
- 响应体可被现有分类器识别为认证、权限或其他明确需要禁用的原因。

火山探针返回 400/404 时记录 `error`，不喂认证黑名单；网络和 5xx 继续进入熔断失败指标。这样既修复误杀，也不会弱化真实无效 Key 的隔离能力。

## 4. 前端恢复交互设计

### 4.1 可恢复状态定义

在 `channelApiKeys.ts` 增加纯函数，集中表达渠道是否因全部黑名单 Key 而可恢复：

```ts
export function hasOnlyDisabledChannelApiKeys(channel: ChannelKeyState): boolean {
  return availableChannelApiKeyCount(channel) === 0
    && disabledChannelApiKeyCount(channel) > 0
}
```

条件必须包含 `disabled > 0`，避免“尚未配置任何 Key”的渠道误显示恢复。首版只统计 `disabledApiKeys`；纯 `APIKeyConfig.enabled=false` 属于用户手动暂停，渠道恢复接口不会改变它，因此不能算作该按钮可处理的状态。

### 4.2 动作判定统一

将 `isBreakerManagedChannel` 重命名为 `isRecoverableChannel`，统一用于行内按钮与更多菜单：

```ts
const isRecoverableChannel = (channel: Channel): boolean => {
  const metrics = getChannelMetrics(channel)
  return channel.status === 'suspended'
    || metrics?.circuitState === 'open'
    || hasOnlyDisabledChannelApiKeys(channel)
}
```

状态语义如下：

| 渠道状态 | Key 状态 | 主操作 |
|---|---|---|
| active，至少一个可用 Key | 任意数量禁用 Key | 暂停 |
| active，无可用 Key且至少一个禁用 Key | 全部被拉黑/耗尽 | 恢复 |
| suspended | 任意 | 恢复 |
| breaker open | 任意 | 恢复 |
| active，无任何 Key | 空配置 | 暂停，不误导为可恢复 |

本次不强制修改渠道状态徽标，避免把运行配置 `active` 与推导出的凭证可用性混成同一字段；如后续需要，可单独增加“无可用 Key”有效状态展示。

### 4.3 单击恢复流程

现有后端 `resume` 接口已经执行“恢复全部 disabled keys + 重置 breaker”，前端直接复用。优化 `resumeChannelInternal`：

1. 先调用 `resume(channelId)`。
2. 仅当渠道当前不是 `active` 时调用 `setStatus(active)`。
3. 对已经是 `active`、仅因全部 Key 禁用而显示恢复的渠道，跳过冗余状态更新，直接触发一次数据刷新。
4. 继续使用 `restoredKeys` 展示“已恢复 N 个被拉黑 Key”的成功提示。

这样用户只需一次点击，网络层也避免一次没有状态变化的 PUT/PATCH 请求。

### 4.4 部分禁用与额度边界

当仍有可用 Key 时保留暂停主操作，用户可在密钥管理中单独恢复禁用 Key。渠道级恢复会恢复包括额度耗尽在内的所有 disabled Key；如果用户在上游重置前主动恢复，Key 可能再次因额度不足被禁用，这是现有明确的手动恢复语义，不在本次额外阻止。

## 5. 文件影响与实施任务

### 5.1 预计文件范围

| 文件/目录 | 计划变更 |
|---|---|
| `backend-go/internal/config/config_utils.go` | 复用或补充 per-key BaseURL 解析辅助函数。 |
| `backend-go/internal/upstreamprobe/` | 新增共享火山套餐数据面探针及单测；最终位置以避免包循环为准。 |
| `backend-go/internal/autopilot/verify_endpoint.go` | 改为复用共享火山探针，删除重复请求构建。 |
| `backend-go/internal/healthcheck/manager.go` | 每个 Key 单独解析 BaseURLs，控制火山 L1/L2 去重。 |
| `backend-go/internal/healthcheck/check.go` | 火山 L1 结果接入、正确错误分类与失败归因。 |
| `backend-go/internal/healthcheck/l2.go` | 裁剪单 Key 配置并覆盖 bound BaseURL。 |
| `backend-go/main.go` | L1Fetcher 适配火山专用探针；不改服务启动顺序。 |
| `frontend/src/utils/channelApiKeys.ts` | 新增全部禁用 Key 的纯状态判断。 |
| `frontend/src/components/ChannelOrchestration.vue` | 统一恢复按钮判定并减少冗余状态请求。 |
| 对应 `_test.go` / `.test.ts` | 覆盖混合套餐、请求特征、黑名单边界和恢复交互。 |

### 5.2 Task 1：锁定 per-key BaseURL

1. 为 Key 解析 bound BaseURL，空绑定回退渠道级地址。
2. 修改 L1 调度，禁止已绑定 Key 遍历其他套餐地址。
3. 修改 L2 探针副本，只保留当前 Key、配置与绑定端点。
4. 修正 `recordFailure` 的 BaseURL 归因。
5. 增加历史未绑定渠道兼容测试。

### 5.3 Task 2：共享火山专用探针

1. 从 Autopilot 提取 Agent/Coding 模型选择、路径拼接和 Claude Code 特征。
2. 定义与调用方解耦的探针结果 DTO。
3. 保持现有 12 秒或调用方 context 超时，不新增独立后台 goroutine。
4. Autopilot 新增渠道验证迁移到共享实现。
5. L1Fetcher 对严格匹配的火山官方套餐地址走专用探针。

### 5.4 Task 3：模型集合与 L1/L2 去重

1. 专用探针成功后从内置 manifest 生成标准模型列表响应。
2. 失败响应保留状态码与截断详情，不伪造 models。
3. 标记该 L1 已完成真实调用，避免同周期再次执行等价 L2。
4. 保持非火山 provider 的 L1/L2 行为不变。

### 5.5 Task 4：渠道恢复主操作

1. 新增 `hasOnlyDisabledChannelApiKeys` 纯函数。
2. 将行内和菜单的恢复判断统一为 `isRecoverableChannel`。
3. active 渠道直接调用 resume 并刷新，不重复 set active。
4. 保持部分 Key 禁用、空 Key、手动暂停 Key 的现有动作语义。

### 5.6 Task 5：验证与提交

1. 运行 Go 定向单测、全量测试和 `go vet ./...`。
2. 运行前端 Vitest、类型检查与生产构建。
3. 不启动、停止或杀掉 3688 端口进程；需要运行态验证时提示用户手动重启。
4. 仅 stage 本任务相关文件，按后端与前端是否可独立回滚决定一个或两个 conventional commits。

## 6. 测试矩阵与验收标准

### 6.1 后端单元测试

| 场景 | 预期 |
|---|---|
| Agent/Coding 两个 Key 各自绑定不同 BaseURL | 每个 Key 只访问自己的端点，不产生跨套餐请求。 |
| 已绑定 Key 的端点返回 401 | 记录 `auth_failed`，调用一次黑名单回调。 |
| 其他套餐端点会返回 401 | 因不会访问该端点，不得误拉黑。 |
| 未绑定历史 Key 配置多个 BaseURL | 维持当前按顺序回退行为。 |
| Agent Claude 探针 | 路径为 `/api/plan/v1/messages`、模型为 `auto`，Claude Code headers/session 一致。 |
| Coding Claude 探针 | 路径为 `/api/coding/v1/messages`、模型为 `ark-code-latest`。 |
| Agent/Coding OpenAI 探针 | 分别命中 `/api/plan/v3/chat/completions` 与 `/api/coding/v3/chat/completions`。 |
| 火山探针 400/404/500 | 记录普通 error，不调用认证黑名单。 |
| 火山探针网络失败 | 记录 error，并将失败归因到 bound BaseURL。 |
| 火山 L1 成功 | 返回内置套餐模型集合，且不再请求 `/models`。 |
| 火山 L1 已真实调用 | 同周期不重复执行 L2。 |
| L2 单 Key 副本 | `BaseURL/BaseURLs/APIKeyConfigs` 仅包含该 Key 的绑定信息。 |

### 6.2 前端单元测试

在 `channelApiKeys.test.ts` 至少覆盖：

- `apiKeys` 中仍保留同名 Key、但 `disabledApiKeys` 有记录时，以 disabled 为准；
- 可用数为 0、禁用数大于 0时，`hasOnlyDisabledChannelApiKeys=true`；
- 部分可用时返回 false；
- `apiKeys=[]` 且 `disabledApiKeys=[]` 时返回 false；
- `null` 列表保持兼容；
- 只有 `enabled=false` 而没有 disabled 记录时不误判为可恢复。

若组件测试基础设施允许，补充 `ChannelOrchestration` 行为测试；否则把动作选择提取为纯函数测试，并保留最小模板验证：

- 全部 Key 禁用显示 `mdi-refresh`；
- 部分可用显示 `mdi-pause-circle`；
- active 且全部 Key 禁用时点击恢复只调用 `resume`，不调用 `setStatus(active)`；
- suspended 或 breaker open 时恢复后仍设置 active。

### 6.3 验证命令

```bash
cd "backend-go"
GOCACHE="/tmp/go-build" GOMODCACHE="/tmp/go-mod" go test ./internal/healthcheck ./internal/autopilot ./internal/upstreamprobe
GOCACHE="/tmp/go-build" GOMODCACHE="/tmp/go-mod" go test ./...
GOCACHE="/tmp/go-build" GOMODCACHE="/tmp/go-mod" go vet ./...

cd "frontend"
bun run test:run
bun run type-check
bun run build
```

若共享探针最终未使用 `internal/upstreamprobe`，定向命令按实际包路径调整。

### 6.4 验收标准

1. 混合套餐渠道中，Agent Plan Key 的任何保活请求都不会访问 `/api/coding`。
2. 火山套餐保活日志中不再出现 `/api/plan/v1/models` 或 `/api/coding/v1/models`。
3. 正确端点返回成功时，Key 不被加入 `DisabledAPIKeys`。
4. 正确端点明确返回认证失败时，现有拉黑与日志脱敏行为保持有效。
5. 四个 Key 全部禁用时，渠道行首次渲染即显示恢复按钮。
6. 用户一次点击即可调用渠道恢复并看到恢复数量；不需要先暂停。
7. 其他 provider、历史未绑定渠道及部分禁用 Key 的交互无回归。

## 7. 上线、状态恢复与回滚

### 7.1 上线顺序

1. 先发布后端 per-key 路由与火山专用探针，阻止新的误拉黑。
2. 再发布前端恢复按钮判断，提供单击恢复入口。
3. 由用户使用界面恢复已确认受本问题影响的火山 Agent Plan Key。
4. 触发一次渠道健康检查或等待下一个调度周期，观察正确端点的 L1 记录。

后端和前端可以同一版本交付，但回滚边界应保持清晰：前端恢复按钮依赖的 `resume` 接口是现有能力，不要求后端新 API。

### 7.2 历史误判状态处理

不进行数据库迁移，也不批量删除 `key_health` 或 `DisabledAPIKeys`。原因是 `authentication_error` 同时可能代表真实无效 Key，无法仅凭 reason 安全区分。

上线后按以下人工路径恢复：

1. 在密钥管理中确认该凭证仍显示正确的 Agent/Coding Plan 与 `Running` 状态。
2. 对全部 Key 禁用的渠道直接点击渠道行“恢复”；部分禁用时可逐个恢复。
3. 观察 `restoredKeys` 提示和后续健康记录。
4. 若正确端点仍返回 401/403，再按真实凭证失效处理，不重复自动恢复。

额度耗尽 Key 在重置时间前手动恢复可能再次被禁用；这属于预期行为。认证失败 Key 没有自动恢复时间，必须由用户明确恢复。

### 7.3 运行态验证

不停止或杀掉当前 3688 端口进程。代码合并后如需让运行实例加载新后端或前端，由用户按现有安全方式手动重启。验证时只观察脱敏日志、渠道状态、请求路径和健康记录，不输出明文凭证。

### 7.4 回滚

- 后端探针异常：回滚共享探针和 healthcheck 调度改动，保留 `BoundBaseURLForKey` 纯函数不影响运行语义。
- 前端动作异常：仅回滚 `isRecoverableChannel` 判断，后端恢复 API 不受影响。
- 不通过回滚恢复被禁用 Key；状态恢复仍使用现有 `resume`/单 Key restore 接口。
- 若出现非火山 provider 回归，优先确认特殊分支是否严格限制在官方火山 host+path，而不是扩大 URL 模糊匹配。

## 8. 风险与提交建议

### 8.1 主要风险

| 风险 | 缓解措施 |
|---|---|
| 专用 L1 与 L2 重复消耗套餐额度 | 显式标记真实调用已完成，同周期跳过等价 L2。 |
| URL 匹配过宽误命中中转站 | 使用解析后的官方 hostname 和精确 path 前缀，不用裸 `strings.Contains` 匹配任意域名。 |
| 400 被误当作鉴权成功 | 火山专用探针只接受 2xx；其他 4xx 记 error，不拉黑。 |
| 历史渠道缺少 per-key 绑定 | 空绑定保持现有渠道级回退，不强制迁移。 |
| active 渠道恢复后界面不刷新 | active 分支显式 emit refresh，等待最新 `disabledApiKeys` 与计数。 |
| 恢复全部 Key 包含未重置额度 Key | 保留现有手动恢复语义，并在成功提示后允许再次显示真实额度状态。 |

### 8.2 待实现时确认

1. 共享探针最终放在 `internal/upstreamprobe` 还是现有探针包，以 `go list` 无循环依赖且职责清晰为准。
2. 自动发现得到的 per-key 模型清单是否能低成本传入 healthcheck；若需要扩大 DTO，本次先使用内置 manifest。
3. `realCallVerified` 放入 `l1KeyOutcome` 还是通过 L1 strategy 类型表达；选择改动最小、测试最直观的方案。
4. 前端组件测试是否已有稳定挂载工具；若没有，优先提取纯状态函数，不为本次引入大型测试框架。

### 8.3 建议提交拆分

若所有验证同时通过，可按以下顺序提交：

1. `fix(healthcheck): respect per-key volcengine plan endpoints`
2. `fix(ui): show resume when all channel keys are disabled`

两个提交分别保持后端安全边界与前端交互边界，便于独立审查和回滚。若共享类型导致无法独立构建，则合并为一个 `fix(volcengine): correct plan key healthcheck and recovery` 提交。

### 8.4 完成定义

实现、测试、构建和文档全部通过后，自动 stage 仅本任务相关文件并本地提交；不执行 `git push`。最终回报测试命令、提交 hash、仍需用户手动重启或恢复历史 Key 的步骤。
