import React from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { LoginPage } from '../features/auth/LoginPage'
import { AppLayout } from '../layouts/AppLayout'
import { ProtectedRoute } from './ProtectedRoute'
import { DashboardPage } from '../features/dashboard/DashboardPage'
import { AssistantPage } from '../features/assistant/AssistantPage'
import { KnowledgePage } from '../features/knowledge/KnowledgePage'
import { TicketsPage } from '../features/tickets/TicketsPage'
import { MembersPage } from '../features/members/MembersPage'
import { SettingsPage } from '../features/settings/SettingsPage'

export const AppRoutes: React.FC = () => {
  return (
    <Routes>
      {/* 公开路由: 登录页 */}
      <Route path="/login" element={<LoginPage />} />

      {/* 受保护路由: 必须登录，且在 AppLayout 内部渲染 */}
      <Route element={<ProtectedRoute />}>
        <Route element={<AppLayout />}>
          <Route index element={<Navigate to="/dashboard" replace />} />
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/assistant" element={<AssistantPage />} />
          <Route path="/knowledge" element={<KnowledgePage />} />
          <Route path="/tickets" element={<TicketsPage />} />
          <Route path="/members" element={<MembersPage />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Route>
      </Route>

      {/* 兜底重定向 */}
      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  )
}
