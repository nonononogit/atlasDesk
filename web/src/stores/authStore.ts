import { create } from 'zustand'
import { apiClient, setAccessToken } from '../api/client'

export interface AuthUser {
  id: string
  email: string
  name: string
  organization_id: string
  organization_name: string
  role_name: string
  permissions: string[]
}

interface AuthState {
  user: AuthUser | null
  isLoading: boolean
  isAuthenticated: boolean
  initAuth: () => Promise<void>
  login: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
  hasPermission: (perm: string) => boolean
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  isLoading: true, // 初始处于加载中，避免页面初次载入时闪现未授权或登录界面
  isAuthenticated: false,

  // 应用初始化时调用：尝试利用 HttpOnly Cookie 静默换取 Token
  initAuth: async () => {
    try {
      set({ isLoading: true })
      const res = await apiClient.post<{ access_token: string; user: AuthUser }>('/api/v1/auth/refresh')
      setAccessToken(res.access_token)
      set({
        user: res.user,
        isAuthenticated: true,
        isLoading: false,
      })
    } catch {
      setAccessToken(null)
      set({
        user: null,
        isAuthenticated: false,
        isLoading: false,
      })
    }
  },

  // 用户主动登录
  login: async (email: string, password: string) => {
    const res = await apiClient.post<{ access_token: string; user: AuthUser }>('/api/v1/auth/login', {
      email,
      password,
    })
    setAccessToken(res.access_token)
    set({
      user: res.user,
      isAuthenticated: true,
      isLoading: false,
    })
  },

  // 用户注销登录
  logout: async () => {
    try {
      await apiClient.post('/api/v1/auth/logout')
    } finally {
      setAccessToken(null)
      set({
        user: null,
        isAuthenticated: false,
        isLoading: false,
      })
    }
  },

  // 权限判断辅助函数
  hasPermission: (perm: string) => {
    const { user } = get()
    if (!user) return false
    if (user.role_name === '超级管理员' || user.role_name === 'super_admin') {
      return true
    }
    return user.permissions.includes(perm)
  },
}))

// 监听未授权全局事件，自动清除状态
if (typeof window !== 'undefined') {
  window.addEventListener('atlasdesk:unauthorized', () => {
    useAuthStore.getState().logout()
  })
}
