import React, { useState } from 'react'
import { knowledgeApi } from '../../api/knowledge'
import { X, BookPlus, Loader2 } from 'lucide-react'

interface KnowledgeBaseModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

export const KnowledgeBaseModal: React.FC<KnowledgeBaseModalProps> = ({ isOpen, onClose, onSuccess }) => {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [visibility, setVisibility] = useState<'private' | 'team' | 'public'>('team')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  if (!isOpen) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) {
      setErrorMsg('知识库名称不能为空')
      return
    }

    try {
      setIsSubmitting(true)
      setErrorMsg('')
      await knowledgeApi.createKnowledgeBase({
        name: name.trim(),
        description: description.trim(),
        visibility,
      })
      onSuccess()
      onClose()
      setName('')
      setDescription('')
    } catch (err: any) {
      setErrorMsg(err.message || '创建知识库失败')
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="fixed inset-0 bg-slate-900/40 backdrop-blur-xs" onClick={onClose} />
      <div className="relative w-full max-w-md bg-white rounded-2xl p-6 shadow-2xl z-10 border border-slate-100">
        <div className="flex items-center justify-between pb-4 border-b border-slate-100 mb-5">
          <div className="flex items-center gap-2 text-slate-900 font-bold">
            <BookPlus className="w-5 h-5 text-sky-500" />
            <span>新建知识库</span>
          </div>
          <button onClick={onClose} className="p-1 text-slate-400 hover:text-slate-600 rounded-lg">
            <X className="w-5 h-5" />
          </button>
        </div>

        {errorMsg && (
          <div className="mb-4 p-3 bg-rose-50 border border-rose-200 text-rose-600 text-xs rounded-lg">
            {errorMsg}
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-slate-700 mb-1">
              知识库名称 <span className="text-rose-500">*</span>
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="例如：产品技术白皮书"
              required
              maxLength={100}
              className="w-full px-3.5 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:border-sky-500 focus:ring-1 focus:ring-sky-500"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-700 mb-1">描述信息</label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="简要说明此知识库所包含的文档范围与适用人员..."
              rows={3}
              maxLength={500}
              className="w-full px-3.5 py-2 text-sm border border-slate-200 rounded-lg focus:outline-none focus:border-sky-500 focus:ring-1 focus:ring-sky-500"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-700 mb-1">可见范围</label>
            <div className="grid grid-cols-3 gap-2">
              {[
                { key: 'team', label: '团队可见' },
                { key: 'public', label: '全员公开' },
                { key: 'private', label: '仅管理员' },
              ].map((item) => (
                <button
                  type="button"
                  key={item.key}
                  onClick={() => setVisibility(item.key as any)}
                  className={`py-2 text-xs font-medium rounded-lg border text-center transition-colors ${
                    visibility === item.key
                      ? 'border-sky-500 bg-sky-50 text-sky-700 font-semibold'
                      : 'border-slate-200 text-slate-600 hover:bg-slate-50'
                  }`}
                >
                  {item.label}
                </button>
              ))}
            </div>
          </div>

          <div className="flex items-center justify-end gap-3 pt-4 border-t border-slate-100">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-xs font-medium text-slate-600 hover:bg-slate-100 rounded-lg"
            >
              取消
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="px-5 py-2 text-xs font-medium text-white bg-sky-600 hover:bg-sky-500 rounded-lg shadow-xs flex items-center gap-1.5 disabled:opacity-50"
            >
              {isSubmitting && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
              <span>确认创建</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
