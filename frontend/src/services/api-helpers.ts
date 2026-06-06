import type { ActivitySegment, ChannelRecentActivity, HealthResponse, ModelsResponse } from './api-types'

export class ApiError extends Error {
  readonly status: number
  readonly details?: unknown

  constructor(message: string, status: number, details?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.details = details
  }
}

// 从环境变量读取配置
export const getApiBase = () => {
  // 在生产环境中，API调用会直接请求当前域名
  if (import.meta.env.PROD) {
    return '/api'
  }

  // 在开发环境中，支持从环境变量配置后端地址
  const backendUrl = import.meta.env.VITE_BACKEND_URL
  const apiBasePath = import.meta.env.VITE_API_BASE_PATH || '/api'

  if (backendUrl) {
    return `${backendUrl}${apiBasePath}`
  }

  // fallback到默认配置
  return '/api'
}

export const API_BASE = getApiBase()

// 打印当前API配置（仅开发环境）
if (import.meta.env.DEV) {
  console.log('🔗 API Configuration:', {
    API_BASE,
    BACKEND_URL: import.meta.env.VITE_BACKEND_URL,
    IS_DEV: import.meta.env.DEV,
    IS_PROD: import.meta.env.PROD
  })
}

// 注意：永远不在 result 中直接引用 API 的 seg 对象，避免后续复用时 reset 循环污染 API 数据
export function expandSparseSegments(activity: ChannelRecentActivity, reuse?: ActivitySegment[]): ActivitySegment[] {
  const totalSegs = activity.totalSegs || 150

  // 兼容旧版数组格式 - 直接返回 API 数组（调用方只读，安全）
  if (Array.isArray(activity.segments)) {
    return activity.segments
  }

  // 复用已有数组或创建新数组
  let result: ActivitySegment[]
  if (reuse && reuse.length === totalSegs) {
    result = reuse
  } else {
    result = new Array(totalSegs)
    for (let i = 0; i < totalSegs; i++) {
      result[i] = {
        requestCount: 0,
        successCount: 0,
        failureCount: 0,
        inputTokens: 0,
        outputTokens: 0
      }
    }
  }

  // 重置所有槽位为 0（只修改我们自己的对象，不会影响 API 数据）
  for (let i = 0; i < totalSegs; i++) {
    result[i].requestCount = 0
    result[i].successCount = 0
    result[i].failureCount = 0
    result[i].inputTokens = 0
    result[i].outputTokens = 0
  }

  // 稀疏 Map 格式：复制字段值（不替换对象引用，避免下次 reset 时污染 API）
  if (activity.segments && typeof activity.segments === 'object') {
    for (const [indexStr, seg] of Object.entries(activity.segments)) {
      const index = parseInt(indexStr, 10)
      if (index >= 0 && index < totalSegs && seg) {
        result[index].requestCount = seg.requestCount
        result[index].successCount = seg.successCount
        result[index].failureCount = seg.failureCount
        result[index].inputTokens = seg.inputTokens
        result[index].outputTokens = seg.outputTokens
      }
    }
  }

  return result
}

/**
 * 构建上游的 /v1/models 端点 URL
 * 参考：backend-go/internal/handlers/messages/models.go:240-257
 */
function buildModelsURL(baseURL: string): string {
  // 处理 # 后缀（跳过版本前缀）
  const skipVersionPrefix = baseURL.endsWith('#')
  if (skipVersionPrefix) {
    baseURL = baseURL.slice(0, -1)
  }
  baseURL = baseURL.replace(/\/$/, '')

  // 检查是否已有版本后缀（如 /v1, /v2）
  const versionPattern = /\/v\d+[a-z]*$/
  const hasVersionSuffix = versionPattern.test(baseURL)

  // 构建端点
  let endpoint = '/models'
  if (!hasVersionSuffix && !skipVersionPrefix) {
    endpoint = '/v1' + endpoint
  }

  return baseURL + endpoint
}

/**
 * 直接从上游获取模型列表（前端直连）
 */
export async function fetchUpstreamModels(
  baseUrl: string,
  apiKey: string
): Promise<ModelsResponse> {
  const url = buildModelsURL(baseUrl)

  const response = await fetch(url, {
    method: 'GET',
    headers: {
      'Authorization': `Bearer ${apiKey}`
    },
    signal: AbortSignal.timeout(10000) // 10秒超时
  })

  if (!response.ok) {
    let errorMessage = `${response.status} ${response.statusText}`
    let errorDetails: unknown = null

    try {
      const errorText = await response.text()
      if (errorText) {
        const errorJson = JSON.parse(errorText)
        // 解析上游错误格式: { "error": { "code": "", "message": "...", "type": "..." } }
        if (errorJson.error && errorJson.error.message) {
          errorMessage = errorJson.error.message
          errorDetails = errorJson.error
        } else if (errorJson.message) {
          errorMessage = errorJson.message
          errorDetails = errorJson
        }
      }
    } catch {
      // 解析失败,使用默认错误消息
    }

    throw new ApiError(errorMessage, response.status, errorDetails)
  }

  return await response.json()
}

/**
 * 获取健康检查信息（包含版本号）
 * 注意：/health 端点不需要认证，直接请求根路径
 */
export const fetchHealth = async (): Promise<HealthResponse> => {
  const baseUrl = import.meta.env.PROD ? '' : (import.meta.env.VITE_BACKEND_URL || '')
  const response = await fetch(`${baseUrl}/health`)
  if (!response.ok) {
    throw new Error(`Health check failed: ${response.status}`)
  }
  return response.json()
}
