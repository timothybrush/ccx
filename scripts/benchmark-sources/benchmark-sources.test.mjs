import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

import {
  canonicalModelToPattern,
  deepsweModelToPattern,
} from './mapper.mjs'
import {
  extractBestPerModel as extractDeepSWEBest,
  toBenchmarkEvidence as toDeepSWEEvidence,
} from './deepswe.mjs'
import {
  extractTableCacheVersion,
  extractBestPerModel as extractDradarBest,
  extractLeaderboardFromTable,
  toBenchmarkEvidence as toDradarEvidence,
} from './dradar.mjs'
import { extractModelInfo } from './litellm.mjs'
import {
  generatedArtifactPaths,
  mergeBenchlmData,
  mergeDeepsweData,
  mergeLitellmData,
  validateRegistry,
} from '../update-benchmark-data.mjs'
import { presetArtifactPaths } from '../generate-preset-manifest.mjs'
import { buildBenchmarkVisualizationData } from './visualization.mjs'
import { renderBenchmarkChart, validateVisualizationData } from '../generate-benchmark-chart.mjs'

function emptyReport() {
  return { updated: [], added: [], errors: [], litellmUpdated: [], litellmSkipped: [] }
}

function readJson(relativePath) {
  return JSON.parse(readFileSync(new URL(relativePath, import.meta.url), 'utf8'))
}

test('runtime and published preset registries stay synchronized with the shared source', () => {
  const source = readJson('../../shared/model-registry/ccx_model_registry.json')
  const embedded = readJson('../../backend-go/internal/presetstore/embedded/model-registry.json')
  const published = readJson('../../docs/public/presets/model-registry.json')

  assert.deepEqual(embedded, source)
  assert.deepEqual(published, source)
})

test('benchmark updater rolls preset artifacts into its generated output transaction', () => {
  for (const artifactPath of presetArtifactPaths) {
    assert.ok(generatedArtifactPaths.includes(artifactPath), `${artifactPath} is not tracked`)
  }
})

test('canonical pattern generation accepts canonical and source model names', () => {
  const expected = '(?:^|[-/])gpt-5\\.6-sol(?=$|@)'
  assert.equal(canonicalModelToPattern('gpt-5.6-sol'), expected)
  assert.equal(deepsweModelToPattern('gpt-5-6-sol'), expected)
  assert.equal(deepsweModelToPattern('gpt-5.6-sol'), null)
})

test('DeepSWE percentile and cohort use one best row per model', () => {
  const rows = [
    { model: 'model-a', pass_at_1: 0.8, reasoning_effort: 'high', n_tasks_attempted: 100 },
    { model: 'model-a', pass_at_1: 0.7, reasoning_effort: 'low', n_tasks_attempted: 100 },
    { model: 'model-b', pass_at_1: 0.6, reasoning_effort: 'high', n_tasks_attempted: 100 },
  ]
  const best = extractDeepSWEBest({ rows }, { 'model-a': 'a', 'model-b': 'b' })
  const evidence = toDeepSWEEvidence(best[0], best)

  assert.equal(best.length, 2)
  assert.equal(evidence.cohortSize, 2)
  assert.equal(evidence.cohortPercentile, 1)
  assert.equal(evidence.taskCount, 100)
})

test('benchmark evidence normalizes missing effort to default', () => {
  const deepEvidence = toDeepSWEEvidence({
    deepsweModel: 'model-a',
    score: 0.5,
    nTasks: 100,
    reasoningEffort: null,
  }, [{ score: 0.5 }])
  const radarEvidence = toDradarEvidence({
    deepsweModel: 'model-a',
    passRate: 0.5,
    cells: 100,
    bestEffort: null,
  }, [{ passRate: 0.5 }])

  assert.equal(deepEvidence.effort, 'default')
  assert.equal(radarEvidence.effort, 'default')
})

test('CodexRadar table cache version is read from the live page contract', () => {
  assert.equal(
    extractTableCacheVersion('var TABLE_CACHE_VERSION = "20260718-discrimination-toggle-2";'),
    '20260718-discrimination-toggle-2',
  )
  assert.throws(() => extractTableCacheVersion('<html></html>'), /TABLE_CACHE_VERSION/)
})

test('dradar cohort size is model count rather than graded run count', () => {
  const best = extractDradarBest({
    models: [
      { model: 'a', effort: 'high', pass_rate: 0.8, graded: 450, cells: 100, cells_passed: 80 },
      { model: 'b', effort: 'high', pass_rate: 0.6, graded: 440, cells: 100, cells_passed: 60 },
    ],
  }, { a: 'a', b: 'b' })
  const evidence = toDradarEvidence(best.a, Object.values(best))

  assert.equal(evidence.cohortSize, 2)
  assert.equal(evidence.cohortPercentile, 1)
  assert.equal(evidence.benchmark, 'codexradar')
  assert.equal(evidence.benchmarkVersion, 'v1')
})

test('CodexRadar leaderboard aggregation uses strict cell majority', () => {
  const leaderboard = extractLeaderboardFromTable({
    cells: {
      'task-a|gpt-5.6-sol|low': { n: 3, p: 2 },
      'task-b|gpt-5.6-sol|low': { n: 2, p: 1 },
      'task-c|gpt-5.6-sol|low': { n: 3, p: 3 },
      'task-d|ignored|low': { n: 3, p: 3 },
    },
  }, { 'gpt-5.6-sol': 'gpt-5.6-sol' })

  assert.deepEqual(leaderboard.models, [{
    model: 'gpt-5.6-sol',
    effort: 'low',
    graded: 8,
    passed: 6,
    cells: 3,
    cells_passed: 2,
    pass_rate: 2 / 3,
  }])
})

test('LiteLLM keeps missing capabilities unknown and maps function calling to toolCalls', () => {
  const info = extractModelInfo({
    source: {
      max_input_tokens: 100_000,
      supports_function_calling: true,
    },
  }, { source: 'canonical' }).canonical

  assert.equal(info.supports.toolCalls, true)
  assert.equal(info.supports.vision, undefined)
  assert.equal(info.supports.reasoning, undefined)
  assert.equal(Object.hasOwn(info.supports, 'functionCalling'), false)
})

test('LiteLLM preserves explicit zero prices', () => {
  const info = extractModelInfo({
    source: {
      input_cost_per_token: 0,
      output_cost_per_token: 0,
      cache_read_input_token_cost: 0,
    },
  }, { source: 'canonical' }).canonical

  assert.equal(info.pricing.inputCacheMissPrice, 0)
  assert.equal(info.pricing.outputPrice, 0)
  assert.equal(info.pricing.inputCacheHitPrice, 0)
})

test('benchmark merge creates a complete valid profile', () => {
  const registry = { benchmarkProfiles: [], upstreamCapabilities: [] }
  mergeDeepsweData(registry, {
    'gpt-5.6-sol': {
      deepsweMeta: { deepsweModel: 'gpt-5-6-sol' },
      benchmarkEvidence: [{
        benchmark: 'deepswe',
        benchmarkVersion: 'v1.1',
        sourceModel: 'gpt-5-6-sol',
        domain: 'coding',
        metric: 'pass_at_1',
        rawValue: 0.8,
        uncertainty: 0.01,
        cohortPercentile: 1,
        taskCount: 100,
        cohortSize: 4,
        effort: 'high',
        selectionBasis: 'best_available_effort',
        sourceUrl: 'https://deepswe.example/',
        capturedAt: '2026-07-21',
      }],
    },
  }, emptyReport(), null)

  assert.doesNotThrow(() => validateRegistry(registry))
  assert.deepEqual(registry.benchmarkProfiles[0].sources, ['https://deepswe.example/'])
  assert.equal(registry.benchmarkProfiles[0].sharedResults, 4)
})

test('BenchLM zero comparable categories do not erase valid evidence metadata', () => {
  const registry = {
    benchmarkProfiles: [{
      patterns: ['(?:^|[-/])kimi-k2\\.7-code(?=$|@)'],
      canonicalModel: 'kimi-k2.7-code',
      benchmarkEvidence: [{
        benchmark: 'deepswe',
        benchmarkVersion: 'v1.1',
        domain: 'coding',
        sourceUrl: 'https://deepswe.example/',
        cohortSize: 16,
      }],
      sources: ['https://deepswe.example/'],
      verifiedAt: '2026-07-21',
      lane: 'provisional',
      sharedResults: 16,
      comparableCategories: 1,
      totalCategories: 1,
    }],
  }
  mergeBenchlmData(registry, {
    'kimi-k2.7-code': {
      overallScore: 55,
      categoryScores: {},
      counts: { sharedBenchmarkCount: 18, comparableCategoryCount: 0, totalCategoryCount: 8 },
      sources: ['https://benchlm.example/compare'],
    },
  }, emptyReport(), null)

  const profile = registry.benchmarkProfiles[0]
  assert.equal(profile.sharedResults, 18)
  assert.equal(profile.comparableCategories, 1)
  assert.equal(profile.totalCategories, 8)
  assert.doesNotThrow(() => validateRegistry(registry))
})

test('visualization combines DeepSWE, BenchLM and CodexRadar sources', () => {
  const evidence = (benchmark, benchmarkVersion, rawValue) => ({
    benchmark,
    benchmarkVersion,
    domain: 'coding',
    metric: 'pass_at_1',
    rawValue,
    effort: 'high',
  })
  const visualization = buildBenchmarkVisualizationData({
    modelMap: { source: 'model' },
    deepsweLeaderboard: { rows: [{
      model: 'source', reasoning_effort: 'high', pass_at_1: 0.7,
      mean_cost_usd: 2, median_cost_usd: 1.5,
    }] },
    deepsweProfiles: { model: { benchmarkEvidence: [evidence('deepswe', 'v1.1', 0.7)] } },
    benchlmProfiles: { model: { overallScore: 80, categoryScores: { coding: 75 } } },
    dradarProfiles: { model: {
      benchmarkEvidence: [evidence('codexradar', 'v1', 0.6)],
      efforts: { high: { passRate: 0.6 } },
      costData: { high: { meanCost: 1, medianCost: 0.8 } },
    } },
  })

  assert.deepEqual([...new Set(visualization.data.map(row => row.source))].sort(), ['CodexRadar', 'DeepSWE v1.1'])
  assert.deepEqual(
    [...new Set(visualization.comparisons.map(row => row.source))].sort(),
    ['BenchLM.ai', 'CodexRadar', 'DeepSWE v1.1'],
  )
  const validated = validateVisualizationData(visualization)
  const html = renderBenchmarkChart(validated.rows, validated.comparisons)
  assert.match(html, /多来源能力比较/)
  assert.match(html, /BenchLM\.ai/)
})

test('LiteLLM fills only unknown capabilities', () => {
  const registry = {
    upstreamCapabilities: [{
      patterns: ['(?:^|[-/])model(?=$|@)'],
      capabilities: { vision: true },
    }],
  }
  mergeLitellmData(registry, {
    model: { supports: { vision: false, toolCalls: true } },
  }, emptyReport(), null)

  assert.equal(registry.upstreamCapabilities[0].capabilities.vision, true)
  assert.equal(registry.upstreamCapabilities[0].capabilities.toolCalls, true)
})
