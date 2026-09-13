import React from 'react'
import { Users, UserCheck } from 'lucide-react'

export const MembersPage: React.FC = () => {
  return (
    <div className="bg-white p-8 rounded-2xl border border-slate-200 shadow-xs max-w-4xl mx-auto text-center">
      <div className="w-14 h-14 rounded-2xl bg-indigo-50 text-indigo-600 flex items-center justify-center mx-auto mb-4 border border-indigo-100">
        <Users className="w-7 h-7" />
      </div>
      <h1 className="text-xl font-bold text-slate-900 mb-2">团队成员与角色权限</h1>
      <p className="text-sm text-slate-500 max-w-md mx-auto mb-6">
        企业租户成员邀请、4 大系统角色分配与操作审计日志，将在后续阶段完善。
      </p>
      <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-slate-100 text-slate-600 text-xs font-medium">
        <UserCheck className="w-3.5 h-3.5 text-indigo-500" />
        <span>当前阶段已具备: 成员管理权限 (member:manage) 校验通过</span>
      </div>
    </div>
  )
}
