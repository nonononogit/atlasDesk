import React, { useState, useRef } from 'react'
import { knowledgeApi, uploadToStorage } from '../../api/knowledge'
import { X, UploadCloud, FileText, CheckCircle2, AlertCircle, Loader2 } from 'lucide-react'

interface DocumentUploadModalProps {
  isOpen: boolean
  knowledgeBaseId: string
  knowledgeBaseName: string
  onClose: () => void
  onSuccess: () => void
}

export const DocumentUploadModal: React.FC<DocumentUploadModalProps> = ({
  isOpen,
  knowledgeBaseId,
  knowledgeBaseName,
  onClose,
  onSuccess,
}) => {
  const [file, setFile] = useState<File | null>(null)
  const [progress, setProgress] = useState(0)
  const [status, setStatus] = useState<'idle' | 'requesting' | 'uploading' | 'completing' | 'success' | 'error'>('idle')
  const [errorMsg, setErrorMsg] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  if (!isOpen) return null

  // 处理文件拖拽或选择
  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      const selected = e.target.files[0]
      // 客户端大小上限检查 (50MB)
      if (selected.size > 50 * 1024 * 1024) {
        setErrorMsg('文件大小超出上限（最大限制 50MB）')
        setFile(null)
        return
      }

      setErrorMsg('')
      setFile(selected)
      setStatus('idle')
      setProgress(0)
    }
  }

  // 触发预签名申请 -> 存储直传 -> 确认完成全链路
  const handleUpload = async () => {
    if (!file || !knowledgeBaseId) return

    try {
      setErrorMsg('')
      setStatus('requesting')

      // 1. 获取预签名直传凭证
      const ticket = await knowledgeApi.requestUploadTicket({
        knowledge_base_id: knowledgeBaseId,
        name: file.name,
        mime_type: file.type || 'text/plain',
        size: file.size,
      })

      // 2. 原生 XHR 直传对象存储并获取真实进度
      setStatus('uploading')
      await uploadToStorage(ticket.upload_url, file, (percent) => {
        setProgress(percent)
      })

      // 3. 确认上传完成并触发状态流转
      setStatus('completing')
      await knowledgeApi.completeUpload(ticket.document_id)

      setStatus('success')
      setTimeout(() => {
        onSuccess()
        handleClose()
      }, 1200)
    } catch (err: any) {
      setStatus('error')
      setErrorMsg(err.message || '上传过程异常中断')
    }
  }

  const handleClose = () => {
    setFile(null)
    setStatus('idle')
    setProgress(0)
    setErrorMsg('')
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="fixed inset-0 bg-slate-900/40 backdrop-blur-xs" onClick={handleClose} />
      <div className="relative w-full max-w-md bg-white rounded-2xl p-6 shadow-2xl z-10 border border-slate-100">
        <div className="flex items-center justify-between pb-4 border-b border-slate-100 mb-5">
          <div>
            <h2 className="text-base font-bold text-slate-900">上传企业文档</h2>
            <p className="text-xs text-slate-500 mt-0.5">归属于知识库：{knowledgeBaseName}</p>
          </div>
          <button onClick={handleClose} className="p-1 text-slate-400 hover:text-slate-600 rounded-lg">
            <X className="w-5 h-5" />
          </button>
        </div>

        {errorMsg && (
          <div className="mb-4 p-3 bg-rose-50 border border-rose-200 text-rose-600 text-xs rounded-lg flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{errorMsg}</span>
          </div>
        )}

        {/* 上传拖拽与选择区域 */}
        <div
          onClick={() => fileInputRef.current?.click()}
          className={`border-2 border-dashed rounded-xl p-6 text-center cursor-pointer transition-colors ${
            file ? 'border-sky-400 bg-sky-50/50' : 'border-slate-200 hover:border-sky-400 hover:bg-slate-50/60'
          }`}
        >
          <input
            ref={fileInputRef}
            type="file"
            accept=".pdf,.docx,.md,.txt"
            onChange={handleFileChange}
            className="hidden"
          />

          {file ? (
            <div className="flex flex-col items-center">
              <FileText className="w-10 h-10 text-sky-500 mb-2" />
              <div className="text-sm font-semibold text-slate-800 break-all">{file.name}</div>
              <div className="text-xs text-slate-400 mt-0.5">{(file.size / 1024 / 1024).toFixed(2)} MB</div>
            </div>
          ) : (
            <div className="flex flex-col items-center">
              <UploadCloud className="w-10 h-10 text-slate-400 mb-2" />
              <div className="text-sm font-medium text-slate-700">点击选择或拖拽文件至此处</div>
              <div className="text-xs text-slate-400 mt-1">支持格式：PDF、DOCX、Markdown、TXT (最大 50MB)</div>
            </div>
          )}
        </div>

        {/* 真实上传进度条 */}
        {(status === 'uploading' || status === 'completing' || status === 'success') && (
          <div className="mt-4 space-y-1.5">
            <div className="flex justify-between text-xs text-slate-600 font-medium">
              <span>
                {status === 'uploading' && `正在直传对象存储 (${progress}%)`}
                {status === 'completing' && '正在校验存储元数据...'}
                {status === 'success' && '上传校验完成！'}
              </span>
              <span>{progress}%</span>
            </div>
            <div className="w-full h-2 bg-slate-100 rounded-full overflow-hidden">
              <div
                className={`h-full transition-all duration-200 ${
                  status === 'success' ? 'bg-emerald-500' : 'bg-sky-500'
                }`}
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
        )}

        {/* 操作按钮 */}
        <div className="flex items-center justify-end gap-3 pt-5 mt-4 border-t border-slate-100">
          <button
            type="button"
            onClick={handleClose}
            className="px-4 py-2 text-xs font-medium text-slate-600 hover:bg-slate-100 rounded-lg"
          >
            取消
          </button>
          <button
            type="button"
            disabled={!file || status === 'requesting' || status === 'uploading' || status === 'completing'}
            onClick={handleUpload}
            className="px-5 py-2 text-xs font-medium text-white bg-sky-600 hover:bg-sky-500 rounded-lg shadow-xs flex items-center gap-1.5 disabled:opacity-50"
          >
            {(status === 'requesting' || status === 'uploading' || status === 'completing') && (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            )}
            {status === 'success' && <CheckCircle2 className="w-3.5 h-3.5" />}
            <span>
              {status === 'idle' && '开始预签名直传'}
              {status === 'requesting' && '申请直传凭据...'}
              {status === 'uploading' && '直传中...'}
              {status === 'completing' && '确认完成中...'}
              {status === 'success' && '上传成功'}
              {status === 'error' && '重新上传'}
            </span>
          </button>
        </div>
      </div>
    </div>
  )
}
