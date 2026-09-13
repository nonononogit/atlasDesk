import React from 'react'
import { Bot, Sparkles } from 'lucide-react'

export const AssistantPage: React.FC = () => {
  return (
    <div className="bg-white p-8 rounded-2xl border border-slate-200 shadow-xs max-w-4xl mx-auto text-center">
      <div className="w-14 h-14 rounded-2xl bg-sky-50 text-sky-600 flex items-center justify-center mx-auto mb-4 border border-sky-100">
        <Bot className="w-7 h-7" />
      </div>
      <h1 className="text-xl font-bold text-slate-900 mb-2">智能知识助手 (RAG)</h1>
      <p className="text-sm text-slate-500 max-w-md mx-auto mb-6">
        基于企业文档切片与混合检索的精准问答助手，将在 Phase 4 接入流式回答与精确来源引用。
      </p>
      <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-slate-100 text-slate-600 text-xs font-medium">
        <Sparkles className="w-3.5 h-3.5 text-sky-500" />
        <span>当前阶段已具备: 知识读取权限 (knowledge:read) 校验通过</span>
      </div>
    </div>
  )
}
