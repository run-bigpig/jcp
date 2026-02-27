import React, { useState, useEffect, useRef } from 'react';
import { Stock, KLineData } from '../types';
import { getAgentConfigs, AgentConfig } from '../services/strategyService';
import { StockSession, ChatMessage, sendMeetingMessage, MeetingMessageRequest, getSessionMessages, retryAgent, retryAgentAndContinue, cancelInterruptedMeeting } from '../services/sessionService';
import { MessageSquare, Loader2, Send, User, Users, X, Reply, Trash2, Wrench, CheckCircle2, AlertCircle, Copy, Check, RotateCcw, Pencil, Square } from 'lucide-react';
import { clearSessionMessages } from '../services/sessionService';
import { NodeRenderer } from 'markstream-react';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import { useMentionPicker } from '../hooks/useMentionPicker';
import { useTheme } from '../contexts/ThemeContext';
import { CancelMeeting } from '../../wailsjs/go/main/App';
import 'markstream-react/index.css';

// 进度事件类型
interface ProgressEvent {
  type: 'agent_start' | 'agent_done' | 'tool_call' | 'tool_result' | 'streaming' | 'agent_error' | 'meeting_interrupted';
  agentId: string;
  agentName: string;
  detail?: string;
  content?: string;
}

// 进度状态
interface ProgressState {
  currentAgent: string | null;
  currentAgentName: string | null;
  steps: { type: string; detail: string; done: boolean }[];
  streamingText: string;
}

interface AgentRoomProps {
  stock: Stock;
  kLineData: KLineData[];
  session: StockSession | null;
  onSessionUpdate: (session: StockSession) => void;
}

export const AgentRoom: React.FC<AgentRoomProps> = ({ session, onSessionUpdate }) => {
  const { colors } = useTheme();
  const [allAgents, setAllAgents] = useState<AgentConfig[]>([]);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [simulatingMap, setSimulatingMap] = useState<Record<string, boolean>>({});
  const [userQuery, setUserQuery] = useState('');
  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // 当前会话是否在会议中
  const isSimulating = session?.stockCode ? simulatingMap[session.stockCode] || false : false;

  // 跟踪当前活跃的 stockCode
  const currentStockCodeRef = useRef<string | null>(null);

  // 跟踪上一次的 stockCode（用于检测切换）
  const prevStockCodeRef = useRef<string | null>(null);

  // 会议取消标识
  const meetingCancelledRef = useRef<Record<string, boolean>>({});

  // 使用自定义 Hooks
  const {
    mentionedAgents,
    showMentionPicker,
    mentionSearchText,
    mentionSelectedIndex,
    filteredAgents,
    mentionListRef,
    handleInputChange: handleMentionInput,
    handleKeyDown: handleMentionKeyDown,
    handleSelectMention,
    toggleMention,
    clearMentions,
    closePicker,
  } = useMentionPicker({ allAgents });

  // 其他状态
  const [replyToMessage, setReplyToMessage] = useState<ChatMessage | null>(null);
  const [showClearConfirm, setShowClearConfirm] = useState(false);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [failedUserMsgId, setFailedUserMsgId] = useState<string | null>(null);
  const [retryingAgentId, setRetryingAgentId] = useState<string | null>(null);

  // 进度状态
  const [progress, setProgress] = useState<ProgressState>({
    currentAgent: null,
    currentAgentName: null,
    steps: [],
    streamingText: '',
  });

  // 在聊天窗口中添加系统提示消息
  const addSystemMessage = (text: string) => {
    setMessages(prev => [...prev, {
      id: `sys-${Date.now()}-${Math.random()}`,
      agentId: 'system',
      agentName: '',
      role: '',
      content: text,
      timestamp: Date.now(),
    }]);
  };

  // 取消指定股票的会议
  const cancelMeeting = (stockCode: string) => {
    // 调用后端取消 API
    CancelMeeting(stockCode).catch(err => {
      console.error('[AgentRoom] 取消会议失败:', err);
    });
    // 前端状态重置
    meetingCancelledRef.current[stockCode] = true;
    setSimulatingMap(prev => ({ ...prev, [stockCode]: false }));
    setProgress({
      currentAgent: null,
      currentAgentName: null,
      steps: [],
      streamingText: '',
    });
    addSystemMessage('讨论已停止');
  };

  // 加载Agent配置
  const loadAgents = () => {
    getAgentConfigs()
      .then(agents => setAllAgents(agents || []))
      .catch(err => {
        console.error('[AgentRoom] 加载Agent配置失败:', err);
        setAllAgents([]);
      });
  };

  // 初始加载Agent配置
  useEffect(() => {
    loadAgents();
  }, []);

  // 监听策略切换事件，重新加载Agent配置
  useEffect(() => {
    const cleanup = EventsOn('strategy:changed', () => {
      console.log('[AgentRoom] 策略已切换，重新加载Agent配置');
      loadAgents();
    });
    return () => {
      EventsOff('strategy:changed');
      if (cleanup) cleanup();
    };
  }, []);

  // 当Session变化时，从后端加载最新消息
  useEffect(() => {
    // 使用 prevStockCodeRef 获取真正的上一次 stockCode
    const prevStockCode = prevStockCodeRef.current;
    const newStockCode = session?.stockCode || null;

    if (newStockCode) {
      // 如果切换到新股票，取消之前股票的会议
      if (prevStockCode && prevStockCode !== newStockCode && simulatingMap[prevStockCode]) {
        cancelMeeting(prevStockCode);
        addSystemMessage('已切换股票，之前的会议已取消');
      }

      // 从后端获取最新消息（包括切换期间产生的新消息）
      getSessionMessages(newStockCode).then(msgs => {
        setMessages(msgs || []);
      });
    } else {
      setMessages([]);
    }
    setUserQuery('');

    // 更新 refs（在 effect 结束时更新，确保下次能正确检测切换）
    prevStockCodeRef.current = newStockCode;
    currentStockCodeRef.current = newStockCode;
  }, [session?.stockCode]);

  // 订阅会议消息事件（实时接收发言）
  useEffect(() => {
    if (!session?.stockCode) return;

    const stockCode = session.stockCode;
    const eventName = `meeting:message:${stockCode}`;
    const cleanup = EventsOn(eventName, (msg: ChatMessage) => {
      // 检查是否已取消或切换了股票
      if (meetingCancelledRef.current[stockCode]) return;
      if (currentStockCodeRef.current === stockCode) {
        setMessages(prev => [...prev, { ...msg, id: `msg-${Date.now()}-${Math.random()}`, timestamp: Date.now() }]);
      }
    });

    return () => {
      EventsOff(eventName);
      if (cleanup) cleanup();
    };
  }, [session?.stockCode]);

  // 订阅进度事件（工具调用、流式输出等）
  useEffect(() => {
    if (!session?.stockCode) return;

    const stockCode = session.stockCode;
    const eventName = `meeting:progress:${stockCode}`;
    const cleanup = EventsOn(eventName, (event: ProgressEvent) => {
      // 检查是否已取消或切换了股票
      if (meetingCancelledRef.current[stockCode]) return;
      if (currentStockCodeRef.current !== stockCode) return;

      setProgress(prev => {
        switch (event.type) {
          case 'agent_start':
            return {
              currentAgent: event.agentId,
              currentAgentName: event.agentName,
              steps: [],
              streamingText: '',
            };
          case 'agent_done':
            return { ...prev, currentAgent: null, currentAgentName: null, steps: [], streamingText: '' };
          case 'tool_call':
            return {
              ...prev,
              steps: [...prev.steps, { type: 'tool_call', detail: event.detail || '', done: false }],
            };
          case 'tool_result':
            const updatedSteps = prev.steps.map(s =>
              s.type === 'tool_call' && s.detail === event.detail ? { ...s, done: true } : s
            );
            return { ...prev, steps: updatedSteps };
          case 'streaming':
            return { ...prev, streamingText: prev.streamingText + (event.content || '') };
          case 'meeting_interrupted':
            return prev; // 状态在外部处理
          default:
            return prev;
        }
      });

      // meeting_interrupted 事件：停止会议进行状态（失败消息卡片内联按钮处理重试/放弃）
      if (event.type === 'meeting_interrupted') {
        setSimulatingMap(prev => ({ ...prev, [stockCode]: false }));
      }
    });

    return () => {
      EventsOff(eventName);
      if (cleanup) cleanup();
    };
  }, [session?.stockCode]);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  const handleSendMessage = async (
    query: string,
    mentions: string[],
    replyTo: ChatMessage | null
  ) => {
    if (!session || !query.trim()) return;

    const stockCode = session.stockCode;

    // 重置取消标识
    meetingCancelledRef.current[stockCode] = false;
    setSimulatingMap(prev => ({ ...prev, [stockCode]: true }));

    // 添加用户消息用于即时显示
    const userMsg: ChatMessage = {
      id: `user-${Date.now()}`,
      agentId: 'user',
      agentName: '老韭菜',
      role: '',
      content: query,
      timestamp: Date.now(),
      replyTo: replyTo?.id,
      mentions: mentions
    };
    const messagesWithUser = [...messages, userMsg];
    setMessages(messagesWithUser);

    try {
      // 使用会议室API
      const req: MeetingMessageRequest = {
        stockCode: session.stockCode,
        content: query,
        mentionIds: mentions,
        replyToId: replyTo?.id || '',
        replyContent: replyTo?.content || ''
      };

      // 统一模式：无论智能模式还是直接@模式，消息都通过事件实时推送
      await sendMeetingMessage(req);
      // 消息已通过事件实时添加，更新session
      onSessionUpdate({
        ...session,
        messages: [] // 会在事件中更新
      });
    } catch (e) {
      console.error('[AgentRoom] sendMeetingMessage error:', e);
      // 解析错误信息并显示给用户
      let errorMsg = '会议发起失败，请稍后重试';
      if (e instanceof Error) {
        if (e.message.includes('timeout') || e.message.includes('超时')) {
          errorMsg = '会议响应超时，请稍后重试';
        } else if (e.message.includes('AI') || e.message.includes('config')) {
          errorMsg = '未配置 AI 服务，请先在设置中配置';
        } else if (e.message.includes('network') || e.message.includes('fetch')) {
          errorMsg = '网络连接失败，请检查网络';
        }
      }
      addSystemMessage(errorMsg);
      // 超时或失败时记录用户消息ID，显示重试/编辑按钮
      setFailedUserMsgId(userMsg.id);
    } finally {
      setSimulatingMap(prev => ({ ...prev, [stockCode]: false }));
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!userQuery.trim() || isSimulating) return;
    // 允许不@任何人（智能模式）

    // 保存当前状态用于发送
    const queryToSend = userQuery;
    const mentionsToSend = [...mentionedAgents];
    const replyToSend = replyToMessage;

    // 立即清空输入和@状态
    setUserQuery('');
    clearMentions();
    setReplyToMessage(null);
    closePicker();

    handleSendMessage(queryToSend, mentionsToSend, replyToSend);
  }

  // 处理输入变化，检测@符号
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    const cursorPos = e.target.selectionStart || 0;
    setUserQuery(value);
    handleMentionInput(value, cursorPos);
  };

  // 选择@韭菜（包装 Hook 方法）
  const onSelectMention = (agent: AgentConfig) => {
    const newQuery = handleSelectMention(agent, userQuery);
    setUserQuery(newQuery);
    inputRef.current?.focus();
  };

  // 处理键盘事件
  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    // 先让 Hook 处理 @ 选择器的键盘事件
    if (handleMentionKeyDown(e)) {
      return;
    }
    // Enter 键选择当前高亮的韭菜
    if (showMentionPicker && filteredAgents.length > 0 && e.key === 'Enter') {
      e.preventDefault();
      onSelectMention(filteredAgents[mentionSelectedIndex]);
      return;
    }
  };

  // 设置引用消息
  const handleReplyTo = (msg: ChatMessage) => {
    setReplyToMessage(msg);
  };

  // 取消引用
  const clearReplyTo = () => {
    setReplyToMessage(null);
  };

  // 复制消息内容
  const handleCopy = async (msgId: string, content: string) => {
    try {
      await navigator.clipboard.writeText(content);
      setCopiedId(msgId);
      setTimeout(() => setCopiedId(null), 2000);
    } catch (err) {
      addSystemMessage('复制失败');
    }
  };

  // 重试发送消息
  const handleRetry = (msg: ChatMessage) => {
    setFailedUserMsgId(null);
    handleSendMessage(msg.content, msg.mentions || [], null);
  };

  // 编辑消息
  const handleEdit = (msg: ChatMessage) => {
    setUserQuery(msg.content);
    setFailedUserMsgId(null);
    inputRef.current?.focus();
  };

  // 重试失败专家（根据 meetingMode 区分行为）
  const handleRetryAgent = async (msg: ChatMessage) => {
    if (!session || retryingAgentId) return;
    const stockCode = session.stockCode;

    setRetryingAgentId(msg.agentId);
    // 移除失败的消息
    setMessages(prev => prev.filter(m => m.id !== msg.id));
    setSimulatingMap(prev => ({ ...prev, [stockCode]: true }));

    try {
      if (msg.meetingMode === 'smart') {
        // 串行模式：重试并继续剩余专家
        await retryAgentAndContinue(stockCode);
      } else {
        // 独立模式：仅重试该专家
        const lastUserMsg = [...messages].reverse().find(m => m.agentId === 'user');
        const query = lastUserMsg?.content || '';
        await retryAgent(stockCode, msg.agentId, query);
      }
    } catch (e) {
      console.error('[AgentRoom] retryAgent error:', e);
      addSystemMessage(`${msg.agentName} 重试失败`);
    } finally {
      setRetryingAgentId(null);
      setSimulatingMap(prev => ({ ...prev, [stockCode]: false }));
    }
  };

  // 放弃中断的会议（串行模式下用户放弃剩余专家）
  const handleAbandonMeeting = async (msg: ChatMessage) => {
    if (!session) return;
    try {
      await cancelInterruptedMeeting(session.stockCode);
    } catch (e) {
      console.error('[AgentRoom] cancelInterruptedMeeting error:', e);
    }
    // 移除失败消息
    setMessages(prev => prev.filter(m => m.id !== msg.id));
    addSystemMessage('已放弃剩余专家讨论');
  };

  // 显示清空确认弹窗
  const handleClearMessages = () => {
    if (!session || isSimulating) return;
    setShowClearConfirm(true);
  };

  // 确认清空消息
  const confirmClearMessages = async () => {
    if (!session) return;
    setShowClearConfirm(false);
    const result = await clearSessionMessages(session.stockCode);
    if (result === 'success') {
      setMessages([]);
      onSessionUpdate({
        ...session,
        messages: []
      });
    }
  };

  return (
    <div className="relative flex flex-col h-full">
      {/* Header */}
      <div className="p-4 border-b fin-divider-soft">
        <div className="flex items-center justify-between">
          <h2 className={`text-lg font-bold flex items-center gap-2 ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>
            <Users style={{ color: 'var(--accent)' }} />
            韭菜讨论中心
          </h2>
          <button
            onClick={handleClearMessages}
            disabled={isSimulating || messages.length === 0}
            className={`p-1.5 rounded transition-colors disabled:opacity-30 disabled:cursor-not-allowed ${colors.isDark ? 'text-slate-400 hover:text-red-400 hover:bg-slate-800' : 'text-slate-500 hover:text-red-500 hover:bg-slate-200'}`}
            title="清空聊天记录"
          >
            <Trash2 size={16} />
          </button>
        </div>
        <p className={`text-xs mt-1 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>@韭菜提问，引用观点深入讨论</p>
      </div>

      {/* Chat Area */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4 fin-scrollbar" ref={scrollRef}>
        {messages.length === 0 && (
          <div className={`h-full flex flex-col items-center justify-center text-sm p-8 text-center opacity-60 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
            <MessageSquare size={32} className="mb-2" />
            <p>直接提问或 @ 选择韭菜专家</p>
            <p className={`text-xs mt-1 ${colors.isDark ? 'text-slate-600' : 'text-slate-400'}`}>不@任何人时，小韭菜会自动安排韭菜专家讨论</p>
          </div>
        )}
        
        {messages.map((msg) => {
          const isSystem = msg.agentId === 'system';
          const isUser = msg.agentId === 'user';
          const agent = allAgents.find(a => a.id === msg.agentId);
          
          if (isSystem) {
            return (
               <div key={msg.id} className="flex justify-center my-2">
                 <span className={`text-xs fin-chip px-3 py-1 rounded-full border fin-divider ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
                   {msg.content}
                 </span>
               </div>
             )
          }

          if (isUser) {
            // 获取@的韭菜名称
            const mentionNames = (msg.mentions || [])
              .map(id => allAgents.find(a => a.id === id)?.name)
              .filter(Boolean);
            // 获取引用的消息
            const quotedMsg = msg.replyTo ? messages.find(m => m.id === msg.replyTo) : null;
            const displayName = msg.agentName || '老韭菜';

            return (
               <div key={msg.id} className="flex gap-3 justify-end animate-in fade-in slide-in-from-bottom-2 duration-300">
                 <div className="flex-1 text-right max-w-[85%]">
                    <div className="flex items-baseline gap-2 mb-1 justify-end">
                      <span className="text-xs font-bold text-accent-2">{displayName}</span>
                      {mentionNames.length > 0 && (
                        <span className={`text-[10px] ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
                          @{mentionNames.join(', ')}
                        </span>
                      )}
                    </div>
                    {/* 引用内容 */}
                    {quotedMsg && (
                      <div className={`inline-block text-left text-xs px-2 py-1 rounded mb-1 border-l-2 max-w-full ${colors.isDark ? 'text-slate-400 bg-slate-800/50 border-slate-500' : 'text-slate-500 bg-slate-200/50 border-slate-400'}`}>
                        <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>引用 {quotedMsg.agentName}：</span>
                        <span className="line-clamp-1">{quotedMsg.content}</span>
                      </div>
                    )}
                    <div className="inline-block text-left text-sm text-white bg-gradient-to-br from-[var(--accent)] to-[var(--accent-2)] p-3 rounded-2xl rounded-tr-none shadow-sm">
                      {msg.content}
                    </div>
                    {/* 失败时显示重试/编辑按钮 */}
                    {failedUserMsgId === msg.id && (
                      <div className="flex items-center gap-2 mt-2 justify-end">
                        <button
                          onClick={() => handleRetry(msg)}
                          className={`flex items-center gap-1 text-xs px-2 py-1 rounded transition-colors ${colors.isDark ? 'text-amber-400 hover:text-amber-300 bg-amber-500/10 hover:bg-amber-500/20' : 'text-amber-600 hover:text-amber-500 bg-amber-500/10 hover:bg-amber-500/20'}`}
                        >
                          <RotateCcw size={12} />
                          重试
                        </button>
                        <button
                          onClick={() => handleEdit(msg)}
                          className={`flex items-center gap-1 text-xs px-2 py-1 rounded transition-colors ${colors.isDark ? 'text-slate-400 hover:text-slate-300 bg-slate-500/10 hover:bg-slate-500/20' : 'text-slate-500 hover:text-slate-400 bg-slate-500/10 hover:bg-slate-500/20'}`}
                        >
                          <Pencil size={12} />
                          编辑
                        </button>
                      </div>
                    )}
                 </div>
                  <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold shrink-0 text-accent-2 border border-accent/30 ${colors.isDark ? 'bg-slate-900/60' : 'bg-slate-100'}`}>
                    <User size={16}/>
                  </div>
               </div>
            )
          }

          // 小韭菜消息（开场白/总结）
          const isModerator = msg.agentId === 'moderator';
          if (isModerator) {
            const isOpening = msg.msgType === 'opening';
            const isSummary = msg.msgType === 'summary';
            return (
              <div key={msg.id} className="flex gap-3 animate-in fade-in slide-in-from-bottom-2 duration-300 group">
                <div className="w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold shrink-0 bg-gradient-to-br from-amber-500 to-orange-500 text-white shadow-md ring-2 ring-slate-900">
                  <Users size={14} />
                </div>
                <div className="flex-1 max-w-[90%]">
                  <div className="flex items-baseline gap-2 mb-1">
                    <span className="text-xs font-bold text-amber-400">{msg.agentName}</span>
                    <span className={`text-[9px] border border-amber-500/30 px-1 rounded ${colors.isDark ? 'text-amber-500/70' : 'text-amber-600/70'}`}>
                      {isOpening ? '开场' : isSummary ? '总结' : msg.role}
                    </span>
                  </div>
                  <div className="relative">
                    <div className={`text-sm p-3 rounded-2xl rounded-tl-none leading-relaxed shadow-sm agent-message-content ${
                      isSummary
                        ? (colors.isDark ? 'bg-gradient-to-br from-amber-900/40 to-orange-900/30 border border-amber-500/30 text-amber-100' : 'bg-gradient-to-br from-amber-100 to-orange-100 border border-amber-400/30 text-amber-900')
                        : (colors.isDark ? 'bg-slate-800/70 border border-amber-500/20 text-slate-200' : 'bg-slate-100 border border-amber-400/20 text-slate-700')
                    }`}>
                      <NodeRenderer content={msg.content} />
                    </div>
                    {/* 复制按钮 */}
                    <button
                      onClick={() => handleCopy(msg.id, msg.content)}
                      className={`absolute -right-2 top-1 opacity-0 group-hover:opacity-100 transition-opacity p-1.5 rounded-full shadow-lg ${colors.isDark ? 'bg-slate-700 hover:bg-slate-600 text-slate-300' : 'bg-white hover:bg-slate-100 text-slate-500 border border-slate-200'}`}
                      title="复制"
                    >
                      {copiedId === msg.id ? <Check size={12} className="text-green-400" /> : <Copy size={12} />}
                    </button>
                  </div>
                </div>
              </div>
            );
          }

          return (
            <div key={msg.id} className={`flex gap-3 animate-in fade-in slide-in-from-bottom-2 duration-300 group`}>
              <div
                className="w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold shrink-0 text-white shadow-md ring-2 ring-slate-900"
                style={{ backgroundColor: msg.error ? '#ef4444' : (agent?.color || '#475569') }}
              >
                {msg.error ? <AlertCircle size={14} /> : (agent?.avatar || msg.agentName?.charAt(0))}
              </div>
              <div className="flex-1 max-w-[85%]">
                <div className="flex items-baseline gap-2 mb-1">
                  <span className={`text-xs font-bold ${msg.error ? 'text-red-400' : (colors.isDark ? 'text-slate-300' : 'text-slate-600')}`}>{msg.agentName || agent?.name}</span>
                  <span className={`text-[9px] uppercase border fin-divider px-1 rounded fin-chip ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>{msg.role || agent?.role}</span>
                  {msg.error && (
                    <span className="text-[9px] px-1 rounded bg-red-500/20 text-red-400 border border-red-500/30">失败</span>
                  )}
                </div>
                <div className="relative">
                  {msg.error ? (
                    <div className={`text-sm p-3 rounded-2xl rounded-tl-none leading-relaxed shadow-sm border ${colors.isDark ? 'bg-red-950/30 border-red-500/30 text-red-300' : 'bg-red-50 border-red-300 text-red-600'}`}>
                      <div className="flex items-center gap-2 mb-2">
                        <AlertCircle size={14} />
                        <span>分析失败</span>
                      </div>
                      <div className={`text-xs ${colors.isDark ? 'text-red-400/70' : 'text-red-500/70'}`}>{msg.error}</div>
                      <div className="flex items-center gap-2 mt-2">
                        <button
                          onClick={() => handleRetryAgent(msg)}
                          disabled={retryingAgentId === msg.agentId}
                          className={`flex items-center gap-1 text-xs px-3 py-1.5 rounded-lg transition-colors ${
                            retryingAgentId === msg.agentId
                              ? 'opacity-50 cursor-not-allowed'
                              : (colors.isDark ? 'text-amber-400 hover:text-amber-300 bg-amber-500/10 hover:bg-amber-500/20' : 'text-amber-600 hover:text-amber-500 bg-amber-500/10 hover:bg-amber-500/20')
                          }`}
                        >
                          {retryingAgentId === msg.agentId ? <Loader2 size={12} className="animate-spin" /> : <RotateCcw size={12} />}
                          {retryingAgentId === msg.agentId ? '重试中...' : (msg.meetingMode === 'smart' ? '重试并继续' : '重试')}
                        </button>
                        {msg.meetingMode === 'smart' && (
                          <button
                            onClick={() => handleAbandonMeeting(msg)}
                            className={`flex items-center gap-1 text-xs px-3 py-1.5 rounded-lg transition-colors ${colors.isDark ? 'text-slate-400 bg-slate-500/10 hover:bg-slate-500/20' : 'text-slate-500 bg-slate-500/10 hover:bg-slate-500/20'}`}
                          >
                            <X size={12} />
                            放弃剩余
                          </button>
                        )}
                      </div>
                    </div>
                  ) : (
                    <>
                      <div className={`text-sm p-3 rounded-2xl rounded-tl-none leading-relaxed shadow-sm agent-message-content ${colors.isDark ? 'text-slate-200 bg-slate-800/70 border border-slate-700/40' : 'text-slate-700 bg-white border border-slate-200'}`}>
                        <NodeRenderer content={msg.content} />
                      </div>
                      {/* 操作按钮组 */}
                      <div className="absolute -right-2 top-1 flex flex-col gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                        <button
                          onClick={() => handleCopy(msg.id, msg.content)}
                          className={`p-1.5 rounded-full shadow-lg ${colors.isDark ? 'bg-slate-700 hover:bg-slate-600 text-slate-300' : 'bg-white hover:bg-slate-100 text-slate-500 border border-slate-200'}`}
                          title="复制"
                        >
                          {copiedId === msg.id ? <Check size={12} className="text-green-400" /> : <Copy size={12} />}
                        </button>
                        <button
                          onClick={() => handleReplyTo(msg)}
                          disabled={isSimulating}
                          className={`p-1.5 rounded-full shadow-lg disabled:opacity-50 ${colors.isDark ? 'bg-slate-700 hover:bg-slate-600 text-slate-300' : 'bg-white hover:bg-slate-100 text-slate-500 border border-slate-200'}`}
                          title="引用回复"
                        >
                          <Reply size={12} />
                        </button>
                      </div>
                    </>
                  )}
                </div>
              </div>
            </div>
          );
        })}
        {/* 进度显示 */}
        {isSimulating && (
          <div className={`mx-4 p-3 fin-panel-soft rounded-xl border animate-in fade-in duration-300 ${colors.isDark ? 'border-slate-700/50' : 'border-slate-300/50'}`}>
            {progress.currentAgent ? (
              <div className="space-y-2">
                <div className="flex items-center gap-2">
                  <Loader2 className="animate-spin h-4 w-4 text-accent-2" />
                  <span className="text-sm text-accent-2 font-medium">{progress.currentAgentName}</span>
                  <span className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>正在分析...</span>
                </div>
                {progress.steps.length > 0 && (
                  <div className="pl-6 space-y-1">
                    {progress.steps.map((step, i) => (
                      <div key={i} className="flex items-center gap-2 text-xs">
                        {step.done ? (
                          <CheckCircle2 className="h-3 w-3 text-green-400" />
                        ) : (
                          <Wrench className="h-3 w-3 text-amber-400 animate-pulse" />
                        )}
                        <span className={step.done ? (colors.isDark ? 'text-slate-400' : 'text-slate-500') : 'text-amber-400'}>
                          {step.detail}
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ) : (
              <div className="flex items-center gap-2 justify-center">
                <Loader2 className="animate-spin h-3 w-3 text-accent-2" />
                <span className={`text-xs animate-pulse ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>会议进行中...</span>
              </div>
            )}
          </div>
        )}
      </div>
      <div className="p-3 border-t fin-divider-soft shrink-0">
        {/* 引用预览 */}
        {replyToMessage && (
          <div className={`flex items-center gap-2 mb-2 p-2 rounded-lg border-l-2 border-accent ${colors.isDark ? 'bg-slate-800/50' : 'bg-slate-100'}`}>
            <Reply size={12} className="text-accent-2 shrink-0" />
            <div className="flex-1 min-w-0">
              <span className="text-[10px] text-accent-2">引用 {replyToMessage.agentName}</span>
              <p className={`text-xs truncate ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>{replyToMessage.content}</p>
            </div>
            <button onClick={clearReplyTo} className={`p-1 ${colors.isDark ? 'text-slate-500 hover:text-slate-300' : 'text-slate-400 hover:text-slate-600'}`}>
              <X size={14} />
            </button>
          </div>
        )}

        {/* 已@韭菜标签 */}
        {mentionedAgents.length > 0 && (
          <div className="flex items-center gap-1 mb-2 flex-wrap">
            <span className={`text-[10px] ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>已@:</span>
            {mentionedAgents.map(id => {
              const agent = allAgents.find(a => a.id === id);
              return agent ? (
                <span
                  key={id}
                  className="flex items-center gap-1 px-2 py-0.5 bg-accent/20 text-accent-2 rounded text-[10px]"
                >
                  @{agent.name}
                  <button onClick={() => toggleMention(id)} className="hover:text-white">
                    <X size={10} />
                  </button>
                </span>
              ) : null;
            })}
          </div>
        )}

        {/* 输入框容器 */}
        <div className="relative">
          {/* @选择器下拉（输入@时显示） */}
          {showMentionPicker && filteredAgents.length > 0 && (
            <div className={`absolute bottom-full left-0 right-0 mb-2 backdrop-blur-sm rounded-xl border shadow-2xl z-10 overflow-hidden ${colors.isDark ? 'bg-slate-900/95 border-slate-700/50' : 'bg-white/95 border-slate-300/50'}`}>
              {/* 标题栏 */}
              <div className={`px-3 py-2 border-b ${colors.isDark ? 'border-slate-700/50 bg-slate-800/50' : 'border-slate-200 bg-slate-100/50'}`}>
                <div className="flex items-center justify-between">
                  <span className={`text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
                    {mentionSearchText ? `搜索: "${mentionSearchText}"` : '选择韭菜'}
                  </span>
                  <span className={`text-[10px] ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>↑↓ 选择 · Enter 确认</span>
                </div>
              </div>
              {/* 韭菜列表 */}
              <div ref={mentionListRef} className="max-h-40 overflow-y-auto py-1 fin-scrollbar">
                {filteredAgents.map((agent, index) => (
                  <button
                    key={agent.id}
                    onClick={() => onSelectMention(agent)}
                    className={`w-full flex items-center gap-3 px-3 py-2 text-left transition-colors ${
                      index === mentionSelectedIndex
                        ? 'bg-accent/20 text-white'
                        : (colors.isDark ? 'text-slate-300 hover:bg-slate-800' : 'text-slate-600 hover:bg-slate-100')
                    }`}
                  >
                    <span className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-medium ${agent.color} shadow-md`}>
                      {agent.avatar}
                    </span>
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium truncate">{agent.name}</div>
                      <div className={`text-[10px] truncate ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>{agent.role}</div>
                    </div>
                    {index === mentionSelectedIndex && (
                      <span className="text-accent-2 text-xs">⏎</span>
                    )}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* 输入框 */}
          <form onSubmit={handleSubmit} className="flex gap-2">
            <input
               ref={inputRef}
               type="text"
               value={userQuery}
               onChange={handleInputChange}
               onKeyDown={handleKeyDown}
               disabled={isSimulating}
               placeholder="直接提问或输入 @ 选择韭菜专家..."
               className="flex-1 fin-input rounded-lg px-4 py-2 text-sm placeholder-slate-500 border fin-divider"
            />
            {isSimulating ? (
              <button
                type="button"
                onClick={() => session?.stockCode && cancelMeeting(session.stockCode)}
                className="text-white p-2 rounded-lg transition-colors flex items-center justify-center w-10 h-10 bg-red-500 hover:bg-red-400"
                title="停止讨论"
              >
                <Square size={14} fill="currentColor" />
              </button>
            ) : (
              <button
                type="submit"
                disabled={!userQuery.trim()}
                className="text-white p-2 rounded-lg transition-colors flex items-center justify-center w-10 h-10 disabled:opacity-50"
                style={{ background: !userQuery.trim() ? '#334155' : `linear-gradient(to bottom right, var(--accent), var(--accent-2))` }}
              >
                <Send size={18} />
              </button>
            )}
          </form>
        </div>
        <div className="mt-1 text-center">
          <span className={`text-[10px] ${colors.isDark ? 'text-slate-600' : 'text-slate-400'}`}>直接提问由小韭菜安排韭菜专家，@ 可指定韭菜专家</span>
        </div>
      </div>

      {/* 清空确认弹窗 */}
      {showClearConfirm && (
        <div className="absolute inset-0 bg-black/50 flex items-center justify-center z-50 backdrop-blur-sm rounded-lg">
          <div className={`fin-panel border fin-divider rounded-xl p-5 w-72 shadow-2xl animate-in fade-in zoom-in-95 duration-200`}>
            <div className="flex items-center gap-3 mb-3">
              <div className="w-10 h-10 rounded-full bg-red-500/20 flex items-center justify-center">
                <Trash2 className="h-5 w-5 text-red-400" />
              </div>
              <h3 className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>清空聊天记录</h3>
            </div>
            <p className={`text-sm mb-5 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>确定要清空所有聊天记录吗？此操作无法撤销。</p>
            <div className="flex gap-2 justify-end">
              <button
                onClick={() => setShowClearConfirm(false)}
                className={`px-4 py-2 text-sm transition-colors rounded-lg ${colors.isDark ? 'text-slate-400 hover:text-white hover:bg-slate-700/60' : 'text-slate-500 hover:text-slate-700 hover:bg-slate-200/60'}`}
              >
                取消
              </button>
              <button
                onClick={confirmClearMessages}
                className="px-4 py-2 bg-red-500 hover:bg-red-400 text-white rounded-lg text-sm transition-colors"
              >
                确认清空
              </button>
            </div>
          </div>
        </div>
      )}

    </div>
  );
};
