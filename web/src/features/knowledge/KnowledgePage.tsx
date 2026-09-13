import React, { useState, useEffect, useCallback } from 'react'
import { knowledgeApi, KnowledgeBase, DocumentItem } from '../../api/knowledge'
import { KnowledgeBaseModal } from './KnowledgeBaseModal'
import { DocumentUploadModal } from './DocumentUploadModal'
import { DocumentChunksModal } from './DocumentChunksModal'
import {
  BookOpen,
  Plus,
  Upload,
  Search,
  Trash2,
  FileText,
  Clock,
  CheckCircle2,
  AlertCircle,
  Loader2,
  Layers,
  Eye,
  Download,
  RefreshCw,
  Edit2,
} from 'lucide-react'

export const KnowledgePage: React.FC = () => {
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBase[]>([])
  const [selectedKb, setSelectedKb] = useState<KnowledgeBase | null>(null)
  const [documents, setDocuments] = useState<DocumentItem[]>([])
  const [searchQuery, setSearchQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [isLoadingKbs, setIsLoadingKbs] = useState(true)
  const [isLoadingDocs, setIsLoadingDocs] = useState(false)

  // 弹窗状态
  const [isKbModalOpen, setIsKbModalOpen] = useState(false)
  const [isUploadModalOpen, setIsUploadModalOpen] = useState(false)
  const [isChunksModalOpen, setIsChunksModalOpen] = useState(false)
  const [selectedDocForChunks, setSelectedDocForChunks] = useState<DocumentItem | null>(null)

  // 1. 加载知识库列表
  const loadKnowledgeBases = useCallback(async () => {
    try {
      setIsLoadingKbs(true)
      const data = await knowledgeApi.listKnowledgeBases()
      setKnowledgeBases(data)
      if (data.length > 0 && !selectedKb) {
        setSelectedKb(data[0])
      }
    } catch (err) {
      console.error('加载知识库失败:', err)
    } finally {
      setIsLoadingKbs(false)
    }
  }, [selectedKb])

  useEffect(() => {
    loadKnowledgeBases()
  }, [loadKnowledgeBases])

  // 2. 加载选中文档列表 (支持搜索与状态筛选)
  const loadDocuments = useCallback(async () => {
    if (!selectedKb) {
      setDocuments([])
      return
    }

    try {
      setIsLoadingDocs(true)
      const res = await knowledgeApi.listDocuments({
        knowledge_base_id: selectedKb.id,
        q: searchQuery.trim(),
        status: statusFilter,
        limit: 50,
      })
      setDocuments(res.items || [])
    } catch (err) {
      console.error('加载文档失败:', err)
    } finally {
      setIsLoadingDocs(false)
    }
  }, [selectedKb, searchQuery, statusFilter])

  useEffect(() => {
    loadDocuments()
  }, [loadDocuments])

  // 3. 规范 K-01: 存在未就绪处理中记录时，每 3 秒自动轮询；全部完成自动停止
  useEffect(() => {
    const hasProcessing = documents.some((d) =>
      ['UPLOADING', 'UPLOADED', 'PARSING', 'CHUNKING', 'EMBEDDING'].includes(d.status),
    )

    if (!hasProcessing) return

    const interval = setInterval(async () => {
      if (!selectedKb) return
      try {
        const res = await knowledgeApi.listDocuments({
          knowledge_base_id: selectedKb.id,
          q: searchQuery.trim(),
          status: statusFilter,
        })
        setDocuments(res.items || [])
      } catch (err) {
        console.error('轮询文档状态失败:', err)
      }
    }, 3000)

    return () => clearInterval(interval)
  }, [documents, selectedKb, searchQuery, statusFilter])

  // 删除知识库
  const handleDeleteKb = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    if (!window.confirm('确认软删除该知识库？删除后关联文档将无法进行问答检索。')) return
    try {
      await knowledgeApi.deleteKnowledgeBase(id)
      if (selectedKb?.id === id) {
        setSelectedKb(null)
      }
      loadKnowledgeBases()
    } catch (err: any) {
      alert(err.message || '删除知识库失败')
    }
  }

  // 删除文档
  const handleDeleteDoc = async (id: string) => {
    if (!window.confirm('确认删除该文档？')) return
    try {
      await knowledgeApi.deleteDocument(id)
      loadDocuments()
    } catch (err: any) {
      alert(err.message || '删除文档失败')
    }
  }

  // 重新触发处理流水线
  const handleReprocessDoc = async (id: string) => {
    try {
      await knowledgeApi.reprocessDocument(id)
      loadDocuments()
    } catch (err: any) {
      alert(err.message || '重新处理失败')
    }
  }

  // 获取下载链接并下载
  const handleDownloadDoc = async (id: string) => {
    try {
      const res = await knowledgeApi.getDownloadUrl(id)
      if (res.download_url) {
        window.open(res.download_url, '_blank')
      }
    } catch (err: any) {
      alert(err.message || '获取下载链接失败')
    }
  }

  // 重命名文档
  const handleRenameDoc = async (doc: DocumentItem) => {
    const newName = window.prompt('请输入新的文档名称:', doc.name)
    if (!newName || newName.trim() === '' || newName === doc.name) return
    try {
      await knowledgeApi.renameDocument(doc.id, newName.trim())
      loadDocuments()
    } catch (err: any) {
      alert(err.message || '重命名文档失败')
    }
  }

  // 打开切片查看弹窗
  const handleOpenChunks = (doc: DocumentItem) => {
    setSelectedDocForChunks(doc)
    setIsChunksModalOpen(true)
  }

  // 文档状态徽章格式化 (符合规格 5.1 状态流转: UPLOADING -> UPLOADED -> PARSING -> CHUNKING -> EMBEDDING -> READY / FAILED)
  const renderStatusBadge = (doc: DocumentItem) => {
    switch (doc.status) {
      case 'READY':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-emerald-50 text-emerald-700 border border-emerald-200">
            <CheckCircle2 className="w-3 h-3" />
            <span>已就绪</span>
          </span>
        )
      case 'FAILED':
        return (
          <div className="flex flex-col items-start gap-1" title={doc.error_message || doc.error_code}>
            <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-rose-50 text-rose-700 border border-rose-200">
              <AlertCircle className="w-3 h-3" />
              <span>{doc.error_code || '处理失败'}</span>
            </span>
            {doc.error_message && (
              <span className="text-[10px] text-rose-600 max-w-[160px] truncate">
                {doc.error_message}
              </span>
            )}
          </div>
        )
      case 'UPLOADED':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-sky-50 text-sky-700 border border-sky-200">
            <Clock className="w-3 h-3" />
            <span>已上传待解析</span>
          </span>
        )
      case 'PARSING':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-50 text-blue-700 border border-blue-200">
            <Loader2 className="w-3 h-3 animate-spin" />
            <span>解析中 (20%)</span>
          </span>
        )
      case 'CHUNKING':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium bg-purple-50 text-purple-700 border border-purple-200">
            <Loader2 className="w-3 h-3 animate-spin" />
            <span>切片中 (50%)</span>
          </span>
        )
      case 'EMBEDDING':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium bg-indigo-50 text-indigo-700 border border-indigo-200">
            <Loader2 className="w-3 h-3 animate-spin" />
            <span>向量化 (80%)</span>
          </span>
        )
      default:
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium bg-amber-50 text-amber-700 border border-amber-200">
            <Loader2 className="w-3 h-3 animate-spin" />
            <span>处理中 ({doc.progress}%)</span>
          </span>
        )
    }
  }

  return (
    <div className="space-y-6">
      {/* 顶部标题与新建按钮 */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight flex items-center gap-2">
            <BookOpen className="w-5 h-5 text-sky-500" />
            <span>企业知识库管理</span>
          </h1>
          <p className="text-sm text-slate-500 mt-0.5">多租户隔离知识库集合与预签名安全直传文档管理</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setIsKbModalOpen(true)}
            className="px-3.5 py-2 text-xs font-medium bg-white text-slate-700 border border-slate-200 rounded-lg hover:bg-slate-50 shadow-xs flex items-center gap-1.5 transition-colors"
          >
            <Plus className="w-4 h-4 text-slate-500" />
            <span>新建知识库</span>
          </button>
          {selectedKb && (
            <button
              onClick={() => setIsUploadModalOpen(true)}
              className="px-4 py-2 text-xs font-medium bg-sky-600 hover:bg-sky-500 text-white rounded-lg shadow-xs flex items-center gap-1.5 transition-colors"
            >
              <Upload className="w-4 h-4" />
              <span>上传文档</span>
            </button>
          )}
        </div>
      </div>

      {/* 知识库集合卡片栏 */}
      {isLoadingKbs ? (
        <div className="p-8 text-center text-xs text-slate-400 bg-white rounded-xl border border-slate-200">
          <Loader2 className="w-5 h-5 animate-spin mx-auto mb-2 text-sky-500" />
          <span>正在加载知识库集合...</span>
        </div>
      ) : knowledgeBases.length === 0 ? (
        <div className="p-8 text-center bg-white rounded-xl border border-dashed border-slate-200">
          <Layers className="w-8 h-8 text-slate-300 mx-auto mb-2" />
          <div className="text-sm font-semibold text-slate-700">暂无知识库</div>
          <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto mb-4">
            创建您的第一个知识库，以便归类存储企业技术白皮书、客服知识及规章制度。
          </p>
          <button
            onClick={() => setIsKbModalOpen(true)}
            className="px-3.5 py-1.5 text-xs font-medium bg-sky-600 text-white rounded-lg shadow-xs"
          >
            立即创建
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {knowledgeBases.map((kb) => {
            const isSelected = selectedKb?.id === kb.id
            return (
              <div
                key={kb.id}
                onClick={() => setSelectedKb(kb)}
                className={`p-4 rounded-xl border cursor-pointer transition-all ${
                  isSelected
                    ? 'bg-sky-50/60 border-sky-300 shadow-xs ring-1 ring-sky-300'
                    : 'bg-white border-slate-200 hover:border-slate-300'
                }`}
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-2">
                    <span className="w-2 h-2 rounded-full bg-sky-500"></span>
                    <h3 className="text-sm font-bold text-slate-800 line-clamp-1">{kb.name}</h3>
                  </div>
                  <button
                    onClick={(e) => handleDeleteKb(kb.id, e)}
                    title="删除知识库"
                    className="text-slate-400 hover:text-rose-500 p-1 rounded"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
                <p className="text-xs text-slate-500 mt-1.5 line-clamp-2 min-h-[32px]">
                  {kb.description || '暂无描述信息'}
                </p>
                <div className="mt-3 flex items-center justify-between text-[11px] text-slate-400">
                  <span className="px-2 py-0.5 rounded bg-slate-100 text-slate-600 font-mono">
                    {kb.visibility === 'public' ? '全员公开' : kb.visibility === 'team' ? '团队可见' : '仅管理员'}
                  </span>
                  <span>{new Date(kb.created_at).toLocaleDateString()}</span>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* 选中文档区域 */}
      {selectedKb && (
        <div className="bg-white rounded-xl border border-slate-200 shadow-xs overflow-hidden">
          {/* 文档筛选工具栏 */}
          <div className="p-4 border-b border-slate-200 flex flex-col sm:flex-row items-center justify-between gap-3">
            <div className="flex items-center gap-2 w-full sm:w-auto">
              <div className="relative flex-1 sm:w-64">
                <Search className="w-4 h-4 absolute left-3 top-2.5 text-slate-400" />
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="搜索文档名称..."
                  className="w-full pl-9 pr-3 py-1.5 text-xs border border-slate-200 rounded-lg focus:outline-none focus:border-sky-500"
                />
              </div>

              <select
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                className="px-2.5 py-1.5 text-xs border border-slate-200 rounded-lg bg-white text-slate-600 focus:outline-none focus:border-sky-500"
              >
                <option value="">全部状态</option>
                <option value="UPLOADING">上传中</option>
                <option value="UPLOADED">已上传</option>
                <option value="READY">已就绪</option>
                <option value="FAILED">失败</option>
              </select>
            </div>

            <div className="text-xs text-slate-400">
              共 <span className="font-semibold text-slate-700">{documents.length}</span> 篇文档
            </div>
          </div>

          {/* 文档表格列表 */}
          {isLoadingDocs ? (
            <div className="p-12 text-center text-xs text-slate-400">
              <Loader2 className="w-5 h-5 animate-spin mx-auto mb-2 text-sky-500" />
              <span>正在加载文档列表...</span>
            </div>
          ) : documents.length === 0 ? (
            <div className="p-12 text-center text-slate-400">
              <FileText className="w-8 h-8 mx-auto mb-2 text-slate-300" />
              <div className="text-sm font-medium text-slate-600">当前知识库暂无文档</div>
              <p className="text-xs text-slate-400 mt-1 mb-4">点击上方“上传文档”按钮直传企业技术资料。</p>
              <button
                onClick={() => setIsUploadModalOpen(true)}
                className="px-3.5 py-1.5 text-xs font-medium bg-sky-600 text-white rounded-lg shadow-xs"
              >
                立即上传
              </button>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-slate-50 text-slate-500 border-b border-slate-200 uppercase tracking-wider font-semibold">
                  <tr>
                    <th className="py-3 px-4">文档名称</th>
                    <th className="py-3 px-4">类型</th>
                    <th className="py-3 px-4">大小</th>
                    <th className="py-3 px-4">处理状态</th>
                    <th className="py-3 px-4">上传时间</th>
                    <th className="py-3 px-4 text-right">操作</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100">
                  {documents.map((doc) => (
                    <tr key={doc.id} className="hover:bg-slate-50/70 transition-colors">
                      <td className="py-3 px-4 font-medium text-slate-800 flex items-center gap-2">
                        <FileText className="w-4 h-4 text-sky-500 shrink-0" />
                        <span className="line-clamp-1">{doc.name}</span>
                      </td>
                      <td className="py-3 px-4 text-slate-500 font-mono text-[11px]">{doc.mime_type}</td>
                      <td className="py-3 px-4 text-slate-500">{(doc.size / 1024).toFixed(1)} KB</td>
                      <td className="py-3 px-4">{renderStatusBadge(doc)}</td>
                      <td className="py-3 px-4 text-slate-400">{new Date(doc.created_at).toLocaleString()}</td>
                      <td className="py-3 px-4 text-right">
                        <div className="flex items-center justify-end gap-1">
                          {/* 查看切片详情按钮 */}
                          <button
                            onClick={() => handleOpenChunks(doc)}
                            className="p-1.5 text-slate-400 hover:text-indigo-600 hover:bg-indigo-50 rounded transition-colors"
                            title="查看切片详情与向量"
                          >
                            <Eye className="w-3.5 h-3.5" />
                          </button>
                          {/* 原文件下载 */}
                          <button
                            onClick={() => handleDownloadDoc(doc.id)}
                            className="p-1.5 text-slate-400 hover:text-sky-600 hover:bg-sky-50 rounded transition-colors"
                            title="下载原文件"
                          >
                            <Download className="w-3.5 h-3.5" />
                          </button>
                          {/* 重新处理 */}
                          <button
                            onClick={() => handleReprocessDoc(doc.id)}
                            className="p-1.5 text-slate-400 hover:text-amber-600 hover:bg-amber-50 rounded transition-colors"
                            title="重新触发解析与切片"
                          >
                            <RefreshCw className="w-3.5 h-3.5" />
                          </button>
                          {/* 重命名 */}
                          <button
                            onClick={() => handleRenameDoc(doc)}
                            className="p-1.5 text-slate-400 hover:text-slate-700 hover:bg-slate-100 rounded transition-colors"
                            title="重命名文档"
                          >
                            <Edit2 className="w-3.5 h-3.5" />
                          </button>
                          {/* 删除 */}
                          <button
                            onClick={() => handleDeleteDoc(doc.id)}
                            className="p-1.5 text-slate-400 hover:text-rose-600 hover:bg-rose-50 rounded transition-colors"
                            title="删除文档"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* 新建知识库弹窗 */}
      <KnowledgeBaseModal
        isOpen={isKbModalOpen}
        onClose={() => setIsKbModalOpen(false)}
        onSuccess={loadKnowledgeBases}
      />

      {/* 文档上传弹窗 */}
      {selectedKb && (
        <DocumentUploadModal
          isOpen={isUploadModalOpen}
          knowledgeBaseId={selectedKb.id}
          knowledgeBaseName={selectedKb.name}
          onClose={() => setIsUploadModalOpen(false)}
          onSuccess={loadDocuments}
        />
      )}

      {/* 切片查看弹窗 */}
      <DocumentChunksModal
        isOpen={isChunksModalOpen}
        document={selectedDocForChunks}
        onClose={() => {
          setIsChunksModalOpen(false)
          setSelectedDocForChunks(null)
        }}
      />
    </div>
  )
}
