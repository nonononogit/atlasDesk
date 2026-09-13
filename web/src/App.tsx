import React, { useEffect } from 'react'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useAuthStore } from './stores/authStore'
import { AppRoutes } from './routes'

// 初始化 TanStack Query 客户端
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})

export const App: React.FC = () => {
  const initAuth = useAuthStore((state) => state.initAuth)

  // 应用首次启动时，静默通过 HttpOnly Cookie 换取 Access Token
  useEffect(() => {
    initAuth()
  }, [initAuth])

  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </QueryClientProvider>
  )
}
