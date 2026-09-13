import React, { useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '../../stores/authStore'
import { ShieldCheck, Lock, Mail, ArrowRight, Loader2, Sparkles } from 'lucide-react'

export const LoginPage: React.FC = () => {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const login = useAuthStore((state) => state.login)
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as any)?.from?.pathname || '/dashboard'

  // 提交登录表单
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!email || !password) {
      setErrorMsg('请填写完整的邮箱与密码')
      return
    }

    try {
      setIsSubmitting(true)
      setErrorMsg('')
      await login(email, password)
      navigate(from, { replace: true })
    } catch (err: any) {
      setErrorMsg(err.message || '登录失败，请检查账号密码')
    } finally {
      setIsSubmitting(false)
    }
  }

  // 快捷填入默认演示超级管理员凭证
  const handleFillDemoAdmin = () => {
    setEmail('admin@atlasdesk.local')
    setPassword('Admin@123456')
    setErrorMsg('')
  }

  return (
    <div className="min-h-screen w-full flex items-center justify-center bg-slate-900 bg-[radial-gradient(ellipse_80%_80%_at_50%_-20%,rgba(14,165,233,0.15),rgba(255,255,255,0))] p-4">
      <div className="w-full max-w-md bg-slate-800/80 backdrop-blur-xl border border-slate-700/60 rounded-2xl p-8 shadow-2xl">
        {/* Brand Logo & Title */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-gradient-to-tr from-sky-500 to-blue-600 text-white shadow-lg shadow-sky-500/25 mb-4">
            <ShieldCheck className="w-6 h-6" />
          </div>
          <h1 className="text-2xl font-bold text-white tracking-tight">AtlasDesk 登录</h1>
          <p className="text-sm text-slate-400 mt-1">企业知识库与智能服务工作台</p>
        </div>

        {/* Error Alert */}
        {errorMsg && (
          <div className="mb-6 p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-400 text-sm flex items-center gap-2 animate-shake">
            <span className="w-1.5 h-1.5 rounded-full bg-rose-500"></span>
            {errorMsg}
          </div>
        )}

        {/* Form */}
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">企业工作邮箱</label>
            <div className="relative">
              <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-400">
                <Mail className="w-4 h-4" />
              </div>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="name@company.com"
                required
                className="w-full pl-10 pr-4 py-2.5 bg-slate-900/60 border border-slate-700 rounded-lg text-white text-sm placeholder-slate-500 focus:outline-none focus:border-sky-500 focus:ring-1 focus:ring-sky-500 transition-colors"
              />
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">登录密码</label>
            <div className="relative">
              <div className="absolute inset-y-0 left-0 pl-3.5 flex items-center pointer-events-none text-slate-400">
                <Lock className="w-4 h-4" />
              </div>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                required
                className="w-full pl-10 pr-4 py-2.5 bg-slate-900/60 border border-slate-700 rounded-lg text-white text-sm placeholder-slate-500 focus:outline-none focus:border-sky-500 focus:ring-1 focus:ring-sky-500 transition-colors"
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={isSubmitting}
            className="w-full py-2.5 px-4 mt-2 bg-gradient-to-r from-sky-500 to-blue-600 hover:from-sky-400 hover:to-blue-500 text-white font-medium text-sm rounded-lg shadow-lg shadow-sky-500/20 flex items-center justify-center gap-2 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>安全验证中...</span>
              </>
            ) : (
              <>
                <span>登录工作台</span>
                <ArrowRight className="w-4 h-4" />
              </>
            )}
          </button>
        </form>

        {/* Demo Fast Fill Section */}
        <div className="mt-8 pt-6 border-t border-slate-700/50">
          <div className="flex items-center justify-between text-xs text-slate-400 mb-3">
            <span>开发与评审快捷入口</span>
            <span className="text-slate-500">内置超级管理员</span>
          </div>
          <button
            type="button"
            onClick={handleFillDemoAdmin}
            className="w-full py-2 px-3 bg-slate-700/40 hover:bg-slate-700/70 border border-slate-600/50 text-slate-300 text-xs rounded-lg flex items-center justify-center gap-2 transition-colors"
          >
            <Sparkles className="w-3.5 h-3.5 text-sky-400" />
            <span>自动填入初始管理员账号 (admin@atlasdesk.local)</span>
          </button>
        </div>
      </div>
    </div>
  )
}
