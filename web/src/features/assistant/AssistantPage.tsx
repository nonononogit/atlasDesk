import React, { useState, useEffect, useRef, useCallback } from 'react'
import {
  assistantApi,
  streamMessage,
  ConversationItem,
  MessageItem,
  MessageCitationItem,
} from '../../api/assistant'
import {
  Bot,
  User,
  Send,
  Square,
  Plus,
  Trash2,
  Bookmark,
  Check,
  Copy,
  ThumbsUp,
  ThumbsDown,
  Sparkles,
  Search,
  X,
  MessageSquare,
  Loader2,
  FileText,
} from 'lucide-react'

// 推荐快捷问题 (规范 A-06: 点击仅填入输入框，不自动触发)
const SUGGESTED_PROMPTS = [
  'AtlasDesk 系统的架构设计是怎样的？',
  '如何为新成员配置组织与数据权限？',
  '知识库文档切片与混合检索支持哪些格式？',
  '系统支持多大文件直传与哪些类型限制？',
]

export const AssistantPage: React.FC = () => {
  // 会话与消息状态
  const [conversations, setConversations] = useState<ConversationItem[]>([])
  const [activeConv, setActiveConv] = useState<ConversationItem | null>(null)
  const [messages, setMessages] = useState<MessageItem[]>([])
  const [inputContent, setInputContent] = useState('')

  // 流式交互控制
  const [isGenerating, setIsGenerating] = useState(false)
  const [retrievalStatus, setRetrievalStatus] = useState<string | null>(null)
  const abortControllerRef = useRef<AbortController | null>(null)

  // 来源面板展开状态
  const [activeCitation, setActiveCitation] = useState<MessageCitationItem | null>(null)
  const [isDrawerOpen, setIsDrawerOpen] = useState(false)

  // 复制与反馈提示
  const [copiedId, setCopiedId] = useState<string | null>(null)
  const [feedbackSuccessId, setFeedbackSuccessId] = useState<string | null>(null)

  const messagesEndRef = useRef<HTMLDivElement>(null)

  // 滚动到最新消息
  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [])

  // 1. 加载会话历史列表
  const loadConversations = useCallback(async () => {
    try {
      const res = await assistantApi.listConversations()
      setConversations(res.items || [])
    } catch (err) {
      console.error('加载会话列表失败:', err)
    }
  }, [])

  useEffect(() => {
    loadConversations()
  }, [loadConversations])

  // 2. 切换会话加载历史消息
  const handleSelectConversation = async (conv: ConversationItem) => {
    if (isGenerating) return
    setActiveConv(conv)
    setActiveCitation(null)
    setIsDrawerOpen(false)
    try {
      const history = await assistantApi.listMessages(conv.id)
      setMessages(history || [])
      setTimeout(scrollToBottom, 50)
    } catch (err) {
      console.error('加载历史消息失败:', err)
    }
  }

  // 3. 新建对话 (规范 A-02: 清空输入与消息，不立即创建数据库记录)
  const handleNewConversation = () => {
    if (isGenerating) return
    setActiveConv(null)
    setMessages([])
    setInputContent('')
    setActiveCitation(null)
    setIsDrawerOpen(false)
    setRetrievalStatus(null)
  }

  // 4. 删除会话
  const handleDeleteConversation = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    if (!window.confirm('确认删除该会话记录？')) return
    try {
      await assistantApi.deleteConversation(id)
      if (activeConv?.id === id) {
        handleNewConversation()
      }
      loadConversations()
    } catch (err) {
      console.error('删除会话失败:', err)
    }
  }

  // 5. 发送问题并流式接收回答 (符合规范 A-03 要求)
  const handleSend = async (contentToSend?: string) => {
    const text = (contentToSend || inputContent).trim()
    if (!text || isGenerating) return
    if (text.length > 2000) {
      alert('问题内容不能超过 2,000 字')
      return
    }

    setInputContent('')
    setIsGenerating(true)
    setRetrievalStatus('正在分析问题并混合检索知识库...')

    // 本地临时插入用户消息
    const tempUserMsgId = 'temp-user-' + Date.now()
    const tempAssistantMsgId = 'temp-assistant-' + Date.now()

    const userMsg: MessageItem = {
      id: tempUserMsgId,
      conversation_id: activeConv?.id || 'pending',
      role: 'user',
      content: text,
      status: 'sent',
      created_at: new Date().toISOString(),
    }

    const assistantMsgPlaceholder: MessageItem = {
      id: tempAssistantMsgId,
      conversation_id: activeConv?.id || 'pending',
      role: 'assistant',
      content: '',
      status: 'sending',
      created_at: new Date().toISOString(),
      citations: [],
    }

    setMessages((prev) => [...prev, userMsg, assistantMsgPlaceholder])
    setTimeout(scrollToBottom, 50)

    const abortController = new AbortController()
    abortControllerRef.current = abortController

    const targetConvId = activeConv?.id || 'new'

    await streamMessage(
      targetConvId,
      text,
      undefined,
      {
        onRetrievalStarted: () => {
          setRetrievalStatus('检索到高相关切片，正在组织答案...')
          scrollToBottom()
        },
        onAnswerDelta: (delta: string) => {
          setRetrievalStatus(null)
          setMessages((prev) =>
            prev.map((msg) =>
              msg.id === tempAssistantMsgId
                ? { ...msg, content: msg.content + delta }
                : msg,
            ),
          )
          scrollToBottom()
        },
        onCitations: (citations: MessageCitationItem[]) => {
          setMessages((prev) =>
            prev.map((msg) =>
              msg.id === tempAssistantMsgId ? { ...msg, citations } : msg,
            ),
          )
          scrollToBottom()
        },
        onDone: (data) => {
          setIsGenerating(false)
          setRetrievalStatus(null)
          setMessages((prev) =>
            prev.map((msg) =>
              msg.id === tempAssistantMsgId
                ? { ...msg, id: data.message_id, status: 'sent', usage: data.usage }
                : msg,
            ),
          )
          // 首次提问后刷新会话侧边栏，获取真实标题
          loadConversations()
        },
        onError: (err) => {
          setIsGenerating(false)
          setRetrievalStatus(null)
          setMessages((prev) =>
            prev.map((msg) =>
              msg.id === tempAssistantMsgId
                ? { ...msg, status: 'failed', content: msg.content || '生成回答失败: ' + err.message }
                : msg,
            ),
          )
        },
      },
      abortController.signal,
    )
  }

  // 6. 停止生成 (规范 A-07)
  const handleStop = () => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort()
      abortControllerRef.current = null
      setIsGenerating(false)
      setRetrievalStatus(null)
    }
  }

  // 7. 复制纯文本回答 (保留 [1] 引用编号)
  const handleCopy = (msg: MessageItem) => {
    navigator.clipboard.writeText(msg.content)
    setCopiedId(msg.id)
    setTimeout(() => setCopiedId(null), 2000)
  }

  // 8. 提交回答点赞/点踩反馈
  const handleFeedback = async (messageId: string, rating: number) => {
    let reason = ''
    if (rating === -1) {
      reason = window.prompt('请简要说明未采纳的原因（如回答不准确、缺少关键步骤）：') || ''
    }
    try {
      await assistantApi.submitFeedback(messageId, rating, reason)
      setFeedbackSuccessId(messageId)
      setTimeout(() => setFeedbackSuccessId(null), 2000)
    } catch (err) {
      console.error('提交反馈失败:', err)
    }
  }

  // 9. 打开来源面板
  const handleOpenSource = (citation: MessageCitationItem) => {
    setActiveCitation(citation)
    setIsDrawerOpen(true)
  }

  return (
    <div className="flex h-[calc(100vh-7.5rem)] bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-sm overflow-hidden animate-fade-in relative">
      {/* ------------------------------------------------------------------------ */}
      {/* 1. 左侧历史会话侧边栏 */}
      {/* ------------------------------------------------------------------------ */}
      <div className="w-64 border-r border-slate-100 dark:border-slate-800 bg-slate-50/60 dark:bg-slate-900/40 flex flex-col shrink-0">
        <div className="p-3 border-b border-slate-100 dark:border-slate-800">
          <button
            onClick={handleNewConversation}
            className="w-full flex items-center justify-center gap-2 px-3 py-2 bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl text-xs font-semibold shadow-xs transition-colors"
          >
            <Plus className="w-4 h-4" />
            <span>新建对话</span>
          </button>
        </div>

        {/* 历史会话列表 */}
        <div className="flex-1 overflow-y-auto p-2 space-y-1">
          <div className="px-2 py-1 text-[11px] font-semibold text-slate-400 uppercase tracking-wider">
            对话记录
          </div>
          {conversations.length === 0 ? (
            <div className="p-4 text-center text-xs text-slate-400">暂无历史记录</div>
          ) : (
            conversations.map((conv) => {
              const isActive = activeConv?.id === conv.id
              return (
                <div
                  key={conv.id}
                  onClick={() => handleSelectConversation(conv)}
                  className={`group flex items-center justify-between px-3 py-2.5 rounded-xl text-xs cursor-pointer transition-all ${
                    isActive
                      ? 'bg-white dark:bg-slate-800 text-indigo-600 dark:text-indigo-400 font-medium shadow-xs'
                      : 'text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800/60'
                  }`}
                >
                  <div className="flex items-center gap-2 truncate">
                    <MessageSquare className="w-3.5 h-3.5 shrink-0" />
                    <span className="truncate">{conv.title}</span>
                  </div>
                  <button
                    onClick={(e) => handleDeleteConversation(conv.id, e)}
                    className="opacity-0 group-hover:opacity-100 p-1 text-slate-400 hover:text-rose-600 rounded transition-opacity"
                    title="删除会话"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              )
            })
          )}
        </div>
      </div>

      {/* ------------------------------------------------------------------------ */}
      {/* 2. 中间消息对话主交互区 */}
      {/* ------------------------------------------------------------------------ */}
      <div className="flex-1 flex flex-col overflow-hidden bg-white dark:bg-slate-900">
        {/* 对话消息滚动流 */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {messages.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-center max-w-lg mx-auto py-12">
              <div className="w-14 h-14 rounded-2xl bg-indigo-50 dark:bg-indigo-950/50 text-indigo-600 dark:text-indigo-400 flex items-center justify-center mb-4 border border-indigo-100 dark:border-indigo-900">
                <Bot className="w-7 h-7" />
              </div>
              <h2 className="text-lg font-bold text-slate-900 dark:text-white mb-2">
                AtlasDesk 企业智能服务台助手
              </h2>
              <p className="text-xs text-slate-500 dark:text-slate-400 mb-6 leading-relaxed">
                结合多格式文档解析、RRF 混合检索与 1536 维向量技术，为您提供精准、带引用的专业问答解答。
              </p>

              {/* 推荐快捷问题卡片 (规范 A-06: 点击仅填入输入框) */}
              <div className="w-full grid grid-cols-1 sm:grid-cols-2 gap-2 text-left">
                {SUGGESTED_PROMPTS.map((prompt, idx) => (
                  <button
                    key={idx}
                    onClick={() => setInputContent(prompt)}
                    className="p-3 text-xs rounded-xl border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-800/40 hover:border-indigo-400 dark:hover:border-indigo-600 text-slate-700 dark:text-slate-300 transition-all flex items-start gap-2"
                  >
                    <Sparkles className="w-3.5 h-3.5 text-indigo-500 shrink-0 mt-0.5" />
                    <span>{prompt}</span>
                  </button>
                ))}
              </div>
            </div>
          ) : (
            messages.map((msg) => {
              const isUser = msg.role === 'user'
              return (
                <div
                  key={msg.id}
                  className={`flex gap-3.5 ${isUser ? 'flex-row-reverse' : 'flex-row'}`}
                >
                  {/* 头像 */}
                  <div
                    className={`w-8 h-8 rounded-xl flex items-center justify-center shrink-0 text-xs font-semibold ${
                      isUser
                        ? 'bg-slate-900 text-white dark:bg-indigo-600'
                        : 'bg-indigo-50 dark:bg-indigo-950/50 text-indigo-600 dark:text-indigo-400 border border-indigo-100 dark:border-indigo-900'
                    }`}
                  >
                    {isUser ? <User className="w-4 h-4" /> : <Bot className="w-4 h-4" />}
                  </div>

                  {/* 消息正文与卡片 */}
                  <div className={`max-w-[78%] space-y-2.5 ${isUser ? 'items-end' : 'items-start'}`}>
                    <div
                      className={`p-4 rounded-2xl text-xs sm:text-sm leading-relaxed whitespace-pre-wrap ${
                        isUser
                          ? 'bg-slate-900 dark:bg-indigo-600 text-white rounded-tr-none'
                          : 'bg-slate-50 dark:bg-slate-800/60 border border-slate-100 dark:border-slate-700/60 text-slate-800 dark:text-slate-100 rounded-tl-none'
                      }`}
                    >
                      {msg.content || (
                        <div className="flex items-center gap-2 text-slate-400">
                          <Loader2 className="w-4 h-4 animate-spin text-indigo-500" />
                          <span>正在组织精准解答...</span>
                        </div>
                      )}
                    </div>

                    {/* 助手回答的引用卡片与操作栏 */}
                    {!isUser && msg.status !== 'sending' && (
                      <div className="space-y-2">
                        {/* 引用卡片列表 (符合规范 A-05) */}
                        {msg.citations && msg.citations.length > 0 && (
                          <div className="flex flex-wrap items-center gap-1.5 pt-1">
                            <span className="text-[11px] text-slate-400 flex items-center gap-1 mr-1">
                              <Bookmark className="w-3 h-3 text-indigo-500" />
                              来源引用:
                            </span>
                            {msg.citations.map((citation) => (
                              <button
                                key={citation.id || citation.citation_index}
                                onClick={() => handleOpenSource(citation)}
                                className="inline-flex items-center gap-1 px-2 py-0.5 rounded-lg bg-indigo-50 dark:bg-indigo-950/60 text-indigo-600 dark:text-indigo-400 hover:bg-indigo-100 dark:hover:bg-indigo-900/60 border border-indigo-200/60 text-[11px] font-medium transition-colors"
                              >
                                <span>[{citation.citation_index}]</span>
                                <span className="max-w-[120px] truncate">{citation.document_title}</span>
                              </button>
                            ))}
                          </div>
                        )}

                        {/* 回答快捷操作栏 (复制、点赞、点踩、重新生成) */}
                        <div className="flex items-center gap-2 text-[11px] text-slate-400 pt-0.5">
                          <button
                            onClick={() => handleCopy(msg)}
                            className="flex items-center gap-1 px-2 py-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
                            title="复制纯文本回答"
                          >
                            {copiedId === msg.id ? (
                              <>
                                <Check className="w-3 h-3 text-emerald-500" />
                                <span className="text-emerald-500">已复制</span>
                              </>
                            ) : (
                              <>
                                <Copy className="w-3 h-3" />
                                <span>复制</span>
                              </>
                            )}
                          </button>

                          <button
                            onClick={() => handleFeedback(msg.id, 1)}
                            className="flex items-center gap-1 px-2 py-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
                            title="有帮助"
                          >
                            <ThumbsUp className="w-3 h-3" />
                            <span>有帮助</span>
                          </button>

                          <button
                            onClick={() => handleFeedback(msg.id, -1)}
                            className="flex items-center gap-1 px-2 py-1 rounded hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
                            title="未采纳"
                          >
                            <ThumbsDown className="w-3 h-3" />
                            <span>没帮助</span>
                          </button>

                          {feedbackSuccessId === msg.id && (
                            <span className="text-emerald-600 dark:text-emerald-400 font-medium">
                              感谢您的反馈！
                            </span>
                          )}
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )
            })
          )}

          {/* 正在检索指示条 */}
          {retrievalStatus && (
            <div className="flex items-center gap-2 px-4 py-2 bg-indigo-50/60 dark:bg-indigo-950/40 border border-indigo-100 dark:border-indigo-900 rounded-xl text-xs text-indigo-600 dark:text-indigo-400 w-fit animate-pulse">
              <Search className="w-3.5 h-3.5 animate-spin" />
              <span>{retrievalStatus}</span>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>

        {/* -------------------------------------------------------------------- */}
        {/* 底部问题输入框与发送控制 */}
        {/* -------------------------------------------------------------------- */}
        <div className="p-4 border-t border-slate-100 dark:border-slate-800 bg-white dark:bg-slate-900">
          <div className="relative flex items-end gap-2 bg-slate-50 dark:bg-slate-800/80 border border-slate-200 dark:border-slate-700 rounded-2xl p-2 focus-within:ring-2 focus-within:ring-indigo-500/20 focus-within:border-indigo-500 transition-all">
            <textarea
              rows={2}
              value={inputContent}
              onChange={(e) => setInputContent(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault()
                  handleSend()
                }
              }}
              placeholder="询问关于知识库的任何问题... (Enter 发送，Shift + Enter 换行)"
              className="w-full bg-transparent text-xs sm:text-sm p-2 text-slate-900 dark:text-white placeholder-slate-400 focus:outline-none resize-none"
            />

            <div className="flex items-center gap-1.5 pb-1 pr-1">
              {isGenerating ? (
                <button
                  onClick={handleStop}
                  className="flex items-center gap-1 px-3 py-1.5 bg-rose-600 hover:bg-rose-700 text-white rounded-xl text-xs font-semibold shadow-xs transition-colors"
                >
                  <Square className="w-3.5 h-3.5 fill-current" />
                  <span>停止生成</span>
                </button>
              ) : (
                <button
                  onClick={() => handleSend()}
                  disabled={!inputContent.trim()}
                  className="p-2 bg-indigo-600 hover:bg-indigo-700 disabled:bg-slate-200 dark:disabled:bg-slate-700 text-white rounded-xl shadow-xs transition-colors"
                  title="发送提问"
                >
                  <Send className="w-4 h-4" />
                </button>
              )}
            </div>
          </div>
          <div className="flex items-center justify-between text-[11px] text-slate-400 px-2 mt-1.5">
            <span>支持 Markdown 格式与代码块渲染</span>
            <span>{inputContent.length} / 2,000 字</span>
          </div>
        </div>
      </div>

      {/* ------------------------------------------------------------------------ */}
      {/* 3. 右侧来源抽屉面板 (Source Drawer) */}
      {/* ------------------------------------------------------------------------ */}
      {isDrawerOpen && activeCitation && (
        <div className="w-80 border-l border-slate-100 dark:border-slate-800 bg-slate-50/70 dark:bg-slate-900 flex flex-col shrink-0 animate-fade-in">
          <div className="flex items-center justify-between p-4 border-b border-slate-100 dark:border-slate-800">
            <div className="flex items-center gap-2">
              <Bookmark className="w-4 h-4 text-indigo-600" />
              <h3 className="font-semibold text-xs text-slate-900 dark:text-white">
                来源片段 [{activeCitation.citation_index}]
              </h3>
            </div>
            <button
              onClick={() => setIsDrawerOpen(false)}
              className="p-1 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          </div>

          <div className="flex-1 overflow-y-auto p-4 space-y-4">
            {/* 文档与页码元数据 */}
            <div className="p-3 bg-white dark:bg-slate-800 rounded-xl border border-slate-200 dark:border-slate-700 space-y-1.5 text-xs">
              <div className="flex items-center gap-2 font-medium text-slate-900 dark:text-white">
                <FileText className="w-4 h-4 text-indigo-500 shrink-0" />
                <span className="truncate">{activeCitation.document_title}</span>
              </div>
              {activeCitation.page_number > 0 && (
                <div className="text-slate-500 dark:text-slate-400 text-[11px]">
                  对应页码: 第 {activeCitation.page_number} 页
                </div>
              )}
            </div>

            {/* 原文快照内容 */}
            <div className="space-y-1.5">
              <div className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider">
                命中高亮原文
              </div>
              <div className="p-3 bg-indigo-50/50 dark:bg-indigo-950/40 rounded-xl border border-indigo-100 dark:border-indigo-900/60 text-xs text-slate-800 dark:text-slate-200 leading-relaxed whitespace-pre-wrap">
                {activeCitation.quote}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
