import React from 'react'
import { Ticket, ArrowRightLeft } from 'lucide-react'

export const TicketsPage: React.FC = () => {
  return (
    <div className="bg-white p-8 rounded-2xl border border-slate-200 shadow-xs max-w-4xl mx-auto text-center">
      <div className="w-14 h-14 rounded-2xl bg-amber-50 text-amber-600 flex items-center justify-center mx-auto mb-4 border border-amber-100">
        <Ticket className="w-7 h-7" />
      </div>
      <h1 className="text-xl font-bold text-slate-900 mb-2">智能工单流转中心</h1>
      <p className="text-sm text-slate-500 max-w-md mx-auto mb-6">
        工单状态机（NEW → OPEN → PENDING → RESOLVED → CLOSED）与 SLA 实时预警，将在 Phase 5 贯通全流程。
      </p>
      <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-slate-100 text-slate-600 text-xs font-medium">
        <ArrowRightLeft className="w-3.5 h-3.5 text-amber-500" />
        <span>当前阶段已具备: 工单查看权限 (ticket:read) 校验通过</span>
      </div>
    </div>
  )
}
