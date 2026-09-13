import React from 'react'
import { Navigate, useLocation, Outlet } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import { Loader2, ShieldX } from 'lucide-react'

interface ProtectedRouteProps {
  requiredPermission?: string
}

// 路由守卫: 严格防止未授权内容闪现，并校验细粒度权限
export const ProtectedRoute: React.FC<ProtectedRouteProps> = ({ requiredPermission }) => {
  const { isLoading, isAuthenticated, hasPermission } = useAuthStore()
  const location = useLocation()

  // 1. 初始化静默换取 Token 期间展示白屏/平滑加载动画，杜绝未授权页面闪现
  if (isLoading) {
    return (
      <div className="min-h-screen flex flex-col items-center justify-center bg-slate-50">
        <div className="flex items-center gap-3 text-sky-600 font-medium text-sm">
          <Loader2 className="w-5 h-5 animate-spin" />
          <span>正在安全验证身份...</span>
        </div>
      </div>
    )
  }

  // 2. 未登录重定向至登录页，并保留来源路由以便登录后回跳
  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  // 3. 检查细粒度权限，无权限展示明确的 403 页面
  if (requiredPermission && !hasPermission(requiredPermission)) {
    return (
      <div className="min-h-[60vh] flex flex-col items-center justify-center text-center p-8">
        <div className="w-12 h-12 rounded-full bg-rose-50 flex items-center justify-center text-rose-500 mb-4">
          <ShieldX className="w-6 h-6" />
        </div>
        <h2 className="text-lg font-bold text-slate-900 mb-1">403 无权访问</h2>
        <p className="text-sm text-slate-500 max-w-md">
          您的账号角色缺少访问此页面所需的权限: <code className="text-rose-600 font-mono text-xs">{requiredPermission}</code>。
        </p>
      </div>
    )
  }

  return <Outlet />
}
