import React, { useState, useEffect } from 'react'
import { knowledgeApi, DocumentItem, DocumentChunk } from '../../api/knowledge'
import { X, Layers, Search, Loader2, Hash, Bookmark, BookOpen } from 'lucide-react'

interface DocumentChunksModalProps {
  isOpen: boolean
  document: DocumentItem | null
  onClose: () => void
}

export const DocumentChunksModal: React.FC<DocumentChunksModalProps> = ({
  isOpen,
  document,
  onClose,
}) => {
  const [chunks, setChunks] = useState<DocumentChunk[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [searchFilter, setSearchFilter] = useState('')

  useEffect(() => {
    if (!isOpen || !document) {
      setChunks([])
      return
    }

    const fetchChunks = async () => {
      try {
        setIsLoading(true)
        const data = await knowledgeApi.getDocumentChunks(document.id)
        setChunks(data || [])
      } catch (err) {
        console.error('获取切片详情失败:', err)
      } finally {
        setIsLoading(false)
      }
    }

    fetchChunks()
  }, [isOpen, document])

  if (!isOpen || !document) return null

  const filteredChunks = chunks.filter((c) =>
    c.content.toLowerCase().includes(searchFilter.toLowerCase()) ||
    (c.metadata?.heading_path && c.metadata.heading_path.toLowerCase().includes(searchFilter.toLowerCase())),
  )

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm animate-fade-in">
      <div className="w-full max-w-4xl bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[85vh]">
        {/* 弹窗顶部栏 */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-100 dark:border-slate-800">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-indigo-50 dark:bg-indigo-950/40 text-indigo-600 dark:text-indigo-400 flex items-center justify-center font-bold">
              <Layers className="w-5 h-5" />
            </div>
            <div>
              <h3 className="font-semibold text-slate-900 dark:text-white flex items-center gap-2">
                <span>{document.name}</span>
                <span className="text-xs px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 text-slate-500 font-normal">
                  共 {chunks.length} 个切片
                </span>
              </h3>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                查看文档经过语义分析、层级切片与 1536 维向量化存储的分块内容
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* 筛选与搜索条 */}
        <div className="px-6 py-3 border-b border-slate-100 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50 flex items-center gap-3">
          <div className="relative flex-1">
            <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              placeholder="搜索切片内容或章节标题..."
              value={searchFilter}
              onChange={(e) => setSearchFilter(e.target.value)}
              className="w-full pl-9 pr-4 py-1.5 text-sm bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 transition-all text-slate-900 dark:text-white placeholder-slate-400"
            />
          </div>
        </div>

        {/* 切片列表内容区 */}
        <div className="flex-1 overflow-y-auto p-6 space-y-4">
          {isLoading ? (
            <div className="py-20 flex flex-col items-center justify-center text-slate-400">
              <Loader2 className="w-8 h-8 animate-spin text-indigo-500 mb-2" />
              <p className="text-sm">正在加载切片与向量元数据...</p>
            </div>
          ) : filteredChunks.length === 0 ? (
            <div className="py-16 text-center text-slate-400">
              <BookOpen className="w-12 h-12 mx-auto mb-3 opacity-40 text-slate-400" />
              <p className="text-sm font-medium text-slate-600 dark:text-slate-300">
                {chunks.length === 0 ? '暂无切片数据 (文档可能未就绪或未生成分块)' : '未找到匹配的切片内容'}
              </p>
            </div>
          ) : (
            filteredChunks.map((chunk) => (
              <div
                key={chunk.id}
                className="p-4 rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-800/60 hover:border-indigo-200 dark:hover:border-indigo-800/50 transition-all space-y-2.5"
              >
                {/* 顶部元数据徽标 */}
                <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500 dark:text-slate-400 border-b border-slate-100 dark:border-slate-700/60 pb-2">
                  <div className="flex items-center gap-3">
                    <span className="flex items-center gap-1 font-mono font-semibold text-indigo-600 dark:text-indigo-400 bg-indigo-50 dark:bg-indigo-950/60 px-2 py-0.5 rounded">
                      <Hash className="w-3.5 h-3.5" />
                      Chunk #{chunk.chunk_index}
                    </span>
                    {chunk.metadata?.heading_path && (
                      <span className="flex items-center gap-1 text-slate-700 dark:text-slate-300 bg-slate-100 dark:bg-slate-700 px-2 py-0.5 rounded max-w-[280px] truncate" title={chunk.metadata.heading_path}>
                        <Bookmark className="w-3 h-3 text-indigo-500 shrink-0" />
                        {chunk.metadata.heading_path}
                      </span>
                    )}
                    {chunk.metadata?.page_number && chunk.metadata.page_number > 0 && (
                      <span className="bg-slate-100 dark:bg-slate-700 px-2 py-0.5 rounded">
                        第 {chunk.metadata.page_number} 页
                      </span>
                    )}
                  </div>
                  <div className="flex items-center gap-2 font-mono text-[11px]">
                    <span>{chunk.token_count} Tokens</span>
                    <span className="text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-950/50 px-1.5 py-0.5 rounded font-sans">
                      1536 维已向量化
                    </span>
                  </div>
                </div>

                {/* 切片正文展示 */}
                <p className="text-sm text-slate-700 dark:text-slate-200 whitespace-pre-wrap leading-relaxed font-sans select-text">
                  {chunk.content}
                </p>
              </div>
            ))
          )}
        </div>

        {/* 弹窗底部 */}
        <div className="px-6 py-3 border-t border-slate-100 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/50 flex justify-end">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-lg transition-colors"
          >
            关闭
          </button>
        </div>
      </div>
    </div>
  )
}
