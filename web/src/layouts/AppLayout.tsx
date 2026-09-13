import React, { useState } from 'react'
import { Outlet, NavLink, useNavigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import {
  LayoutDashboard,
  Bot,
  BookOpen,
  Ticket,
  Users,
  Settings,
  LogOut,
  Menu,
  X,
  Search,
  Building2,
  ShieldAlert,
} from 'lucide-react'

interface MenuItem {
  name: string
  path: string
  icon: React.ComponentType<{ className?: string }>
  permission: string
  badge?: string
}

// 规格第 6.3 节规定的 6 个受权限控制的核心菜单项
const NAVIGATION_ITEMS: MenuItem[] = [
  {
    name: '工作台',
    path: '/dashboard',
    icon: LayoutDashboard,
    permission: 'dashboard:read',
  },
  {
    name: '知识助手',
    path: '/assistant',
    icon: Bot,
    permission: 'knowledge:read',
  },
  {
    name: '企业知识库',
    path: '/knowledge',
    icon: BookOpen,
    permission: 'knowledge:read',
  },
  {
    name: '智能工单',
    path: '/tickets',
    icon: Ticket,
    permission: 'ticket:read',
    badge: '3', // 待处理工单徽标提示
  },
  {
    name: '团队成员',
    path: '/members',
    icon: Users,
    permission: 'member:manage',
  },
  {
    name: '系统设置',
    path: '/settings',
    icon: Settings,
    permission: 'ai_config:manage',
  },
]

export const AppLayout: React.FC = () => {
  const [mobileDrawerOpen, setMobileDrawerOpen] = useState(false)
  const user = useAuthStore((state) => state.user)
  const logout = useAuthStore((state) => state.logout)
  const hasPermission = useAuthStore((state) => state.hasPermission)
  const navigate = useNavigate()
  const location = useLocation()

  // 退出登录并清理全部缓存
  const handleLogout = async () => {
    await logout()
    navigate('/login', { replace: true })
  }

  // 过滤当前用户具有权限的菜单
  const visibleMenus = NAVIGATION_ITEMS.filter((item) => hasPermission(item.permission))

  return (
    <div className="min-h-screen bg-slate-50 flex flex-col antialiased">
      {/* 顶部全局顶栏 (Header) */}
      <header className="sticky top-0 z-40 bg-white border-b border-slate-200/80 px-4 sm:px-6 h-16 flex items-center justify-between shadow-sm">
        <div className="flex items-center gap-4">
          {/* 移动端汉堡折叠按钮 */}
          <button
            type="button"
            onClick={() => setMobileDrawerOpen(true)}
            className="md:hidden p-2 text-slate-600 hover:text-slate-900 rounded-lg hover:bg-slate-100"
          >
            <Menu className="w-5 h-5" />
          </button>

          {/* Logo */}
          <div className="flex items-center gap-2">
            <span className="w-4 h-4 rounded-full bg-sky-500 ring-4 ring-sky-100"></span>
            <span className="font-bold text-lg text-slate-900 tracking-tight">AtlasDesk</span>
          </div>

          {/* 组织标识 (MVP 只展示当前组织，不实现伪切换) */}
          <div className="hidden sm:flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-slate-100 text-slate-700 text-xs font-medium border border-slate-200">
            <Building2 className="w-3.5 h-3.5 text-slate-500" />
            <span>{user?.organization_name || '演示企业'}</span>
          </div>
        </div>

        {/* 顶栏右侧快捷搜索与用户信息 */}
        <div className="flex items-center gap-3">
          {/* 全局搜索快捷触发按钮 (Cmd+K 提示) */}
          <button
            type="button"
            className="hidden md:flex items-center gap-2 px-3 py-1.5 rounded-lg border border-slate-200 bg-slate-50 text-xs text-slate-400 hover:border-slate-300 hover:text-slate-600 transition-colors"
          >
            <Search className="w-3.5 h-3.5" />
            <span>搜索文档、工单与成员...</span>
            <kbd className="px-1.5 py-0.5 rounded bg-white border border-slate-200 text-[10px] text-slate-500 font-mono shadow-xs">
              ⌘K
            </kbd>
          </button>

          {/* 用户画像与退出按钮 */}
          <div className="flex items-center gap-3 pl-3 border-l border-slate-200">
            <div className="text-right hidden sm:block">
              <div className="text-xs font-semibold text-slate-800">{user?.name || '管理员'}</div>
              <div className="text-[11px] text-slate-400">{user?.role_name || '角色未知'}</div>
            </div>

            <div className="w-8 h-8 rounded-full bg-gradient-to-br from-sky-400 to-blue-600 flex items-center justify-center text-white text-xs font-bold shadow-xs">
              {user?.name ? user.name.slice(0, 1) : 'U'}
            </div>

            <button
              onClick={handleLogout}
              title="退出登录"
              className="p-1.5 text-slate-400 hover:text-rose-600 hover:bg-rose-50 rounded-lg transition-colors"
            >
              <LogOut className="w-4 h-4" />
            </button>
          </div>
        </div>
      </header>

      <div className="flex-1 flex overflow-hidden">
        {/* PC 端常驻侧边栏 (Sidebar) */}
        <aside className="hidden md:flex flex-col w-64 border-r border-slate-200 bg-white p-4 justify-between">
          <nav className="space-y-1">
            <div className="text-[11px] font-medium text-slate-400 uppercase tracking-wider px-3 py-2">
              工作导航
            </div>
            {visibleMenus.map((item) => {
              const Icon = item.icon
              const isActive = location.pathname === item.path
              return (
                <NavLink
                  key={item.path}
                  to={item.path}
                  className={`flex items-center justify-between px-3 py-2 rounded-lg text-sm font-medium transition-all ${
                    isActive
                      ? 'bg-sky-50 text-sky-700 shadow-xs font-semibold'
                      : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900'
                  }`}
                >
                  <div className="flex items-center gap-3">
                    <Icon className={`w-4 h-4 ${isActive ? 'text-sky-600' : 'text-slate-400'}`} />
                    <span>{item.name}</span>
                  </div>
                  {item.badge && (
                    <span className="px-1.5 py-0.5 text-[10px] font-bold rounded-full bg-amber-100 text-amber-700">
                      {item.badge}
                    </span>
                  )}
                </NavLink>
              )
            })}
          </nav>

          <div className="p-3 bg-slate-50 rounded-xl border border-slate-200 text-xs text-slate-500">
            <div className="font-semibold text-slate-700 mb-0.5 flex items-center gap-1.5">
              <ShieldAlert className="w-3.5 h-3.5 text-sky-500" />
              <span>RBAC 权限守卫</span>
            </div>
            <p className="text-[11px] text-slate-400">侧栏菜单根据服务端下发的实时权限动态过滤展示。</p>
          </div>
        </aside>

        {/* 移动端抽屉 (Drawer) */}
        {mobileDrawerOpen && (
          <div className="fixed inset-0 z-50 md:hidden flex">
            {/* Backdrop */}
            <div
              className="fixed inset-0 bg-slate-900/40 backdrop-blur-xs"
              onClick={() => setMobileDrawerOpen(false)}
            />
            {/* Drawer Content */}
            <div className="relative w-64 bg-white h-full p-4 flex flex-col justify-between shadow-2xl z-10 animate-slide-in">
              <div>
                <div className="flex items-center justify-between pb-4 mb-4 border-b border-slate-100">
                  <div className="flex items-center gap-2">
                    <span className="w-3 h-3 rounded-full bg-sky-500"></span>
                    <span className="font-bold text-slate-900">AtlasDesk</span>
                  </div>
                  <button
                    onClick={() => setMobileDrawerOpen(false)}
                    className="p-1 text-slate-400 hover:text-slate-600"
                  >
                    <X className="w-5 h-5" />
                  </button>
                </div>

                <nav className="space-y-1">
                  {visibleMenus.map((item) => {
                    const Icon = item.icon
                    const isActive = location.pathname === item.path
                    return (
                      <NavLink
                        key={item.path}
                        to={item.path}
                        onClick={() => setMobileDrawerOpen(false)}
                        className={`flex items-center justify-between px-3 py-2.5 rounded-lg text-sm font-medium ${
                          isActive
                            ? 'bg-sky-50 text-sky-700 font-semibold'
                            : 'text-slate-600 hover:bg-slate-50'
                        }`}
                      >
                        <div className="flex items-center gap-3">
                          <Icon className="w-4 h-4" />
                          <span>{item.name}</span>
                        </div>
                      </NavLink>
                    )
                  })}
                </nav>
              </div>

              <button
                onClick={handleLogout}
                className="w-full flex items-center justify-center gap-2 py-2 px-3 text-sm text-rose-600 bg-rose-50 rounded-lg hover:bg-rose-100"
              >
                <LogOut className="w-4 h-4" />
                <span>退出登录</span>
              </button>
            </div>
          </div>
        )}

        {/* 核心主工作区 */}
        <main className="flex-1 overflow-y-auto p-4 sm:p-8">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
