// ==============================================================================
// AtlasDesk 原生 Fetch API 封装客户端
// 实现 Access Token 内存存储、并发排队无感刷新机制与统一错误处理
// ==============================================================================

export interface ApiError {
  code: string
  message: string
  details?: Record<string, any>
}

export interface ApiResponse<T> {
  data: T
  request_id: string
}

let inMemoryAccessToken: string | null = null

// 设置或清空内存中的 Access Token
export const setAccessToken = (token: string | null) => {
  inMemoryAccessToken = token
}

// 获取当前内存中的 Access Token
export const getAccessToken = (): string | null => {
  return inMemoryAccessToken
}

// 并发排队锁状态
let isRefreshing = false
let refreshSubscribers: Array<(token: string) => void> = []

// 订阅刷新完成事件
const subscribeTokenRefresh = (cb: (token: string) => void) => {
  refreshSubscribers.push(cb)
}

// 通知所有等待请求继续执行
const onRefreshed = (token: string) => {
  refreshSubscribers.forEach((cb) => cb(token))
  refreshSubscribers = []
}

// 执行刷新请求
const executeTokenRefresh = async (): Promise<string | null> => {
  try {
    const res = await fetch('/api/v1/auth/refresh', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include', // 必须携带 HttpOnly Cookie
    })

    if (!res.ok) {
      setAccessToken(null)
      return null
    }

    const data: ApiResponse<{ access_token: string }> = await res.json()
    const newToken = data.data.access_token
    setAccessToken(newToken)
    return newToken
  } catch {
    setAccessToken(null)
    return null
  }
}

// 核心通用请求函数
export async function apiFetch<T>(
  endpoint: string,
  options: RequestInit = {},
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  }

  // 附加 Authorization 头 (纯内存中获取)
  if (inMemoryAccessToken) {
    headers['Authorization'] = `Bearer ${inMemoryAccessToken}`
  }

  const response = await fetch(endpoint, {
    ...options,
    headers,
    credentials: 'include',
  })

  // 拦截 401 未授权错误，排队发起单个 Refresh 请求
  if (response.status === 401 && !endpoint.includes('/auth/login') && !endpoint.includes('/auth/refresh')) {
    if (!isRefreshing) {
      isRefreshing = true
      const newToken = await executeTokenRefresh()
      isRefreshing = false

      if (newToken) {
        onRefreshed(newToken)
        // 重试当前请求
        headers['Authorization'] = `Bearer ${newToken}`
        const retryRes = await fetch(endpoint, {
          ...options,
          headers,
          credentials: 'include',
        })
        return handleResponse<T>(retryRes)
      } else {
        // 刷新失败，广播重置并抛出异常
        refreshSubscribers = []
        window.dispatchEvent(new Event('atlasdesk:unauthorized'))
        throw new Error('会话已过期，请重新登录')
      }
    } else {
      // 正在刷新中，挂起当前请求加入等待队列
      return new Promise<T>((resolve, reject) => {
        subscribeTokenRefresh(async (newToken) => {
          try {
            headers['Authorization'] = `Bearer ${newToken}`
            const retryRes = await fetch(endpoint, {
              ...options,
              headers,
              credentials: 'include',
            })
            resolve(await handleResponse<T>(retryRes))
          } catch (err) {
            reject(err)
          }
        })
      })
    }
  }

  return handleResponse<T>(response)
}

// 统一响应解析
async function handleResponse<T>(response: Response): Promise<T> {
  const isJson = response.headers.get('content-type')?.includes('application/json')
  const payload = isJson ? await response.json() : null

  if (!response.ok) {
    const errorMsg = payload?.error?.message || `请求失败 (${response.status})`
    const error = new Error(errorMsg) as Error & { code?: string; details?: any }
    if (payload?.error) {
      error.code = payload.error.code
      error.details = payload.error.details
    }
    throw error
  }

  // 后端标准封装格式解包
  return payload?.data !== undefined ? payload.data : payload
}

// 便捷请求动词方法
export const apiClient = {
  get: <T>(url: string, options?: RequestInit) =>
    apiFetch<T>(url, { ...options, method: 'GET' }),
  post: <T>(url: string, body?: any, options?: RequestInit) =>
    apiFetch<T>(url, {
      ...options,
      method: 'POST',
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),
  patch: <T>(url: string, body?: any, options?: RequestInit) =>
    apiFetch<T>(url, {
      ...options,
      method: 'PATCH',
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),
  delete: <T>(url: string, options?: RequestInit) =>
    apiFetch<T>(url, { ...options, method: 'DELETE' }),
}
