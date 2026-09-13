import React from 'react'
import { useAuthStore } from '../../stores/authStore'
import { Activity, Users, Ticket, BookOpen, ShieldCheck } from 'lucide-react'

export const DashboardPage: React.FC = () => {
  const user = useAuthStore((state) => state.user)

  return (
    <div className="space-y-6">
      {/* 欢迎看板 */}
      <div className="bg-white p-6 rounded-2xl border border-slate-200 shadow-xs flex flex-col sm:flex-row justify-between sm:items-center gap-4">
        <div>
          <h1 className="text-xl font-bold text-slate-900 tracking-tight flex items-center gap-2">
            <span>你好，{user?.name || '管理员'}</span>
            <span className="text-xs px-2.5 py-0.5 rounded-full bg-emerald-50 text-emerald-700 border border-emerald-200 font-medium">
              在线
            </span>
          </h1>
          <p className="text-sm text-slate-500 mt-1">
            当前所属组织：<span className="font-medium text-slate-700">{user?.organization_name}</span> · 当前角色：<span className="font-medium text-slate-700">{user?.role_name}</span>
          </p>
        </div>
        <div className="flex items-center gap-2 text-xs text-sky-700 bg-sky-50 px-3 py-1.5 rounded-lg border border-sky-200">
          <ShieldCheck className="w-4 h-4 text-sky-500" />
          <span>RBAC 权限已就绪 ({user?.permissions.length} 项可用)</span>
        </div>
      </div>

      {/* 4 个标准核心指标卡 (符合规格 7.2 口径) */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="bg-white p-5 rounded-xl border border-slate-200 shadow-xs">
          <div className="flex items-center justify-between text-slate-400 mb-2">
            <span className="text-xs font-medium text-slate-500">今日问答量</span>
            <Activity className="w-4 h-4 text-sky-500" />
          </div>
          <div className="text-2xl font-bold text-slate-900">0</div>
          <div className="text-[11px] text-slate-400 mt-1">与昨日相同时段持平</div>
        </div>

        <div className="bg-white p-5 rounded-xl border border-slate-200 shadow-xs">
          <div className="flex items-center justify-between text-slate-400 mb-2">
            <span className="text-xs font-medium text-slate-500">AI 自助解决率</span>
            <BookOpen className="w-4 h-4 text-emerald-500" />
          </div>
          <div className="text-2xl font-bold text-slate-900">0%</div>
          <div className="text-[11px] text-slate-400 mt-1">正向反馈已结束会话占比</div>
        </div>

        <div className="bg-white p-5 rounded-xl border border-slate-200 shadow-xs">
          <div className="flex items-center justify-between text-slate-400 mb-2">
            <span className="text-xs font-medium text-slate-500">待处理工单</span>
            <Ticket className="w-4 h-4 text-amber-500" />
          </div>
          <div className="text-2xl font-bold text-slate-900">0</div>
          <div className="text-[11px] text-slate-400 mt-1">包含新建与流转中工单</div>
        </div>

        <div className="bg-white p-5 rounded-xl border border-slate-200 shadow-xs">
          <div className="flex items-center justify-between text-slate-400 mb-2">
            <span className="text-xs font-medium text-slate-500">企业成员数</span>
            <Users className="w-4 h-4 text-indigo-500" />
          </div>
          <div className="text-2xl font-bold text-slate-900">1</div>
          <div className="text-[11px] text-slate-400 mt-1">已激活 1 位超级管理员</div>
        </div>
      </div>

      {/* Phase 1 成果与系统状态说明 */}
      <div className="bg-white p-6 rounded-xl border border-slate-200 shadow-xs">
        <h2 className="text-sm font-semibold text-slate-900 mb-2">系统当前已就绪能力</h2>
        <div className="space-y-2 text-xs text-slate-600">
          <div className="flex items-center gap-2">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
            <span><strong>认证鉴权</strong>: 支持短时 JWT Access Token 内存防泄漏 + HttpOnly Cookie Refresh Token 自动轮换与注销。</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
            <span><strong>多租户与 RBAC</strong>: 数据库实体已通过显式迁移完成，支持组织隔离与 4 系统角色细粒度权限过滤。</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
            <span><strong>全链路中间件</strong>: 包含 Request ID 透传、Recovery 异常捕获、CORS、访问日志、限流与组织防越权保护。</span>
          </div>
        </div>
      </div>
    </div>
  )
}
