# 添加渠道预检异步化 + 后台 discovery 落盘续传（修订版）

## 背景与真因

用户痛点："添加渠道点创建按钮会等很久"，英伟达 NIM 渠道尤甚（TODO.md:90）。

经代码核查，真因**不是** capability-test（链路 B，已异步 Job + 轮询），而是**链路 A 前端前置同步 discover**：

- `QuickAddChannelForm.handleSubmit`（frontend/src/components/QuickAddChannelForm.vue:342）在提交前 `await discoverAutoAddRoutes(...)` → `api.discoverChannelConfig` → `POST /api/channel-discovery` → 后端 `ChannelDiscovery` handler（internal/handlers/channel_discovery.go:238）**完全同步**：
  1. `discoverTransientModels` 拉上游 `/models` 全量列表（英伟达几百模型 → 慢）
  2. `runDiscoveryProtocolProbes`：4 协议 × 全量模型，串行 + pacer 限速（RPM 30 = 每 2s 一个），**无首个成功早退**（runDiscoveryProtocolProbe 把每个模型都测完）
  3. `runCompatDiagnose` + `runDiscoveryCapabilityProbes`（vision/toolcall）更多探测
  4. 全部跑完才 `c.JSON` 返回
- 跑完才 `await autoAddChannel`（写配置）→ `submitting=false` 关对话框。

**关键已有能力（方案可极简化的支点）**：后端 `autoAddChannel`（`handleCustomAutoAdd`）本身已经是“建渠道即返回 + 后台异步补全”：

- `routes` 可空（默认单条 `{{requestKind}}`）、`SupportedModels` 可空（= 全模型）
- 建的渠道 `AutoManaged=true`；当渠道级 `SupportedModels` 为空时，模型路由由 `ModelProfileStore → ModelResolver` 运行时解析
- 创建后立即 `triggerDiscoveryForChannel → TriggerDiscovery`（auto_discovery.go:117）起 goroutine 跑全量 `discoverEndpoints` + `writeProfiles`，任务完成后画像才落盘
- handler 同步部分只建渠道，**不阻塞等探测**

→ 前端那个前置全量 discover 是**重复劳动**（后端 auto-add 自己会再触发一遍后台 discovery）。真正要做的是：前端不再等待全量协议探测，改为“快速获取一个真实模型并探测协议 → 直接 autoAddChannel → 关闭对话框”，后端已有后台 discovery 接管完整模型清单；再给后台 discovery task 加 SQLite 落盘 + 重启续传。

### 修订后的设计约束

- **fast 探活必须优先使用上游真实模型名**：先请求 `/models`（或已知 provider manifest），不能对任意自定义上游盲探固定的 GPT/Claude/Gemini 名称。
- **探测模型不是渠道白名单**：fast 结果中的 `testedModel` 只用于证据和测试，不得写入 `UpstreamConfig.SupportedModels`。
- **任务完成以画像持久化成功为准**：不能先把 task 标记为 `done`，再异步写 ProfileStore。
- **续传身份必须稳定且脱敏**：使用 `keyHash`/`credentialUID` + canonical endpoint UID，不使用 `KeyMask` 作为唯一键。
- **续传策略必须二选一**：若采用端点级 checkpoint，则必须定义 SQLite 原子事务，或明确“先画像 Flush、后 checkpoint”的至少一次写入顺序；若只做整体重跑，则不引入虚假的“跳过已完成端点”语义。

## 范围（用户已确认）

- kind 来源：提交前**快速获取一个真实模型并探测协议**定 primaryKind（不做全量模型协议探测）
- 改动范围：**前端**跳过前置全量 discover + **后端** discovery 任务加 SQLite 落盘与重启续传
- 用户另有意图（不在本任务硬绑定）：capability-test（链路 B）转内部自动、隐藏前端入口——本方案只在末尾标注为后续项，不在此实现

## 现状关键文件

- 前端
  - `frontend/src/components/QuickAddChannelForm.vue:331` handleSubmit、:323 discoverCustomRoutes
  - `frontend/src/components/AddChannelModal.vue:421` discoverAutoAddRoutes 调用点
  - `frontend/src/services/autopilot-api.ts:225` discoverAutoAddRoutes、:208 autoAddChannel
  - `frontend/src/services/api.ts` discoverChannelConfig
- 后端
  - `backend-go/internal/handlers/channel_discovery.go:238` ChannelDiscovery handler（同步全量）
  - `backend-go/internal/handlers/channel_discovery.go:1577` runDiscoveryProtocolProbes、:1586 runDiscoveryProtocolProbe（无早退）
  - `backend-go/internal/autopilot/handlers_auto_managed.go:1556` handleCustomAutoAdd、:2442 triggerDiscoveryForChannel
  - `backend-go/internal/autopilot/auto_discovery.go:56` DiscoveryTask、:69 AutoDiscoveryRunner、:117 TriggerDiscovery、:141 runDiscovery、:210 discoverEndpoints
  - `backend-go/internal/autopilot/discovery_task_store.go`（新增）任务持久化与 GC
  - `backend-go/internal/autopilot/profile_store.go` ProfileStore（内存缓存 + SQLite 异步持久化 + loadAll 启动恢复）
  - `backend-go/internal/autopilot/schema_migration.go` autopilot schema v6→v7 迁移
  - `backend-go/main.go:1286` runner 初始化；服务启动前的续传 hook

## 设计

### 第 1 步：后端新增“快速探活”接口（获取真实模型并定 kind）

不改造原 `ChannelDiscovery`（它仍负责完整 discovery/能力诊断），新增 handler `ChannelDiscoveryFast`，只复用其中的临时渠道构造、模型拉取和单模型测试基础设施。

- 路由统一为：`POST /api/channel-discovery-fast`，与现有 `POST /api/channel-discovery` 一致，避免把 URL 中的 `{kind}` 同时当作用户输入和自动探测结果。
- 新增独立的 `ChannelDiscoveryFastRequest`，支持 `baseUrls` + `apiKeys`（兼容单个 `apiKey`），并透传 `authHeader/customHeaders/proxyUrl/insecureSkipVerify`；`channelKind` 只作为可选提示，不能限制自动探测结果。
- fast 探活按 `(baseURL, key)` 组合做有上限的尝试，选择第一组能成功获取真实模型并完成协议探测的凭证；不能只固定使用 `apiKeys[0]` 导致第二个有效 Key 被忽略。响应只返回 `testedKeyHash`，不返回明文 Key。
- 选择探测模型：
  1. 调用现有 models fetcher 获取上游真实模型列表；只做模型清单请求，不执行全量协议探测。
  2. 用现有 `selectDiscoveryModels` 选择一个首选真实模型。
  3. 只有已知 provider manifest 能提供模型清单时，才允许在 models 请求失败后使用 manifest；禁止对任意自定义上游直接使用固定 GPT/Claude/Gemini 名称。
  4. 既没有真实模型也没有可信 manifest 时返回明确错误，前端不创建渠道。
- 协议探测：
  1. 对同一个真实模型探测 `messages/responses/chat/gemini`，不再对每个协议遍历全量模型。
  2. 可以并行以降低延迟，但必须让 `discoveryProbePacer` 具备并发安全性；若保持串行，则使用专用的 fast timeout，不能声称时延恒等于一次请求。
  3. 收集成功结果后复用现有 `recommendDiscoveryChannelKind` 选择 `primaryKind`，不能以协议数组顺序中的“首个成功”代替推荐逻辑。
  4. `executeModelTest` 只负责测试，不把测试模型写成渠道白名单。
- 返回体：

  ```json
  {
    "primaryKind": "chat",
    "testedModel": "meta/llama-3.1-8b-instruct",
    "streamingSupported": true,
    "testedKeyHash": "…",
    "rateLimit": {}
  }
  ```

  `primaryKind` 必须来自成功协议；`testedModel` 和 `streamingSupported` 只作为证据展示。若需要创建路由，前端只提交 `routes:[{channelKind: primaryKind}]`，不提交 `supportedModels`。
- 所有协议探测失败时返回 4xx/可识别的业务错误，**不建渠道**；错误信息不能泄露 API Key。
- 单测覆盖：真实 NIM 模型名、models endpoint 失败且无 manifest、第一组凭证失败但第二组成功、双协议同时成功时的推荐类型、限速/超时和并发 pacer。

### 第 2 步：前端改提交流程

`QuickAddChannelForm.handleSubmit`（:331）与 `AddChannelModal`（:421）：

- 把 `await discoverAutoAddRoutes(...)`（全量）换成 `await discoverFast(...)`（调第 1 步接口，仅探一个真实模型）
- 拿到 `primaryKind` 后调用 `autoAddChannel(primaryKind, { name, baseUrls, apiKeys, routes: [{ channelKind: primaryKind }] })`；不要把 `testedModel` 放入 `supportedModels`
- `submitting=false; emit('added', index)` → 对话框关闭
- **不再等后台 discovery**（后端 auto-add 内部已 `triggerDiscoveryForChannel` 后台补全）
- provider 模式（`isProviderMode`，仅传 providerId+apiKeys）**不动**——它本就不前置 discover
- API 层 `autopilot-api.ts` 新增 `discoverFast`，`api.ts` 新增 `discoverChannelConfigFast`；补齐 `AutoAddFastDiscovery` 类型和响应校验
- 错误处理：探活全失败 → 沿用 `extractAutoAddErrorMessage` 提示，不提交
- `images/vectors` 不调用 fast discovery，保持现有“不支持协议探测”的直接添加路径
- 可选：关闭后保留一个非阻塞的“后台探测进行中”轻提示（读现有 `getTask`/snapshot），非本步必须

### 第 3 步：后端 discovery task 落盘 + 重启续传（核心缺口）

现状：`AutoDiscoveryRunner.tasks` 是内存 map，`runDiscovery` 当前先把 task 标记完成，再统一写 ProfileStore。进程中途退出时既没有可靠 checkpoint，也无法判断画像是否已经持久化。需要补齐“至少一次执行 + 幂等写入 + 启动续传”。

#### 3.1 SQLite 表与迁移

复用 `ProfileStore.DB()` 对应的 autopilot SQLite，不新开第二个数据库。当前 autopilot schema 为 v6，本变更升级到 v7；v6→v7 迁移和新库建表都必须幂等，并使用事务。

```sql
CREATE TABLE IF NOT EXISTS autopilot_discovery_tasks (
  channel_uid       TEXT PRIMARY KEY,
  account_uid       TEXT NOT NULL DEFAULT '',
  channel_kind      TEXT NOT NULL DEFAULT '',
  status            TEXT NOT NULL,              -- running/done/failed
  started_at        INTEGER,
  finished_at       INTEGER,
  error             TEXT NOT NULL DEFAULT '',
  endpoints_payload TEXT NOT NULL DEFAULT '[]', -- 脱敏 JSON
  created_at        INTEGER NOT NULL,
  updated_at        INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_discovery_tasks_status
  ON autopilot_discovery_tasks(status);
```

`started_at/finished_at/created_at/updated_at` 统一使用 Unix milliseconds。`endpoints_payload` 不保存明文 API Key，也不能仅依赖 `KeyMask`；每个端点结果至少包含：

- `endpointUID`（由 channelUID + canonical baseURL + keyHash 生成）；
- `keyHash`、`credentialUID`、`baseURL`；
- `models`、`modelsCount`、`protocolOk`、错误和 discovery metadata；
- `profilePersisted`，表示该结果对应的画像已经成功 Flush。

#### 3.2 DiscoveryTaskStore 与状态机

新增 `DiscoveryTaskStore`，底层使用 `ProfileStore.DB()`，不要让 `AutoDiscoveryRunner` 直接散落 SQL。`AutoDiscoveryRunner` 注入该 store，并保持现有内存 map 作为快速读缓存。

- `TriggerDiscovery` 创建 `running` 记录时清空旧 payload；`channel_kind` 从 `cfgManager` 通过 `findChannelIndexAndKind` 推导，`account_uid` 从当前 channel 读取。
- 数据库写入失败时不得启动 goroutine，`TriggerDiscovery` 需要能区分“已在运行”和“持久化失败”（例如返回 `(started bool, err error)`），让 handler 分别返回 409 或 503；现有内存去重（running 拒绝重复触发）保留。
- `runDiscovery` 必须按端点增量处理：
  1. 探测一个 `(baseURL, credential)` 组合；
  2. 用当前真实 key 写入该端点的 ProfileStore/ModelProfileStore，并强制 Flush；
  3. 只有画像 Flush 成功后，才把该端点以 `profilePersisted=true` 写入 task checkpoint。

  这样即使在画像写入和 checkpoint 之间崩溃，恢复时也只是重复一次幂等写入；不能出现“checkpoint 已成功但画像不存在”。
- `writeProfiles` 拆出单端点版本并返回 error；不能继续吞掉持久化错误。最终 task 只有在所有端点处理完且最后一次 Flush 成功后才能更新为 `done`；全部端点失败则为 `failed`。
- context 被取消时保留 `running` 状态和已持久化 checkpoint，不得把部分结果标记为 `done`。
- GC 使用 runner 生命周期中的定时器，定期删除 `done/failed` 且 `finished_at` 超过 24 小时的记录；不得删除 `running`。

#### 3.3 重启续传

在 `main.go` 创建 runner 后、HTTP server 开始接受请求前，调用同步的 `ResumeIncompleteDiscoveries(ctx, cfgManager)`：

- 读取所有 `status='running'` 记录，并先恢复内存 task，再启动 goroutine；启动完成前 `auto-discover` 请求不可进入。
- 按 `channel_uid` 从当前配置查找 channel。找不到时把任务标记为 `failed`，错误为“渠道已删除”，保留记录供诊断，不静默删除。
- 根据 `endpointUID/keyHash/credentialUID` 匹配当前凭证和 canonical baseURL；只跳过 `profilePersisted=true` 且 endpointUID 仍匹配当前配置的结果。没有稳定身份或画像未持久化的结果必须重试。
- 恢复任务继续使用同一套 `runDiscovery`，保证正常触发和重启续传走同一状态机；内存已有同 channelUID 的 running task 时跳过，避免重复 goroutine。
- runner 使用服务级 root context，关闭时取消未完成任务并保留 `running` 状态，下一次启动继续处理。

#### 3.4 discoverEndpoints 增量化

保留现有 `discoverEndpoints` 作为无 checkpoint 的测试兼容包装，新增内部增量实现，例如：

```text
discoverEndpointsWithCheckpoint(ctx, channel, previousResults, onEndpoint)
```

- 遍历 `(baseURL, resolved key)` 组合时计算稳定 endpointUID；仅跳过 payload 中 `profilePersisted=true` 且 endpointUID 仍匹配当前配置的结果。
- 失败端点不跳过，重启后允许重试；模型列表变化时以本次结果覆盖旧画像。
- 自动托管渠道必须先通过现有 `resolveAutoManagedKeys` 解析实际 key，再生成 keyHash/credentialUID；不得因为配置中的 APIKeys 已脱敏而丢弃结果。
- `runDiscovery` 仍只负责 `/models` 清单和画像写入，不引入 ChannelDiscovery handler 的逐模型协议/能力探测。

### 第 4 步（可选/后续，不在本任务实现）

用户提到"能力测试转内部自动、隐藏前端入口"：
- EditChannelModal 的"测试能力"按钮（frontend/src/utils/editChannelPayload.ts:56 createHandleTestCapability）隐藏
- capability-test Job 加 SQLite 落盘 + 重启续传（capability_test_jobs.go 内存 map → 同第 3 步模式）
- 列为本任务后续，待用户确认是否同期做

## 实现顺序与验证

1. 后端 `discover-fast`：真实 `/models` 选择、已知 manifest fallback、单模型协议探测、推荐 kind、错误脱敏和 pacer 并发安全；补充 handler 单测。
2. 前端 `discoverFast`：补类型和响应校验，修改 `QuickAddChannelForm`/`AddChannelModal`，确认请求中不含 `supportedModels`。
3. 后端 schema v6→v7：创建 task 表、迁移测试、新库建表测试、时间和 payload 序列化测试。
4. `DiscoveryTaskStore`：实现 start/checkpoint/finish/load-running/GC，并测试数据库写失败时不启动任务。
5. `AutoDiscoveryRunner`：端点级幂等画像写入、Flush 后 checkpoint、取消保留 running、最终持久化成功后才 done。
6. `ResumeIncompleteDiscoveries`：在服务开始接收请求前恢复；覆盖渠道已删除、凭证变化、重复触发和 endpointUID 不匹配。
7. 故障注入测试：画像 Flush 前崩溃、checkpoint 后崩溃、重启后重复写入不产生重复画像。
8. 验证：`cd backend-go && make test`、`cd backend-go && make lint`、`cd frontend && bun run type-check`、`cd frontend && bun run build`。
9. 全部验证通过后，按项目约定仅提交本任务相关文件，commit message 使用 `feat: 添加渠道预检异步化与后台 discovery 断点续传`。

## 风险与边界

- discover-fast 在没有真实模型、可信 manifest 或成功协议时**不建渠道**；错误只展示可操作原因，不展示 key/baseURL 原文。
- fast 探测模型永远不写入渠道级 `SupportedModels`；后台模型清单由 endpoint profile/model profile 提供，避免创建后暂时只支持一个模型。
- 后台 discovery 可能在崩溃恢复时重复请求 `/models` 或重复写画像；写入必须幂等，不能以重复一次为失败。
- endpoint checkpoint 只保存脱敏身份和结果；key rotation、baseURL canonicalization 或 credential UID 改变后不得盲目跳过旧结果。
- `/models` 本身极慢时，fast 接口仍可能超时；需要独立 timeout、可识别错误和前端重试提示，不能承诺固定 1–3 秒。
- provider 模式、images/vectors 快速添加和 capability-test 后续项保持不变。
- task 表与现有 autopilot SQLite 共用连接和 schema 版本，不新开数据库、不修改发布产物。
