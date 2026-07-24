# 火山 Agent Plan AFP 感知模型路由实施计划

> 状态：提案。本文仅定义实施边界与验证路径；不在本计划中直接修改路由策略或生产配置。

**目标：** 让 CCX 对火山 Agent Plan 的模型选择使用可审计的 AFP 实际抵扣成本，并把模型能力、质量与成本作为独立维度处理，避免把 `kimi-k3`、`glm-5.2` 和 DeepSeek 模型按粗粒度套餐类别混为同级候选。

**范围：** `backend-go/internal/config/`、`backend-go/internal/autopilot/`、相关管理 API、Autopilot 路由诊断与中文文档。

**非目标：** 修改用户显式 `modelMapping`、绕过 `X-Channel`/会话 override、以 AFP 定价推断模型质量、把 Agent Plan 的 AFP 规则擅自应用到未经核验的 Coding Plan 计费单位，或自动改写已有渠道优先级。

## 目录

1. [背景、事实源与验收标准](#1-背景事实源与验收标准)
2. [现状、问题与设计不变量](#2-现状问题与设计不变量)
3. [AFP 成本模型与活动规则](#3-afp-成本模型与活动规则)
4. [模型能力、质量与路由决策](#4-模型能力质量与路由决策)
5. [实现分层与数据流](#5-实现分层与数据流)
6. [分任务实施步骤](#6-分任务实施步骤)
7. [测试、灰度与回滚](#7-测试灰度与回滚)
8. [风险、待确认项与提交范围](#8-风险待确认项与提交范围)

## 1. 背景、事实源与验收标准

### 1.1 业务触发

当前火山渠道只识别 Agent/Coding Plan 的套餐身份和窗口用量，未把 Agent Plan 的 AFP 抵扣规则带入模型选优。结果是路由会使用公共 USD 价格、渠道优先级或旧映射做判断，无法解释活动期 `glm-5.2` 的有效系数，也无法区分 `kimi-k3` 与 `glm-5.2` 的显著成本差异。

本计划以用户提供的火山官方套餐说明为事实源；实现前应把来源 URL、抓取日期和适用套餐写入内置数据注释，避免将临时活动误当成永久定价。

### 1.2 Agent Plan 的定价事实

文本模型单次 AFP 消耗为：

`AFP = (inputTokens × inputCoefficient + outputTokens × outputCoefficient) / 10_000`

输入系数按总输入长度分段：`≤32k` 使用模型基础系数的 `0.67`，`(32k, 128k]` 使用 `1`，`>128k` 使用 `2`；输出系数不分段。所有时间以 `Asia/Shanghai` 解释。

| 模型 | 基础输入/输出系数 | 本计划时点（2026-07-24）的有效系数 | 备注 |
|---|---:|---:|---|
| `deepseek-v4-flash` | `0.5 / 0.5` | `0.5 / 0.5` | 低成本标准模型，无活动倍率。 |
| `glm-5.2`、`glm-latest` | `4.5 / 4.5` | `1.125 / 1.125` | `×0.25` 活动至 2026-08-08 23:59:59。 |
| `deepseek-v4-pro` | `5.5 / 5.5` | `5.5 / 5.5` | `×0.4` 活动已于 2026-07-15 00:00:00 结束。 |
| `kimi-k2.6` | `4.5 / 4.5` | `4.5 / 4.5` | `×0.4` 活动已结束。 |
| `kimi-k2.7-code` | `4.5 / 4.5` | `4.5 / 4.5` | `×0.25` 活动已结束。 |
| `kimi-k3` | `10 / 10` | `10 / 10` | 无本轮活动；基础成本约为 GLM 基础成本的 2.22 倍、当前活动成本的 8.89 倍。 |

### 1.3 验收标准

| 领域 | 完成定义 |
|---|---|
| 计费正确性 | 给定模型、输入长度、输出 token、套餐类型与请求时刻，能确定性算出 AFP；边界时刻自动恢复基础系数。 |
| 路由正确性 | 同一 Agent Plan 配额池内以 AFP 比较成本；不会把 AFP 数值直接与其他供应商 USD 数值混算。 |
| 模型区分 | `kimi-k3` 与 `glm-5.2` 具有独立模型画像、质量证据与成本记录；不存在“同一进阶档即等价”的捷径。 |
| 可解释性 | dry-run、Routing Trace 和诊断输出能说明基础系数、活动规则、有效系数、估算 AFP 与未采用原因。 |
| 安全性 | 显式模型映射、手动渠道、协议/能力/上下文约束和 failover 语义保持优先，AFP 数据缺失时 fail-open 到既有策略。 |

## 2. 现状、问题与设计不变量

### 2.1 现有实现边界

- `config.ProviderTemplate` 已有 `ModelCostMultipliers` 与 `ModelQualityPriorities` 扩展点，但火山 `volcengine` 模板尚未填充两者；Compshare 的静态倍率不能复用为火山事实。
- `SmartRouter.buildChannelEntry` 用模型注册表的公共 USD 定价和 `ProviderTimePricingMultiplier` 构造 `EstimatedCost`；后者只有 provider 级起始时间和每日窗口，不能表达模型、套餐、结束时间或输入分段。
- `ModelResolver.rankEligibleModels` 仅在可比较质量候选之间使用 provider 静态倍率；它同样没有 AFP、活动日历或请求输入长度。
- `VolcengineAccessKeyPair.Plan` 已持久化 `agent_plan` / `coding_plan`，且 `VolcenginePlanUsage` 已提供套餐窗口余量，是识别成本比较作用域的可用事实，但并不包含模型抵扣规则。
- `TaskClassWorker` 的默认评分偏向速度、成本与节省；因此未建模 AFP 时，子代理可被旧渠道优先级或错误成本估计导向 DeepSeek。

### 2.2 根因与影响

1. 目前没有“基础 AFP 系数 × 输入分段 × 活动倍率”的统一计算器。
2. 现有 `CostTier` 过粗，不能表示 `deepseek-v4-flash`、活动期 GLM、`deepseek-v4-pro` 和 `kimi-k3` 的真实相对消耗。
3. 公共 USD 价格、AFP 订阅配额和其他供应商套餐的单位不同；直接把数值放入一个排序公式会制造伪精度。
4. 计费分类“进阶”不是质量排序。价格可证明成本不同，不能单独证明 K3 的能力恰为 GLM 的两倍；质量排序必须由能力声明、探测与可审计的人工证据共同决定。

### 2.3 设计不变量

- AFP 只在确认同一火山 Agent Plan 配额作用域的候选之间作为可比成本；跨 provider 或跨未知套餐仅记录，不用裸 AFP 覆盖既有排序。
- `deepseek-v4-flash` 的 AFP 更低并不自动输给 GLM；`kimi-k3` 更贵也不自动被禁止。质量目标、上下文、工具、推理和用户显式意图先完成硬约束过滤。
- 价格活动是数据，不是手工调整渠道 `Priority` 的替代品；活动开始/结束必须由时钟自动生效和回退。
- 活动数据失效、未知模型、未知输入 token 或无套餐身份时不报零成本，不阻断请求，回退到现有公共定价/排序并在 trace 中标记原因。
- Coding Plan 的活动对象虽与 Agent Plan 同属官方活动，但本计划不假设其使用 AFP；未取得其独立计费公式前，只保留通用活动元数据，不将 Agent Plan AFP 套入 Coding Plan。

## 3. AFP 成本模型与活动规则

### 3.1 单一、可版本化的政策目录

在 `backend-go/internal/config/` 新建火山套餐定价目录，而不是把值散落在 `ProviderTemplate`、路由器和前端预设中。目录至少包含：

```go
type VolcengineAFPModelRule struct {
    RuleID, Plan, SourceURL string
    ModelIDs                []string // 精确 ID；glm-latest 作为 glm-5.2 的显式别名
    InputBase, OutputBase   AFPScaledCoefficient
    Promotions              []AFPPromotionRule
}
type AFPPromotionRule struct {
    PromotionID string
    StartsAt, EndsAtExclusive time.Time // Asia/Shanghai，[start, end)
    Multiplier AFPScaledCoefficient
    SourceURL string
}
```

系数使用固定精度整数（建议 1,000,000 分之一 AFP）而不是浮点累积；`4.5 × 0.67 × 0.25` 必须稳定得到 `0.75375`。目录只包含官方已确认数据、来源和生效窗口，首版不自动抓取网页或在运行时联网更新。

### 3.2 解析与计算接口

新增纯函数 `ResolveVolcengineAFPCost(at, plan, modelID, inputTokens, outputTokens)`，返回：

- 是否命中已知 Agent Plan 规则、规则 ID、基础/活动系数、输入分段和活动 ID；
- 输入、输出与总 AFP 的固定精度结果；
- `CostConfidence` 与不可计算原因（未知 plan、未知模型、未知输入 token、未知输出 token）。

匹配顺序固定为：精确模型 ID/别名 -> 套餐类型 -> 输入分段 -> 在 `[startsAt, endsAtExclusive)` 内的活动倍率。官方展示为 `23:59:59` 的结束时刻在目录中规范化为下一秒的排他边界；`00:00:00` 的语义必须由正式页面复核后写入。多个活动重叠是启动期配置错误，不允许“最后一条覆盖”；无活动则乘以 `1`。历史已结束活动仍保留，供 trace 回放和边界测试，不能在活动结束后从目录删除。

### 3.3 请求前估算与请求后核算

路由前只使用请求画像中可安全获得的输入 token 估算，以及显式输出上限或已定义的输出预算；缺失任一值时保留系数向量但降低成本置信度，不能伪造 `0 AFP`。路由后使用上游返回/本地核算的实际输入和输出 token 计算实际 AFP，作为诊断与未来质量/成本证据，不反馈改写已完成请求。

`RequestProfile` 应扩展为“输入 token 估算、输出 token 预算、来源与置信度”，由所有协议入口统一构建。首版不引入 LLM 预测器；客户端显式 `max_tokens`、Scheduler 的上下文约束和已有本地 token 估算是唯一允许输入。

### 3.4 成本比较作用域

引入内部 `CostEvidence`：`{unit, scopeID, estimated, actual, confidence, source}`。同一 `agent_plan` 账号/套餐窗口内的候选使用 `unit=afp`、相同 `scopeID`，可以直接按 `estimated` 排序；公共按量渠道使用 `unit=usd`。不同单位或不同配额池不直接比较数值，只使用各自的健康、质量、用户优先级和已有的保守 fallback 规则。

可选的“AFP 货币化”必须以后续显式用户配置的每 AFP 价值为前提，不在首版隐式猜测。这样不会把“12 AFP”错误地当作“12 美元”，也不会让一个余额已耗尽的套餐因名义 AFP 更低而被强行选中。

### 3.5 关键样例

对 `100k` 输入、`10k` 输出的普通文本请求，活动期 Agent Plan 的预期值如下：

| 实际模型 | 预期 AFP | 路由含义 |
|---|---:|---|
| `deepseek-v4-flash` | `5.5` | 若能力足够，是成本最低的候选。 |
| `glm-5.2` | `12.375` | 活动期高性价比候选，但并不比 Flash 更低成本。 |
| `deepseek-v4-pro` | `60.5` | 其活动已结束，不能继续按折扣价排序。 |
| `kimi-k3` | `110` | 只能在其模型级质量/能力收益足以证明合理时升级使用。 |

该表是路由测试夹具，不是对外账单。真实计费仍以火山控制台为准；差异超过约定容差时只记录告警、暂停 AFP 成本影响，并回退既有策略。

## 4. 模型能力、质量与路由决策

### 4.1 独立的模型级画像

在现有模型注册表能力字段之外，为火山已验证模型建立显式路由画像：`QualityTier`、同档 `QualityPriority`、上下文窗口、工具调用、推理模式、视觉能力、来源证据与复核日期。`kimi-k3`、`glm-5.2`、`deepseek-v4-pro` 和 `deepseek-v4-flash` 必须分别建档；不得从“文本生成（进阶）”或 AFP 系数直接继承同一个质量档。

`ProviderTemplate.ModelQualityPriorities` 可以作为同质量候选的稳定 tie-breaker，但火山条目只有在能力探测、官方规格和人工评审均齐备后才填入。AFP 成本只进入成本维度，不能凭“10 对 4.5”自动断言 K3 的质量是 GLM 的两倍。

### 4.2 选模顺序

对一个可自动解析的火山 Agent Plan 候选，按以下顺序决定实际模型：

1. 保留用户显式 `modelMapping`、`X-Channel`、会话 override、promotion、协议、上下文、视觉、工具、推理与健康约束。
2. 使用模型画像过滤不满足能力下界的模型；复杂度、长上下文和工具需求必须提高质量/能力下界，即使请求来自 `subagent`。
3. 在满足下界的最小质量收益带内，比较同一 AFP 作用域的有效 AFP；低置信度成本不改变顺序。
4. 若 AFP 相近，再比较模型级质量优先级、实测成功率、延迟、渠道健康和稳定的用户偏好。
5. 无合格火山模型或选择失败时走现有 Scheduler/failover；不得因为 AFP 计算失败而拒绝请求。

### 4.3 策略含义

- `deepseek-v4-flash` 适合作为能力满足时的低成本 worker/lightweight 候选；它不是所有子代理的强制默认值。
- 活动期 `glm-5.2` 是中高质量、长上下文任务的有效 AFP 候选，不能被过期的 Pro/Kimi 活动倍率压低。
- `kimi-k3` 是独立的高成本模型画像。它仅在明确质量目标、模型能力证据或用户策略满足时参与升级，而不是被粗粒度“进阶”标签合并。
- 主代理与子代理可拥有不同质量预算，但不能用“子代理”这一标签无条件降低复杂任务的质量下界；角色专用序列/ManualRoutingIntent 仍是用户可见、可撤销的优先控制面。

## 5. 实现分层与数据流

### 5.1 职责划分

| 层 | 计划变更 | 不负责 |
|---|---|---|
| `internal/config` | 内置 AFP 政策目录、计划作用域解析、固定精度计算 DTO、模型级质量元数据的校验与深拷贝。 | 发起火山计费请求、保存密钥或修改渠道排序。 |
| `internal/autopilot` | 从请求画像构造成本证据；在 `ModelResolver` 和 `SmartRouter` 复用同一计算结果；记录可解释摘要。 | 取代 Scheduler 的硬约束与 endpoint failover。 |
| `internal/metrics` | 接收实际 token 用量并记录实际 AFP/偏差的安全指标。 | 将估算 AFP 当作火山官方账单。 |
| 管理 API / 前端 | 展示规则来源、活动状态、有效系数、成本置信度和 dry-run 解释。 | 直接编辑内置活动规则或回显凭证。 |

### 5.2 套餐作用域解析

新增只读 `ResolveVolcenginePlanScope(upstream, apiKeyConfig)`：优先通过 `AccountUID + CredentialUID` 查询自动托管账号中的 `VolcengineAccessKeyPair.Plan`，再用已验证的火山数据面 URL 作受限回退。返回稳定 `scopeID`、计划种类、套餐状态、用量快照时间和可比性；不返回 AK/SK、API Key 或原始 URL。

只有 `plan == agent_plan` 且套餐状态可用时返回 AFP 可比作用域。无法解析 credential、使用手工中转地址、用量过期或配额窗口已耗尽时，将 AFP 置信度降为不足并保留现有调度语义。

### 5.3 请求与结果数据流

`协议入口 -> RequestProfile -> PlanScopeResolver -> AFPCostResolver -> ModelResolver -> SmartRouter -> Scheduler/EndpointPolicy -> 实际 token 统计 -> AFP outcome/Trace`

同一个请求将 `AFPCostEvidence` 放入 context，避免 `ModelResolver`、`SmartRouter` 与 outcome 分别按不同时间重新计算活动状态。请求开始时固定 `pricingEvaluatedAt` 与政策版本；在途请求跨过活动结束时仍使用入口快照，下一请求自动使用新规则。

### 5.4 可解释输出

在既有 dry-run 与 trace 候选字段中追加可选的安全摘要：`costUnit`、`costScope`（匿名稳定 ID）、`baseInputCoefficient`、`baseOutputCoefficient`、`inputSegment`、`promotionID`、`promotionApplied`、`estimatedAFP`、`actualAFP`、`costConfidence` 和 `pricingPolicyVersion`。不输出密钥、完整 BaseURL、账号名、控制台响应或请求正文。

当 Trace v2 实施计划先落地时，这些字段进入其 DTO 和持久化详情；若未落地，则先作为向后兼容的可选 JSON 字段写入当前 Trace，避免两项工作互相阻塞。

## 6. 分任务实施步骤

所有后端行为先写失败测试，再补实现；每个任务只提交该任务必要的文件。活动规则以编译期内置目录起步，后续更新必须同时更新来源、有效期和测试，不通过远程网页抓取静默改变运行时成本。

### Task 1：建立 Agent Plan AFP 政策与纯计算器

**文件：** 新建 `backend-go/internal/config/volcengine_afp_pricing.go`、`backend-go/internal/config/volcengine_afp_pricing_test.go`。

1. 为输入分段、固定精度系数、模型规则、活动窗口、计算结果和不可计算原因建立专用类型；禁止复用 `float64` 的 `ModelCostMultipliers` 表示账务事实。
2. 收录 Agent Plan 基础系数、`glm-latest` 别名和历史/当前活动窗口；每条规则附官方来源与最后核验日期。
3. 实现无副作用的 `ResolveVolcengineAFPCost`，返回基础与有效系数、活动命中、总 AFP 和置信度。
4. 先写表驱动测试：三档输入边界、零 token、超大 token、`glm-latest`、未知模型、活动开始/结束精确时刻、已结束的 Pro/Kimi 促销、整数溢出与稳定小数展示。

### Task 2：解析火山套餐作用域与可比较性

**文件：** 新建或修改 `backend-go/internal/config/volcengine_plan_scope.go`、对应 `_test.go`；必要时复用 `config_accounts.go` 的只读查询。

1. 通过 `AccountUID`、`CredentialUID` 和已绑定的火山套餐元数据解析 `agent_plan` / `coding_plan`、匿名 scope ID、状态与用量快照。
2. 仅在凭证关联缺失时，使用已知 `/api/plan`、`/api/coding` URL 作为低置信度 hint；不从渠道名称、API Key 或自由文本猜测套餐。
3. 建立同 scope、不同 scope、未知 scope、失效套餐、用量耗尽、多个 endpoint/key 的测试矩阵；断言返回值不会带出任何密钥或原始 URL。
4. Coding Plan 返回可识别但 `AFPComparable=false`，直到其独立计费规则被补充和测试覆盖。

### Task 3：把成本证据接入请求画像

**文件：** 修改 `backend-go/internal/autopilot/request_profile*.go`、新增 `afp_cost_evidence.go` 及测试。

1. 在统一请求画像中增加输入 token 估算、输出预算、来源和置信度；六类代理入口的缺省值语义必须一致。
2. 请求入口一次性生成 `PricingSnapshot`（政策版本、评估时间、scope、AFP 结果），并写入 context；任何后续步骤只能读取该快照。
3. 为显式 `max_tokens`、上下文约束、未知输出预算、流式/非流式、长上下文三个输入段写测试，确保未知不会伪装成免费。

### Task 4：统一 ModelResolver 与 SmartRouter 的成本比较

**文件：** 修改 `backend-go/internal/autopilot/model_resolver.go`、`smart_router.go`、相关测试；按需收敛 `provider_templates.go` 的静态倍率职责。

1. 让两个选优路径都消费同一 `CostEvidence`，禁止一处用 AFP、另一处继续用公共 USD 后给出冲突答案。
2. 在同一 AFP scope 内按有效 AFP 排序；跨单位、跨 scope 或低置信度候选不比较裸成本，维持既有质量/健康/优先级顺序。
3. 保留 `ModelQualityPriorities` 作为独立且有证据的同档 tie-breaker；为 K3、GLM、DeepSeek 建立不依赖价格推断的模型级画像测试夹具。
4. 写失败测试覆盖：Flash 对 GLM 的成本胜出、活动期 GLM 对 Pro 的成本胜出、K3 仅在更高质量约束下升级、显式 `modelMapping` 不被 AFP 覆盖、未知 AFP fail-open。

### Task 5：暴露诊断、实际 AFP 与可控启用

**文件：** 修改 `backend-go/internal/autopilot/handlers_dryrun.go`、`routing_trace*.go`、对应 API/前端类型与测试。

1. dry-run 返回每个候选的安全 AFP 摘要、未比较原因和排序影响，便于在不发真实请求时复核规则。
2. 请求结束后以实际 token 用量计算 AFP，并在 Trace/窗口指标中区分 `estimated` 与 `actual`；计量失败不影响响应。
3. 增加独立 `afpCostRoutingEnabled` 开关，默认关闭；`shadow` 只记录建议，`assist/auto` 必须经过第 7 节的观察门槛后才能启用。
4. 管理端只展示只读活动状态和来源；不提供在线编辑官方倍率的入口，活动更新通过版本化代码/预设发布。

### Task 6：模型策略与文档更新

**文件：** 修改模型注册表/内置预设、`docs/` 中火山渠道说明及相关单测。

1. 明确“Agent Plan AFP”与“Coding Plan 未核验计费”边界，列出数据来源、活动结束时间和更新责任。
2. 将 `kimi-k3`、`glm-5.2`、`deepseek-v4-pro`、`deepseek-v4-flash` 的能力、质量证据和成本事实分开呈现；不再用“进阶”作为路由同义词。
3. 不在本任务自动改写用户已有映射。若要把主代理定向 K3、子代理/长上下文定向 GLM，另建经用户确认的角色专用渠道序列或 ManualRoutingIntent 变更。

## 7. 测试、灰度与回滚

### 7.1 分层测试矩阵

| 层级 | 验证内容 |
|---|---|
| 纯函数 | AFP 公式、固定精度、输入分段、别名、活动起止、历史回放、未知值和溢出。 |
| 配置 | 套餐 scope 解析、深拷贝、热重载、密钥脱敏、失效/耗尽套餐降置信度。 |
| Resolver/Router | 同 scope AFP 排序、跨 scope 不混算、质量约束、K3/GLM 独立画像、手动映射与 fail-open。 |
| HTTP dry-run | 请求画像到候选解释的完整安全响应；不发送上游请求。 |
| 请求生命周期 | 实际 token 用量回填 AFP、streaming 终态、trace 写入失败与活动边界中的在途快照。 |
| 手工 smoke | 使用已授权的非生产测试凭证核对一个 GLM 活动期样本与火山控制台 AFP；不把凭证、原始响应或账号数据写入测试夹具。 |

### 7.2 上线顺序

1. 发布政策目录、计算器和 trace 字段，但保持 `afpCostRoutingEnabled=false`；确认所有现有请求不改变渠道选择。
2. 在 shadow 模式收集火山 Agent Plan 的建议与实际选择，核查规则 ID、有效系数、scope、输入分段和未知原因。
3. 只有以下条件同时满足才对用户明确选定的单一作用域启用 assist：计算错误为零、手动映射零回归、跨单位比较零发生、GLM/Pro/Flash/K3 样例与人工 AFP 核验一致。
4. 自动模式只在 shadow/assist 证据满足既有 Readiness/SLO 门槛后由用户显式启用；不把本轮活动作为全局自动重排的授权。

### 7.3 回滚

- `afpCostRoutingEnabled=false` 必须立即让成本证据退化为“仅观测”，不删除 trace、模型画像或原有策略。
- 发现来源数据变更、AFP 与控制台偏差超阈值、错误跨 scope 排序或手动路由受影响时，关闭该开关并记录政策版本/活动 ID。
- 过期活动修正应更新目录并发布新版本；不得通过修改历史 trace 或手工拖动渠道优先级掩盖问题。

### 7.4 完成前验证命令

```bash
git diff --check
cd "backend-go" && GOCACHE="/tmp/go-build" GOMODCACHE="/tmp/go-mod" go test ./internal/config ./internal/autopilot ./internal/scheduler
cd "backend-go" && GOCACHE="/tmp/go-build" GOMODCACHE="/tmp/go-mod" go vet ./...
make build
cd "backend-go" && make test
cd "frontend" && bun run build
```

真实火山 smoke 不属于默认 CI；仅由拥有对应测试套餐且明确授权的操作者运行。

## 8. 风险、待确认项与提交范围

### 8.1 风险控制

| 风险 | 控制措施 |
|---|---|
| 官方活动变更或提前结束 | 政策目录保留来源、核验日期和版本；默认关闭成本路由；活动更新走代码审查与边界测试。 |
| 把 AFP 当作跨供应商货币 | 以 `unit + scopeID` 强制隔离；缺少用户定义的兑换规则时禁止跨单位比较。 |
| 价格被误当作质量 | 模型级能力和质量证据独立维护；质量排序变更必须有探测/评审依据。 |
| 输出 token 估算不准 | 将估算与实际 AFP 分开；低置信度不影响排序；用控制台样本校准后再灰度。 |
| 临近活动边界的在途请求抖动 | 入口固定 `PricingSnapshot`，只让后续请求使用新活动状态。 |
| 规则泄露账号信息 | scope 仅使用服务器生成的匿名 ID；管理 API、trace、测试夹具都不含 AK/SK、API Key 或完整端点。 |

### 8.2 实施前待确认项

1. 由套餐所有者确认 Agent Plan AFP 表的官方来源、时区和活动结束时刻是否采用闭区间；如官方页面改动，以最新页面为准。
2. 获取 Coding Plan 的独立用量公式和基础系数前，不启用其 AFP 成本比较。
3. 为 `kimi-k3`、`glm-5.2` 等模型确认可复现的质量/能力证据与所需任务类，不以价格倍率代替评测。
4. 确认将 AFP 仅用于同一套餐 scope 的相对排序，还是未来引入经用户配置的 AFP 货币化预算；两种策略不得混合上线。
5. 确认 GLM 活动结束后是否需要保留历史 trace 的复算视图；本计划默认保留原政策版本，不重算历史决策。

### 8.3 事实源

- 火山 Agent/Coding Plan 指定模型抵扣系数活动：<https://www.volcengine.com/docs/82379/2533565>
- 火山 Agent Plan AFP 定义、公式与模型抵扣系数：以本计划讨论中提供的官方表为准，实施时补入可公开访问的正式文档 URL 与核验日期。
- CCX 当前事实源：`backend-go/internal/config/provider_templates.go`、`backend-go/internal/autopilot/model_resolver.go`、`backend-go/internal/autopilot/smart_router.go`、`backend-go/internal/config/config.go`。

### 8.4 提交范围

实施时按 Task 拆分提交，避免把 AFP 政策、选路语义、前端展示和用户路由策略混在同一个 commit。计划文档本身使用：

```bash
git add -- docs/superpowers/plans/2026-07-24-volcengine-afp-aware-routing.md
git commit -m "docs: plan volcengine AFP-aware routing"
```
