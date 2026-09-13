import React from 'react'
import { BookOpen, UploadCloud } from 'lucide-react'

export const KnowledgePage: React.FC = () => {
  return (
    <div className="bg-white p-8 rounded-2xl border border-slate-200 shadow-xs max-w-4xl mx-auto text-center">
      <div className="w-14 h-14 rounded-2xl bg-emerald-50 text-emerald-600 flex items-center justify-center mx-auto mb-4 border border-emerald-100">
        <BookOpen className="w-7 h-7" />
      </div>
      <h1 className="text-xl font-bold text-slate-900 mb-2">企业知识库与文档管理</h1>
      <p className="text-sm text-slate-500 max-w-md mx-auto mb-6">
        企业知识库集合与预签名直传对象存储管理，将在 Phase 2 接入多格式文档上传与解析状态机。
      </p>
      <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-slate-100 text-slate-600 text-xs font-medium">
        <UploadCloud className="w-3.5 h-3.5 text-emerald-500" />
        <span>当前阶段已具备: 知识库读写权限 (knowledge:read/write) 校验通过</span>
      </div>
    </div>
  )
}
