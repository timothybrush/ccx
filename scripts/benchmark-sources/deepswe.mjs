/**
 * deepswe 数据抓取器
 *
 * 从 https://deepswe.datacurve.ai 抓取 leaderboard 数据。
 *
 * 数据来源：
 * - v1.1: /artifacts/v1.1/leaderboard-live.json (live leaderboard, 持续更新)
 * - v1:   /artifacts/v1/leaderboard.json (frozen, May 2026)
 *
 * 数据结构：
 * - rows: [{model, harness, reasoning_effort, pass_rate, pass_at_1, pass_at_4, ...}]
 * - 每个模型可能有多个 harness + reasoning_effort 组合
 */

const BASE_URL = 'https://deepswe.datacurve.ai'

/**
 * 获取 deepswe leaderboard 数据
 * @param {string} version - 'v1.1' 或 'v1'
 * @returns {Promise<Object>}
 */
export async function fetchLeaderboard(version = 'v1.1') {
  const endpoint = version === 'v1.1' ? 'leaderboard-live' : 'leaderboard'
  const url = `${BASE_URL}/artifacts/${version}/${endpoint}.json`

  console.log(`[deepswe] Fetching ${url}`)

  const resp = await fetch(url, {
    headers: {
      'User-Agent': 'ccx-benchmark-updater/1.0',
      Accept: 'application/json',
    },
  })

  if (!resp.ok) {
    throw new Error(`HTTP ${resp.status} ${resp.statusText} for ${url}`)
  }

  return resp.json()
}

/**
 * 获取 v1-delta 数据 (v1 vs v1.1 对比)
 * @returns {Promise<Object>}
 */
export async function fetchV1Delta() {
  const url = `${BASE_URL}/artifacts/v1.1/v1-delta.json`

  console.log(`[deepswe] Fetching ${url}`)

  const resp = await fetch(url, {
    headers: {
      'User-Agent': 'ccx-benchmark-updater/1.0',
      Accept: 'application/json',
    },
  })

  if (!resp.ok) {
    throw new Error(`HTTP ${resp.status} ${resp.statusText} for ${url}`)
  }

  return resp.json()
}

/**
 * 从 leaderboard 数据中提取每个模型的最佳表现
 *
 * 策略：对每个模型，取 pass_at_1 最高的记录
 * （也可以考虑取 pass_at_4 最高，但 pass_at_1 更能反映实际使用体验）
 *
 * @param {Object} data - leaderboard JSON 数据
 * @param {Object} modelMap - deepswe 模型名 -> CCX canonicalModel 映射
 * @returns {Array} - [{canonicalModel, passRate, passAt1, passAt4, harness, reasoningEffort, nAttempted, nTasks, source, capturedAt}]
 */
export function extractBestPerModel(data, modelMap) {
  const rows = data.rows || []
  const bestPerModel = new Map()

  for (const row of rows) {
    const deepsweModel = row.model
    const canonical = modelMap[deepsweModel]

    if (!canonical) {
      continue // 跳过未映射的模型
    }

    const existing = bestPerModel.get(canonical)
    const score = row.pass_at_1 ?? row.pass_rate ?? 0

    if (!existing || score > existing.score) {
      bestPerModel.set(canonical, {
        canonicalModel: canonical,
        deepsweModel,
        score,
        passRate: row.pass_rate,
        passAt1: row.pass_at_1,
        passAt4: row.pass_at_4,
        harness: row.harness,
        reasoningEffort: row.reasoning_effort,
        nAttempted: row.n_attempted,
        nTasks: row.n_tasks_attempted,
        nTasksPassed: row.n_tasks_passed_any,
        ciLo: row.ci_lo,
        ciHi: row.ci_hi,
        ciHalf: row.ci_half,
        nRuns: row.n_runs,
        source: row.source,
      })
    }
  }

  return Array.from(bestPerModel.values())
}

/**
 * 计算 cohort percentile
 * @param {Array} allRows - 所有 leaderboard rows
 * @param {number} score - 当前模型的分数
 * @returns {number} - 百分位数 (0-1)
 */
export function calculateCohortPercentile(allRows, score) {
  const allScores = allRows
    .map(r => r.pass_at_1 ?? r.pass_rate ?? 0)
    .filter(s => s > 0)
    .sort((a, b) => a - b)

  if (allScores.length === 0) return 0

  const index = allScores.findIndex(s => s >= score)
  return index === -1 ? 1 : index / allScores.length
}

/**
 * 生成 benchmarkEvidence 对象
 * @param {Object} modelData - extractBestPerModel 的输出
 * @param {Array} allRows - 所有 leaderboard rows (用于计算 percentile)
 * @param {string} benchmarkVersion - benchmark 版本 (如 'v1.1')
 * @returns {Object} - benchmarkEvidence 条目
 */
export function toBenchmarkEvidence(modelData, allRows, benchmarkVersion = 'v1.1') {
  const percentile = calculateCohortPercentile(allRows, modelData.score)

  return {
    benchmark: 'deepswe',
    benchmarkVersion,
    sourceModel: modelData.deepsweModel,
    domain: 'coding',
    metric: 'pass_at_1',
    rawValue: modelData.score,
    uncertainty: modelData.ciHalf || 0,
    cohortPercentile: percentile,
    taskCount: modelData.nTasks,
    cohortSize: modelData.nAttempted,
    effort: modelData.reasoningEffort,
    selectionBasis: 'best_available_effort',
    sourceUrl: `${BASE_URL}/`,
    capturedAt: new Date().toISOString().split('T')[0],
  }
}

/**
 * 主函数：抓取并转换 deepswe 数据
 * @param {Object} modelMap - deepswe 模型名 -> CCX canonicalModel 映射
 * @returns {Promise<Object>} - {canonicalModel: {benchmarkEvidence, deepsweMeta}}
 */
export async function fetchDeepsweData(modelMap) {
  try {
    const [v11Data, v1Data] = await Promise.all([
      fetchLeaderboard('v1.1'),
      fetchLeaderboard('v1').catch(() => null), // v1 是 frozen，可能不可用
    ])

    const result = {}

    // 处理 v1.1 (live leaderboard)
    if (v11Data?.rows) {
      const bestV11 = extractBestPerModel(v11Data, modelMap)
      for (const model of bestV11) {
        const evidence = toBenchmarkEvidence(model, v11Data.rows, 'v1.1')
        if (!result[model.canonicalModel]) {
          result[model.canonicalModel] = { benchmarkEvidence: [], deepsweMeta: {} }
        }
        result[model.canonicalModel].benchmarkEvidence.push(evidence)
        result[model.canonicalModel].deepsweMeta = {
          harness: model.harness,
          reasoningEffort: model.reasoningEffort,
          passAt4: model.passAt4,
          ciLo: model.ciLo,
          ciHi: model.ciHi,
          nRuns: model.nRuns,
        }
      }
    }

    // 处理 v1 (frozen, 作为参考)
    if (v1Data?.rows) {
      const bestV1 = extractBestPerModel(v1Data, modelMap)
      for (const model of bestV1) {
        if (!result[model.canonicalModel]) {
          result[model.canonicalModel] = { benchmarkEvidence: [], deepsweMeta: {} }
        }
        // v1 数据作为补充，如果 v1.1 没有该模型的数据则添加
        const existingEvidence = result[model.canonicalModel].benchmarkEvidence.find(
          e => e.benchmark === 'deepswe' && e.benchmarkVersion === 'v1.1'
        )
        if (!existingEvidence) {
          const evidence = toBenchmarkEvidence(model, v1Data.rows, 'v1')
          result[model.canonicalModel].benchmarkEvidence.push(evidence)
        }
      }
    }

    console.log(`[deepswe] Extracted data for ${Object.keys(result).length} models`)
    return result
  } catch (err) {
    console.error(`[deepswe] Failed to fetch data:`, err.message)
    throw err
  }
}
