// ==============================================================================
// AtlasDesk RAG 智能助手 API 客户端
// 封装会话 CRUD、原生 fetch SSE 流式问答解析器与回答反馈
// ==============================================================================

import { apiClient, getAccessToken } from './client'

export interface ConversationItem {
  id: string
  organization_id: string
  user_id: string
  title: string
  created_at: string
  updated_at: string
}

export interface MessageCitationItem {
  id: string
  message_id: string
  chunk_id?: string
  citation_index: number
  quote: string
  document_title: string
  page_number: number
  created_at: string
}

export interface MessageFeedbackItem {
  id: string
  message_id: string
  user_id: string
  rating: number // 1 或 -1
  reason?: string
}

export interface MessageItem {
  id: string
  conversation_id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  status: 'sending' | 'sent' | 'cancelled' | 'failed'
  model?: string
  usage?: {
    prompt_tokens: number
    completion_tokens: number
    total_tokens: number
  }
  created_at: string
  citations?: MessageCitationItem[]
  feedback?: MessageFeedbackItem
}

export interface StreamCallbacks {
  onRetrievalStarted?: (data: { query: string; message_id: string }) => void
  onAnswerDelta?: (delta: string) => void
  onCitations?: (citations: MessageCitationItem[]) => void
  onDone?: (data: { message_id: string; usage: any }) => void
  onError?: (error: { message: string }) => void
}

export const assistantApi = {
  // 1. 获取用户会话列表 (游标分页)
  listConversations: (cursor?: string, limit: number = 20) => {
    const params = new URLSearchParams()
    if (cursor) params.set('cursor', cursor)
    params.set('limit', String(limit))
    return apiClient.get<{ items: ConversationItem[]; next_cursor?: string; has_more: boolean }>(
      `/api/v1/conversations?${params.toString()}`,
    )
  },

  // 2. 创建新会话
  createConversation: (title?: string) =>
    apiClient.post<ConversationItem>('/api/v1/conversations', { title }),

  // 3. 删除会话
  deleteConversation: (id: string) =>
    apiClient.delete(`/api/v1/conversations/${id}`),

  // 4. 获取会话内消息列表
  listMessages: (conversationId: string) =>
    apiClient.get<MessageItem[]>(`/api/v1/conversations/${conversationId}/messages`),

  // 5. 获取单条引用溯源详情
  getCitation: (citationId: string) =>
    apiClient.get<MessageCitationItem>(`/api/v1/citations/${citationId}`),

  // 6. 提交回答满意度反馈
  submitFeedback: (messageId: string, rating: number, reason?: string) =>
    apiClient.post(`/api/v1/messages/${messageId}/feedback`, { rating, reason }),
}

// 基于原生 fetch 和 ReadableStream 的 SSE 流式读取器 (符合规范 A-03 要求)
export async function streamMessage(
  conversationId: string,
  content: string,
  knowledgeBaseIds: string[] | undefined,
  callbacks: StreamCallbacks,
  signal?: AbortSignal,
): Promise<void> {
  const token = getAccessToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const endpoint = `/api/v1/conversations/${conversationId}/messages/stream`
  const response = await fetch(endpoint, {
    method: 'POST',
    headers,
    body: JSON.stringify({
      content,
      knowledge_base_ids: knowledgeBaseIds,
    }),
    credentials: 'include',
    signal,
  })

  if (!response.ok) {
    let errorMsg = `流式连接失败 (${response.status})`
    try {
      const errJson = await response.json()
      if (errJson?.error?.message) {
        errorMsg = errJson.error.message
      }
    } catch {
      // 忽略解析异常
    }
    callbacks.onError?.({ message: errorMsg })
    return
  }

  if (!response.body) {
    callbacks.onError?.({ message: '响应体为空' })
    return
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder('utf-8')
  let buffer = ''

  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      let currentEvent = 'message'
      for (const line of lines) {
        const trimmed = line.trim()
        if (!trimmed) continue

        if (trimmed.startsWith('event:')) {
          currentEvent = trimmed.replace(/^event:\s*/, '')
        } else if (trimmed.startsWith('data:')) {
          const rawData = trimmed.replace(/^data:\s*/, '')
          try {
            const parsed = JSON.parse(rawData)
            switch (currentEvent) {
              case 'retrieval_started':
                callbacks.onRetrievalStarted?.(parsed)
                break
              case 'answer_delta':
                callbacks.onAnswerDelta?.(parsed.delta || '')
                break
              case 'citations':
                callbacks.onCitations?.(parsed)
                break
              case 'done':
                callbacks.onDone?.(parsed)
                break
              case 'error':
                callbacks.onError?.(parsed)
                break
              default:
                break
            }
          } catch {
            // 纯文本数据直接触发 delta
            if (currentEvent === 'answer_delta') {
              callbacks.onAnswerDelta?.(rawData)
            }
          }
        }
      }
    }
  } catch (err: any) {
    if (err.name === 'AbortError') {
      // 用户主动取消
      return
    }
    callbacks.onError?.({ message: err.message || '流式传输中断' })
  }
}
