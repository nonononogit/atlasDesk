import React from 'react'
import { Settings, Cpu } from 'lucide-react'

export const SettingsPage: React.FC = () => {
  return (
    <div className="bg-white p-8 rounded-2xl border border-slate-200 shadow-xs max-w-4xl mx-auto text-center">
      <div className="w-14 h-14 rounded-2xl bg-purple-50 text-purple-600 flex items-center justify-center mx-auto mb-4 border border-purple-100">
        <Settings className="w-7 h-7" />
      </div>
      <h1 className="text-xl font-bold text-slate-900 mb-2">系统设置与 AI 配置</h1>
      <p className="text-sm text-slate-500 max-w-md mx-auto mb-6">
        OpenAI 兼容模型接入、Embedding 参数与检索策略配置，支持版本化与 API Key 掩码安全存储。
      </p>
      <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-slate-100 text-slate-600 text-xs font-medium">
        <Cpu className="w-3.5 h-3.5 text-purple-500" />
        <span>当前阶段已具备: AI配置管理权限 (ai_config:manage) 校验通过</span>
      </div>
    </div>
  )
}
