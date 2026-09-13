// ==============================================================================
// AtlasDesk 知识库与文档 API 客户端
// 封装知识库 CRUD、文档游标列表与 XHR 直传对象存储进度监听
// ==============================================================================

import { apiClient } from './client'

export interface KnowledgeBase {
  id: string
  organization_id: string
  name: string
  description: string
  visibility: 'public' | 'team' | 'private'
  created_at: string
  updated_at: string
}

export interface DocumentItem {
  id: string
  organization_id: string
  knowledge_base_id: string
  name: string
  mime_type: string
  size: number
  status: 'UPLOADING' | 'UPLOADED' | 'PARSING' | 'CHUNKING' | 'EMBEDDING' | 'READY' | 'FAILED'
  progress: number
  error_code?: string
  error_message?: string
  created_at: string
  updated_at: string
}

export interface CursorPage<T> {
  items: T[]
  next_cursor?: string
  has_more: boolean
}

export interface UploadTicket {
  document_id: string
  upload_url: string
  object_key: string
  expires_in: number
}

// 知识库 API 接口
export const knowledgeApi = {
  // 1. 获取知识库列表
  listKnowledgeBases: () =>
    apiClient.get<KnowledgeBase[]>('/api/v1/knowledge-bases'),

  // 2. 创建新知识库
  createKnowledgeBase: (data: { name: string; description?: string; visibility?: string }) =>
    apiClient.post<KnowledgeBase>('/api/v1/knowledge-bases', data),

  // 3. 删除知识库
  deleteKnowledgeBase: (id: string) =>
    apiClient.delete(`/api/v1/knowledge-bases/${id}`),

  // 4. 获取文档列表 (支持筛选与游标)
  listDocuments: (params: {
    knowledge_base_id?: string
    q?: string
    type?: string
    status?: string
    cursor?: string
    limit?: number
  }) => {
    const searchParams = new URLSearchParams()
    if (params.knowledge_base_id) searchParams.set('knowledge_base_id', params.knowledge_base_id)
    if (params.q) searchParams.set('q', params.q)
    if (params.type) searchParams.set('type', params.type)
    if (params.status) searchParams.set('status', params.status)
    if (params.cursor) searchParams.set('cursor', params.cursor)
    if (params.limit) searchParams.set('limit', String(params.limit))
    return apiClient.get<CursorPage<DocumentItem>>(`/api/v1/documents?${searchParams.toString()}`)
  },

  // 5. 申请预签名直传凭据
  requestUploadTicket: (data: {
    knowledge_base_id: string
    name: string
    mime_type: string
    size: number
  }) => apiClient.post<UploadTicket>('/api/v1/documents/uploads', data),

  // 6. 确认上传完成
  completeUpload: (documentId: string, checksum?: string) =>
    apiClient.post<DocumentItem>(`/api/v1/documents/${documentId}/complete-upload`, { checksum }),

  // 7. 查询单个文档处理状态 (用于轮询)
  getDocumentStatus: (documentId: string) =>
    apiClient.get<{ id: string; status: string; progress: number; error_code?: string; error_message?: string }>(
      `/api/v1/documents/${documentId}/status`,
    ),

  // 8. 软删除文档
  deleteDocument: (documentId: string) =>
    apiClient.delete(`/api/v1/documents/${documentId}`),
}

// 基于原生 XHR 直传对象存储并获取真实百分比进度
export function uploadToStorage(
  uploadUrl: string,
  file: File,
  onProgress?: (percent: number) => void,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', uploadUrl, true)
    xhr.setRequestHeader('Content-Type', file.type || 'application/octet-stream')

    if (xhr.upload && onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          const percent = Math.round((e.loaded / e.total) * 100)
          onProgress(percent)
        }
      }
    }

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve()
      } else {
        reject(new Error(`直传存储失败 (HTTP ${xhr.status})`))
      }
    }

    xhr.onerror = () => reject(new Error('网络中断，上传失败'))
    xhr.onabort = () => reject(new Error('上传已被用户取消'))

    xhr.send(file)
  })
}
