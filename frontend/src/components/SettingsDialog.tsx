import React, { useState, useEffect, useCallback, useRef } from 'react';
import { X, Cpu, ChevronLeft, Plug, Plus, Trash2, Wrench, Check, Loader2, Brain, RefreshCw, Download, RotateCcw, Globe, Layers, Sliders, Star, MessageSquare, Copy, Sparkles } from 'lucide-react';
import { getConfig, updateConfig, getAvailableTools, ToolInfo, testAIConnection } from '../services/configService';
import { getAgentConfigs } from '../services/strategyService';
import { getMCPServers, MCPServerConfig, MCPServerStatus, testMCPConnection, getMCPServerTools, MCPToolInfo } from '../services/mcpService';
import { checkForUpdate, doUpdate, restartApp, getCurrentVersion, onUpdateProgress, UpdateInfo, UpdateProgress } from '../services/updateService';
import { getStrategies, getActiveStrategyID, setActiveStrategy, deleteStrategy, generateStrategy, updateStrategy, enhancePrompt, Strategy, StrategyAgent } from '../services/strategyService';
import { useTheme } from '../contexts/ThemeContext';

interface AIConfig {
  id: string;
  name: string;
  provider: string;
  baseUrl: string;
  apiKey: string;
  modelName: string;
  maxTokens: number;
  temperature: number;
  timeout: number;
  isDefault: boolean;
  // OpenAI Responses API 开关
  useResponses: boolean;
  // Vertex AI 专用字段
  project: string;
  location: string;
  credentialsJson: string;
}

interface MemoryConfig {
  enabled: boolean;
  aiConfigId: string;
  maxRecentRounds: number;
  maxKeyFacts: number;
  maxSummaryLength: number;
  compressThreshold: number;
}

// 代理模式类型
type ProxyMode = 'none' | 'system' | 'custom';

// 代理配置接口
interface ProxyConfig {
  mode: ProxyMode;
  customUrl: string;
}

// OpenClaw 配置接口
interface OpenClawConfig {
  enabled: boolean;
  port: number;
  apiKey: string;
}

type TabType = 'provider' | 'intent' | 'strategy' | 'mcp' | 'memory' | 'proxy' | 'openclaw' | 'update';

interface SettingsDialogProps {
  isOpen: boolean;
  onClose: () => void;
}

// Toast 通知 hook
interface ToastState {
  show: boolean;
  type: 'success' | 'error' | 'loading';
  message: string;
}

const useSettingsToast = () => {
  const [toast, setToast] = useState<ToastState>({ show: false, type: 'success', message: '' });

  const showToast = useCallback((type: ToastState['type'], message: string) => {
    setToast({ show: true, type, message });
    if (type !== 'loading') {
      setTimeout(() => setToast(prev => ({ ...prev, show: false })), 2000);
    }
  }, []);

  const hideToast = useCallback(() => {
    setToast(prev => ({ ...prev, show: false }));
  }, []);

  return { toast, showToast, hideToast };
};

export const SettingsDialog: React.FC<SettingsDialogProps> = ({ isOpen, onClose }) => {
  const { colors } = useTheme();
  const [activeTab, setActiveTab] = useState<TabType>('provider');
  const [aiConfigs, setAiConfigs] = useState<AIConfig[]>([]);
  const [mcpServers, setMcpServers] = useState<MCPServerConfig[]>([]);
  const [mcpStatus, setMcpStatus] = useState<Record<string, MCPServerStatus>>({});
  const [mcpTools, setMcpTools] = useState<Record<string, MCPToolInfo[]>>({});
  const [selectedMCP, setSelectedMCP] = useState<MCPServerConfig | null>(null);
  const [memoryConfig, setMemoryConfig] = useState<MemoryConfig>({
    enabled: true,
    aiConfigId: '',
    maxRecentRounds: 3,
    maxKeyFacts: 20,
    maxSummaryLength: 300,
    compressThreshold: 5,
  });
  const [proxyConfig, setProxyConfig] = useState<ProxyConfig>({
    mode: 'none',
    customUrl: '',
  });
  const [openClawConfig, setOpenClawConfig] = useState<OpenClawConfig>({
    enabled: false,
    port: 51888,
    apiKey: '',
  });
  const [strategies, setStrategies] = useState<Strategy[]>([]);
  const [activeStrategyId, setActiveStrategyId] = useState<string>('');
  const [moderatorAiId, setModeratorAiId] = useState<string>('');
  const [strategyAiId, setStrategyAiId] = useState<string>('');

  // Toast 通知
  const { toast, showToast, hideToast } = useSettingsToast();

  useEffect(() => {
    if (isOpen) {
      loadAllConfigs();
    }
  }, [isOpen]);

  const loadAllConfigs = async () => {
    const config = await getConfig();
    setAiConfigs(config.aiConfigs || []);
    const mcps = await getMCPServers();
    setMcpServers(mcps || []);
    if (config.memory) setMemoryConfig(config.memory);
    if (config.proxy) {
      setProxyConfig({
        mode: config.proxy.mode as ProxyMode,
        customUrl: config.proxy.customUrl || '',
      });
    }
    if (config.openClaw) {
      setOpenClawConfig({
        enabled: config.openClaw.enabled || false,
        port: config.openClaw.port || 8080,
        apiKey: config.openClaw.apiKey || '',
      });
    }
    if (config.moderatorAiId) setModeratorAiId(config.moderatorAiId);
    if (config.strategyAiId) setStrategyAiId(config.strategyAiId);

    // 加载策略配置
    const loadedStrategies = await getStrategies();
    setStrategies(loadedStrategies || []);
    const activeId = await getActiveStrategyID();
    setActiveStrategyId(activeId);

    // 自动检测已启用的 MCP 服务器状态
    const enabledMcps = (mcps || []).filter(m => m.enabled);
    for (const mcp of enabledMcps) {
      testMCPConnection(mcp.id).then(status => {
        setMcpStatus(prev => ({ ...prev, [mcp.id]: status }));
        if (status.connected) {
          getMCPServerTools(mcp.id).then(tools => {
            setMcpTools(prev => ({ ...prev, [mcp.id]: tools || [] }));
          });
        }
      });
    }
  };

  // 防抖保存的 ref
  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingUpdatesRef = useRef<Partial<{
    aiConfigs: AIConfig[];
    mcpServers: MCPServerConfig[];
    memory: MemoryConfig;
    proxy: ProxyConfig;
    moderatorAiId: string;
    strategyAiId: string;
  }>>({});

  // 实际执行保存的函数
  const doSave = useCallback(async () => {
    const updates = pendingUpdatesRef.current;
    if (Object.keys(updates).length === 0) return;

    showToast('loading', '保存中...');
    try {
      const currentConfig = await getConfig();
      await updateConfig({
        ...currentConfig,
        ...updates,
        defaultAiId: (updates.aiConfigs || currentConfig.aiConfigs)?.find(c => c.isDefault)?.id || '',
      } as any);
      pendingUpdatesRef.current = {};
      hideToast();
      showToast('success', '已保存');
    } catch (e) {
      hideToast();
      showToast('error', '保存失败');
    }
  }, [showToast, hideToast]);

  // 防抖保存配置（延迟 500ms）
  const saveConfig = useCallback((updates: Partial<{
    aiConfigs: AIConfig[];
    mcpServers: MCPServerConfig[];
    memory: MemoryConfig;
    proxy: ProxyConfig;
    openClaw: OpenClawConfig;
    moderatorAiId: string;
    strategyAiId: string;
  }>) => {
    // 合并待保存的更新
    pendingUpdatesRef.current = { ...pendingUpdatesRef.current, ...updates };

    // 清除之前的定时器
    if (saveTimerRef.current) {
      clearTimeout(saveTimerRef.current);
    }

    // 设置新的定时器
    saveTimerRef.current = setTimeout(() => {
      doSave();
      saveTimerRef.current = null;
    }, 500);
  }, [doSave]);

  // 组件卸载时清理定时器并保存未保存的更改
  useEffect(() => {
    return () => {
      if (saveTimerRef.current) {
        clearTimeout(saveTimerRef.current);
        doSave();
      }
    };
  }, [doSave]);

  if (!isOpen) return null;

  const tabs: { id: TabType; label: string; icon: React.ReactNode }[] = [
    { id: 'provider', label: '模型基座', icon: <Cpu className="h-4 w-4" /> },
    { id: 'intent', label: '意图配置', icon: <MessageSquare className="h-4 w-4" /> },
    { id: 'strategy', label: '策略管理', icon: <Layers className="h-4 w-4" /> },
    { id: 'mcp', label: 'MCP服务', icon: <Plug className="h-4 w-4" /> },
    { id: 'memory', label: '记忆管理', icon: <Brain className="h-4 w-4" /> },
    { id: 'proxy', label: '网络代理', icon: <Globe className="h-4 w-4" /> },
    { id: 'openclaw', label: 'OpenClaw', icon: <Plug className="h-4 w-4" /> },
    { id: 'update', label: '软件更新', icon: <RefreshCw className="h-4 w-4" /> },
  ];

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50 backdrop-blur-sm">
      <div className="fin-panel border fin-divider rounded-xl w-[720px] max-h-[85vh] overflow-hidden shadow-2xl">
        <Header onClose={onClose} />
        <div className="flex h-[500px]">
          {/* 左侧选项卡 */}
          <div className="w-44 fin-panel-strong border-r fin-divider p-2">
            {tabs.map(tab => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm mb-1 transition-all ${
                  activeTab === tab.id
                    ? 'bg-gradient-to-br from-[var(--accent)] to-[var(--accent-2)] text-white'
                    : (colors.isDark ? 'text-slate-400 hover:bg-slate-800/60 hover:text-white' : 'text-slate-500 hover:bg-slate-200/60 hover:text-slate-800')
                }`}
              >
                {tab.icon}
                {tab.label}
              </button>
            ))}
          </div>
          {/* 右侧内容 */}
          <div className="flex-1 overflow-y-auto p-4 fin-scrollbar">
            {activeTab === 'provider' && (
              <ProviderSettings
                configs={aiConfigs}
                onChange={(configs) => {
                  setAiConfigs(configs);
                  saveConfig({ aiConfigs: configs });
                }}
                moderatorAiId={moderatorAiId}
                strategyAiId={strategyAiId}
                strategies={strategies}
                memoryAiId={memoryConfig.aiConfigId}
              />
            )}
            {activeTab === 'intent' && (
              <IntentSettings
                configs={aiConfigs}
                moderatorAiId={moderatorAiId}
                onModeratorAiIdChange={(id) => {
                  setModeratorAiId(id);
                  saveConfig({ moderatorAiId: id });
                }}
              />
            )}
            {activeTab === 'strategy' && (
              <StrategySettings
                strategies={strategies}
                activeStrategyId={activeStrategyId}
                strategyAiId={strategyAiId}
                onStrategiesChange={setStrategies}
                onActiveChange={setActiveStrategyId}
                onStrategyAiIdChange={(id) => {
                  setStrategyAiId(id);
                  saveConfig({ strategyAiId: id });
                }}
                onAgentsReload={async () => {
                  await getAgentConfigs();
                }}
                mcpServers={mcpServers}
                aiConfigs={aiConfigs}
                showToast={showToast}
              />
            )}
            {activeTab === 'mcp' && (
              <MCPSettings
                servers={mcpServers}
                mcpStatus={mcpStatus}
                mcpTools={mcpTools}
                selectedMCP={selectedMCP}
                onSelectMCP={setSelectedMCP}
                onServersChange={(servers) => {
                  setMcpServers(servers);
                  saveConfig({ mcpServers: servers });
                }}
                onTestConnection={async (id) => {
                  const status = await testMCPConnection(id);
                  setMcpStatus(prev => ({ ...prev, [id]: status }));
                  if (status.connected) {
                    const tools = await getMCPServerTools(id);
                    setMcpTools(prev => ({ ...prev, [id]: tools || [] }));
                  }
                  return status;
                }}
              />
            )}
            {activeTab === 'memory' && (
              <MemorySettings
                config={memoryConfig}
                aiConfigs={aiConfigs}
                onChange={(config) => {
                  setMemoryConfig(config);
                  saveConfig({ memory: config });
                }}
              />
            )}
            {activeTab === 'proxy' && (
              <ProxySettings
                config={proxyConfig}
                onChange={(config) => {
                  setProxyConfig(config);
                  saveConfig({ proxy: config });
                }}
              />
            )}
            {activeTab === 'openclaw' && (
              <OpenClawSettings
                config={openClawConfig}
                onChange={(config) => {
                  setOpenClawConfig(config);
                  saveConfig({ openClaw: config });
                }}
              />
            )}
            {activeTab === 'update' && (
              <UpdateSettings />
            )}
          </div>
        </div>
      </div>

      {/* Toast 通知 */}
      {toast.show && (
        <div className="fixed bottom-4 right-4 z-[100]">
          <div className={`flex items-center gap-2 px-4 py-2 rounded-lg border backdrop-blur-sm ${
            toast.type === 'success' ? 'bg-green-500/10 border-green-500/30' :
            toast.type === 'error' ? 'bg-red-500/10 border-red-500/30' :
            'bg-blue-500/10 border-blue-500/30'
          }`}>
            {toast.type === 'success' && <Check className="h-4 w-4 text-green-400" />}
            {toast.type === 'error' && <X className="h-4 w-4 text-red-400" />}
            {toast.type === 'loading' && <Loader2 className="h-4 w-4 text-blue-400 animate-spin" />}
            <span className="text-sm text-white">{toast.message}</span>
          </div>
        </div>
      )}
    </div>
  );
};

const Header: React.FC<{ onClose: () => void }> = ({ onClose }) => {
  const { colors } = useTheme();
  return (
    <div className="flex items-center justify-between px-5 py-4 border-b fin-divider fin-panel-strong">
      <h2 className={`text-lg font-semibold ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>设置</h2>
      <button onClick={onClose} className={`transition-colors p-1 rounded ${colors.isDark ? 'text-slate-500 hover:text-white hover:bg-slate-800/60' : 'text-slate-400 hover:text-slate-700 hover:bg-slate-200/60'}`}>
        <X className="h-5 w-5" />
      </button>
    </div>
  );
};

// ========== Provider 设置选项卡 ==========
const PROVIDERS = ['openai', 'gemini', 'vertexai', 'anthropic'] as const;
type ProviderType = typeof PROVIDERS[number];

const PROVIDER_LABELS: Record<ProviderType, string> = {
  openai: 'OpenAI',
  gemini: 'Gemini',
  vertexai: 'Vertex AI',
  anthropic: 'Anthropic',
};

interface ProviderSettingsProps {
  configs: AIConfig[];
  onChange: (configs: AIConfig[]) => void;
  moderatorAiId: string;
  strategyAiId: string;
  strategies: Strategy[];
  memoryAiId: string;
}

// 视图类型
type ProviderView = 'list' | 'edit';

const ProviderSettings: React.FC<ProviderSettingsProps> = ({ configs, onChange, moderatorAiId, strategyAiId, strategies, memoryAiId }) => {
  const [view, setView] = useState<ProviderView>('list');
  const [selectedConfig, setSelectedConfig] = useState<AIConfig | null>(null);
  const [showAddModal, setShowAddModal] = useState(false);
  const [newProviderType, setNewProviderType] = useState<ProviderType>('openai');

  // 添加新配置
  const handleAddConfig = () => {
    const newConfig: AIConfig = {
      id: `${newProviderType}-${Date.now()}`,
      name: `${PROVIDER_LABELS[newProviderType]} ${configs.filter(c => c.provider === newProviderType).length + 1}`,
      provider: newProviderType,
      baseUrl: getDefaultBaseUrl(newProviderType),
      apiKey: '',
      modelName: getDefaultModel(newProviderType),
      maxTokens: 2048,
      temperature: 0.7,
      timeout: 60,
      isDefault: configs.length === 0,
      useResponses: false,
      project: '',
      location: 'us-central1',
      credentialsJson: '',
    };
    onChange([...configs, newConfig]);
    setSelectedConfig(newConfig);
    setView('edit');
    setShowAddModal(false);
  };

  // 更新配置
  const handleUpdate = (updated: AIConfig) => {
    if (updated.isDefault) {
      onChange(configs.map(c => c.id === updated.id ? updated : { ...c, isDefault: false }));
    } else {
      onChange(configs.map(c => c.id === updated.id ? updated : c));
    }
    setSelectedConfig(updated);
  };

  // 删除配置
  const handleDelete = (id: string) => {
    const config = configs.find(c => c.id === id);
    if (config?.isDefault) return;
    onChange(configs.filter(c => c.id !== id));
    setView('list');
    setSelectedConfig(null);
  };

  // 设为默认
  const handleSetDefault = (id: string) => {
    onChange(configs.map(c => ({ ...c, isDefault: c.id === id })));
  };

  // 复制配置
  const handleCopy = (config: AIConfig) => {
    const newConfig: AIConfig = {
      ...config,
      id: `${config.provider}-${Date.now()}`,
      name: `${config.name} (副本)`,
      isDefault: false,
    };
    onChange([...configs, newConfig]);
    setSelectedConfig(newConfig);
    setView('edit');
  };

  // 获取删除禁用原因
  const getDeleteDisabledReason = (id: string): string | undefined => {
    const usages: string[] = [];
    if (moderatorAiId === id) usages.push('意图分析');
    if (strategyAiId === id) usages.push('策略生成');
    if (memoryAiId === id) usages.push('记忆功能');
    // 检查策略中的 agent 是否使用此配置
    for (const strategy of strategies) {
      for (const agent of strategy.agents || []) {
        if (agent.aiConfigId === id) {
          usages.push(`策略"${strategy.name}"的Agent"${agent.name}"`);
        }
      }
    }
    if (usages.length > 0) {
      return `正在被使用: ${usages.join(', ')}`;
    }
    return undefined;
  };

  // 编辑视图
  if (view === 'edit' && selectedConfig) {
    return (
      <ProviderEditView
        config={selectedConfig}
        onBack={() => { setView('list'); setSelectedConfig(null); }}
        onChange={handleUpdate}
        onDelete={() => handleDelete(selectedConfig.id)}
      />
    );
  }

  // 列表视图
  return (
    <ProviderListView
      configs={configs}
      onSelect={(config) => { setSelectedConfig(config); setView('edit'); }}
      onSetDefault={handleSetDefault}
      onDelete={handleDelete}
      onCopy={handleCopy}
      onAdd={() => setShowAddModal(true)}
      showAddModal={showAddModal}
      newProviderType={newProviderType}
      onSelectType={setNewProviderType}
      onConfirmAdd={handleAddConfig}
      onCancelAdd={() => setShowAddModal(false)}
      getDeleteDisabledReason={getDeleteDisabledReason}
    />
  );
};

// ========== Provider 列表视图 ==========
interface ProviderListViewProps {
  configs: AIConfig[];
  onSelect: (config: AIConfig) => void;
  onSetDefault: (id: string) => void;
  onDelete: (id: string) => void;
  onCopy: (config: AIConfig) => void;
  onAdd: () => void;
  showAddModal: boolean;
  newProviderType: ProviderType;
  onSelectType: (type: ProviderType) => void;
  onConfirmAdd: () => void;
  onCancelAdd: () => void;
  getDeleteDisabledReason: (id: string) => string | undefined;
}

const ProviderListView: React.FC<ProviderListViewProps> = ({
  configs, onSelect, onSetDefault, onDelete, onCopy, onAdd,
  showAddModal, newProviderType, onSelectType, onConfirmAdd, onCancelAdd, getDeleteDisabledReason
}) => {
  const { colors } = useTheme();
  const defaultCount = configs.filter(c => c.isDefault).length;

  return (
    <div className="space-y-4">
      {/* 头部 */}
      <div className="flex items-center justify-between">
        <div>
          <h3 className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>AI 模型配置</h3>
          <p className={`text-xs mt-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
            共 {configs.length} 个配置，{defaultCount} 个默认
          </p>
        </div>
        <button
          onClick={onAdd}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-gradient-to-br from-[var(--accent)] to-[var(--accent-2)] text-white rounded-lg"
        >
          <Plus className="h-3.5 w-3.5" />
          添加
        </button>
      </div>

      {/* 配置列表 */}
      <div className="space-y-2">
        {configs.length === 0 ? (
          <p className={`text-sm text-center py-8 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>暂无 AI 配置</p>
        ) : (
          configs.map(config => {
            const deleteReason = getDeleteDisabledReason(config.id);
            return (
              <ProviderListItem
                key={config.id}
                config={config}
                onSelect={() => onSelect(config)}
                onSetDefault={() => onSetDefault(config.id)}
                onDelete={() => onDelete(config.id)}
                onCopy={() => onCopy(config)}
                deleteDisabled={!!deleteReason}
                deleteDisabledReason={deleteReason}
              />
            );
          })
        )}
      </div>

      {/* 添加配置弹窗 */}
      {showAddModal && (
        <AddAIConfigModal
          selectedType={newProviderType}
          onSelectType={onSelectType}
          onConfirm={onConfirmAdd}
          onCancel={onCancelAdd}
        />
      )}
    </div>
  );
};

// ========== 添加 AI 配置弹窗 ==========
interface AddAIConfigModalProps {
  selectedType: ProviderType;
  onSelectType: (type: ProviderType) => void;
  onConfirm: () => void;
  onCancel: () => void;
}

const AddAIConfigModal: React.FC<AddAIConfigModalProps> = ({ selectedType, onSelectType, onConfirm, onCancel }) => {
  const { colors } = useTheme();
  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-[60] backdrop-blur-sm">
      <div className="fin-panel border fin-divider rounded-xl w-[360px] p-5 shadow-2xl">
        <h3 className={`text-lg font-semibold mb-4 ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>添加 AI 配置</h3>
        <div className="space-y-3 mb-5">
          <label className={`block text-sm mb-2 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>选择类型</label>
          <div className="flex gap-2">
            {PROVIDERS.map(p => (
              <button
                key={p}
                onClick={() => onSelectType(p)}
                className={`flex-1 px-2 py-2 text-sm rounded-lg transition-all whitespace-nowrap ${
                  selectedType === p
                    ? 'bg-gradient-to-br from-[var(--accent)] to-[var(--accent-2)] text-white'
                    : (colors.isDark ? 'fin-panel border fin-divider text-slate-400 hover:text-white' : 'fin-panel border fin-divider text-slate-500 hover:text-slate-800')
                }`}
              >
                {PROVIDER_LABELS[p]}
              </button>
            ))}
          </div>
        </div>
        <div className="flex justify-end gap-3">
          <button onClick={onCancel} className={`px-4 py-2 text-sm ${colors.isDark ? 'text-slate-400 hover:text-white' : 'text-slate-500 hover:text-slate-800'}`}>
            取消
          </button>
          <button
            onClick={onConfirm}
            className="px-4 py-2 bg-gradient-to-br from-[var(--accent)] to-[var(--accent-2)] text-white rounded-lg text-sm"
          >
            添加
          </button>
        </div>
      </div>
    </div>
  );
};

// ========== Provider 列表项 ==========
interface ProviderListItemProps {
  config: AIConfig;
  onSelect: () => void;
  onSetDefault: () => void;
  onDelete: () => void;
  onCopy: () => void;
  deleteDisabled?: boolean;
  deleteDisabledReason?: string;
}

const ProviderListItem: React.FC<ProviderListItemProps> = ({
  config, onSelect, onSetDefault, onDelete, onCopy, deleteDisabled, deleteDisabledReason
}) => {
  const { colors } = useTheme();
  return (
    <div
      onClick={onSelect}
      className={`p-3 rounded-lg border transition-all cursor-pointer ${
        config.isDefault
          ? 'border-accent/50 bg-accent/10'
          : (colors.isDark ? 'border-slate-700 hover:border-slate-600' : 'border-slate-300 hover:border-slate-400')
      }`}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-full flex items-center justify-center bg-blue-500/20 text-blue-400">
            <Cpu className="h-5 w-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className={`text-sm font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{config.name}</span>
              <span className={`text-xs px-1.5 py-0.5 fin-chip rounded ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
                {PROVIDER_LABELS[config.provider as ProviderType] || config.provider}
              </span>
              {config.isDefault && (
                <span className="text-xs px-1.5 py-0.5 bg-accent/20 text-accent-2 rounded">默认</span>
              )}
            </div>
            <p className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>{config.modelName}</p>
          </div>
        </div>
        <div className="flex items-center gap-1" onClick={e => e.stopPropagation()}>
          {/* 复制按钮 - 始终显示 */}
          <button
            onClick={onCopy}
            className={`p-1.5 rounded transition-colors ${colors.isDark ? 'text-slate-400 hover:text-blue-400 hover:bg-blue-500/20' : 'text-slate-500 hover:text-blue-500 hover:bg-blue-500/10'}`}
            title="复制配置"
          >
            <Copy className="h-4 w-4" />
          </button>
          {!config.isDefault && (
            <>
              <button
                onClick={onSetDefault}
                className={`p-1.5 rounded transition-colors ${colors.isDark ? 'text-slate-400 hover:text-yellow-400 hover:bg-yellow-500/20' : 'text-slate-500 hover:text-yellow-500 hover:bg-yellow-500/10'}`}
                title="设为默认"
              >
                <Star className="h-4 w-4" />
              </button>
              <button
                onClick={deleteDisabled ? undefined : onDelete}
                className={`p-1.5 rounded transition-colors ${
                  deleteDisabled
                    ? (colors.isDark ? 'text-slate-600 cursor-not-allowed' : 'text-slate-400 cursor-not-allowed')
                    : (colors.isDark ? 'text-slate-400 hover:text-red-400 hover:bg-red-500/20' : 'text-slate-500 hover:text-red-500 hover:bg-red-500/10')
                }`}
                title={deleteDisabled ? deleteDisabledReason : "删除"}
              >
                <Trash2 className="h-4 w-4" />
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
};

// ========== Provider 编辑视图 ==========
interface ProviderEditViewProps {
  config: AIConfig;
  onBack: () => void;
  onChange: (config: AIConfig) => void;
  onDelete: () => void;
}

const ProviderEditView: React.FC<ProviderEditViewProps> = ({
  config, onBack, onChange, onDelete
}) => {
  const { colors } = useTheme();
  const isVertexAI = config.provider === 'vertexai';
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; error?: string } | null>(null);

  const handleTestConnection = async () => {
    setTesting(true);
    setTestResult(null);
    try {
      const result = await testAIConnection(config as any);
      setTestResult(result === 'success' ? { success: true } : { success: false, error: result });
    } catch (e: any) {
      setTestResult({ success: false, error: e.message || '未知错误' });
    } finally {
      setTesting(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* 头部 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <button
            onClick={onBack}
            className={`p-1.5 rounded-lg transition-colors ${colors.isDark ? 'hover:bg-slate-700/60 text-slate-400 hover:text-white' : 'hover:bg-slate-200/60 text-slate-500 hover:text-slate-800'}`}
          >
            <ChevronLeft className="h-5 w-5" />
          </button>
          <div className="w-10 h-10 rounded-full flex items-center justify-center bg-blue-500/20 text-blue-400">
            <Cpu className="h-5 w-5" />
          </div>
          <div>
            <h3 className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{config.name}</h3>
            <p className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
              {PROVIDER_LABELS[config.provider as ProviderType] || config.provider}
              {config.isDefault && ' · 默认配置'}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={handleTestConnection}
            disabled={testing}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg disabled:opacity-50 transition-colors shrink-0 ${colors.isDark ? 'bg-slate-700 hover:bg-slate-600 text-slate-300' : 'bg-slate-200 hover:bg-slate-300 text-slate-600'}`}
          >
            {testing ? (
              <>
                <Loader2 className="h-3 w-3 animate-spin" />
                测试中...
              </>
            ) : (
              '测试连接'
            )}
          </button>
          {!config.isDefault && (
            <button
              onClick={onDelete}
              className={`p-2 rounded-lg transition-colors ${colors.isDark ? 'hover:bg-red-500/20 text-slate-400 hover:text-red-400' : 'hover:bg-red-500/10 text-slate-500 hover:text-red-500'}`}
            >
              <Trash2 className="h-4 w-4" />
            </button>
          )}
        </div>
      </div>

      {/* 测试结果反馈 */}
      {testResult && (
        <div className={`text-xs px-3 py-2 rounded-lg ${
          testResult.success
            ? 'bg-accent/10 text-accent-2'
            : 'bg-red-500/10 text-red-400'
        }`}>
          {testResult.success ? '连接成功' : (
            <span className="line-clamp-2">{testResult.error || '连接失败'}</span>
          )}
        </div>
      )}

      {/* 表单内容 */}
      <div className="space-y-4">
        <FormField label="配置名称" value={config.name} onChange={v => onChange({ ...config, name: v })} />

        {!isVertexAI && (
          <>
            <FormField label="Base URL" value={config.baseUrl} onChange={v => onChange({ ...config, baseUrl: v })} />
            <FormField label="API Key" value={config.apiKey} onChange={v => onChange({ ...config, apiKey: v })} type="password" />
          </>
        )}

        {config.provider === 'openai' && (
          <div className="flex items-center justify-between">
            <label className={`text-sm ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>使用 Responses API</label>
            <ToggleSwitch checked={config.useResponses} onChange={v => onChange({ ...config, useResponses: v })} />
          </div>
        )}

        {isVertexAI && (
          <>
            <FormField label="GCP 项目 ID" value={config.project || ''} onChange={v => onChange({ ...config, project: v })} />
            <FormField label="区域" value={config.location || ''} onChange={v => onChange({ ...config, location: v })} />
            <div>
              <label className={`block text-sm mb-1.5 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>服务账号证书 (JSON)</label>
              <textarea
                value={config.credentialsJson || ''}
                onChange={e => onChange({ ...config, credentialsJson: e.target.value })}
                rows={4}
                placeholder="粘贴服务账号 JSON 证书内容"
                className={`w-full fin-input rounded-lg px-3 py-2 text-sm resize-none font-mono ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
              />
            </div>
          </>
        )}

        <FormField label="模型名称" value={config.modelName} onChange={v => onChange({ ...config, modelName: v })} />

        {/* 温度配置 */}
        <div>
          <label className={`block text-sm mb-1.5 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
            温度 <span className={colors.isDark ? 'text-slate-500' : 'text-slate-400'}>({config.temperature})</span>
          </label>
          <input
            type="range"
            min="0"
            max="1"
            step="0.1"
            value={config.temperature}
            onChange={e => onChange({ ...config, temperature: parseFloat(e.target.value) })}
            className={`w-full h-2 rounded-lg appearance-none cursor-pointer accent-[var(--accent)] ${colors.isDark ? 'bg-slate-700' : 'bg-slate-300'}`}
          />
          <div className={`flex justify-between text-xs mt-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
            <span>精确 (0)</span>
            <span>创意 (1)</span>
          </div>
        </div>

        {/* Max Tokens 配置 */}
        <div>
          <label className={`block text-sm mb-1.5 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>最大输出 Token</label>
          <input
            type="number"
            min="0"
            max="128000"
            step="256"
            value={config.maxTokens}
            onChange={e => {
              const val = parseInt(e.target.value);
              onChange({ ...config, maxTokens: isNaN(val) ? 0 : val });
            }}
            className={`w-full fin-input rounded-lg px-3 py-2 text-sm ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
            placeholder="2048"
          />
          <p className={`text-xs mt-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>建议值：2048-8192，最大取决于模型,设置为0时表示不传递这个参数</p>
        </div>

      </div>
    </div>
  );
};

// ========== 开关组件 ==========
const ToggleSwitch: React.FC<{ checked: boolean; onChange: (v: boolean) => void }> = ({ checked, onChange }) => (
  <button
    type="button"
    onClick={() => onChange(!checked)}
    className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${
      checked ? 'bg-[var(--accent)]' : 'bg-slate-600'
    }`}
  >
    <span className={`inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform ${
      checked ? 'translate-x-[18px]' : 'translate-x-[3px]'
    }`} />
  </button>
);

// ========== 意图配置设置 ==========
interface IntentSettingsProps {
  configs: AIConfig[];
  moderatorAiId: string;
  onModeratorAiIdChange: (id: string) => void;
}

const IntentSettings: React.FC<IntentSettingsProps> = ({ configs, moderatorAiId, onModeratorAiIdChange }) => {
  const { colors } = useTheme();
  const selectedConfig = configs.find(c => c.id === moderatorAiId);
  const defaultConfig = configs.find(c => c.isDefault);

  return (
    <div className="space-y-6">
      <div>
        <h3 className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>意图分析配置</h3>
        <p className={`text-sm mt-1 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
          配置"小韭菜"使用的 AI 模型，用于分析用户意图和选择专家
        </p>
      </div>

      {/* 当前配置 */}
      <div className="fin-panel rounded-lg p-4 border fin-divider">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-10 h-10 rounded-full flex items-center justify-center bg-purple-500/20 text-purple-400">
            <MessageSquare className="h-5 w-5" />
          </div>
          <div>
            <div className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>小韭菜</div>
            <div className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>会议主持 · 意图分析</div>
          </div>
        </div>

        <div>
          <label className={`block text-sm mb-2 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>使用的 AI 模型</label>
          <select
            value={moderatorAiId}
            onChange={e => onModeratorAiIdChange(e.target.value)}
            className={`w-full fin-input rounded-lg px-3 py-2 text-sm ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
          >
            <option value="">使用默认配置 {defaultConfig ? `(${defaultConfig.name})` : ''}</option>
            {configs.map(config => (
              <option key={config.id} value={config.id}>
                {config.name} - {config.modelName}
              </option>
            ))}
          </select>
        </div>

        {/* 当前选择的配置信息 */}
        {(selectedConfig || defaultConfig) && (
          <div className="mt-4 pt-4 border-t fin-divider">
            <div className={`text-xs mb-2 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>当前配置详情</div>
            <div className="grid grid-cols-2 gap-2 text-sm">
              <div className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>模型</div>
              <div className={colors.isDark ? 'text-white' : 'text-slate-800'}>{(selectedConfig || defaultConfig)?.modelName}</div>
              <div className={colors.isDark ? 'text-slate-400' : 'text-slate-500'}>提供商</div>
              <div className={colors.isDark ? 'text-white' : 'text-slate-800'}>
                {PROVIDER_LABELS[(selectedConfig || defaultConfig)?.provider as ProviderType]}
              </div>
            </div>
          </div>
        )}
      </div>

      {/* 说明 */}
      <div className={`text-xs space-y-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
        <p>• 小韭菜负责分析用户问题的意图，并选择合适的专家进行回答</p>
        <p>• 建议使用响应较快的模型以减少等待时间</p>
        <p>• 留空则使用系统默认的 AI 配置</p>
      </div>
    </div>
  );
};

// ========== 表单组件 ==========
interface FormFieldProps {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
}

const FormField: React.FC<FormFieldProps> = ({ label, value, onChange, type = 'text' }) => {
  const { colors } = useTheme();
  return (
    <div>
      <label className={`block text-sm mb-1.5 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>{label}</label>
      <input
        type={type}
        value={value}
        onChange={e => onChange(e.target.value)}
        className={`w-full fin-input rounded-lg px-3 py-2 text-sm transition-colors ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
      />
    </div>
  );
};

// ========== Helper functions ==========
const getDefaultBaseUrl = (provider: string): string => {
  switch (provider) {
    case 'openai': return 'https://api.openai.com/v1';
    case 'gemini': return 'https://generativelanguage.googleapis.com';
    case 'anthropic': return 'https://api.anthropic.com';
    default: return '';
  }
};

const getDefaultModel = (provider: string): string => {
  switch (provider) {
    case 'openai': return 'gpt-5.2';
    case 'gemini': return 'gemini-2.5-flash';
    case 'vertexai': return 'gemini-2.5-flash';
    case 'anthropic': return 'claude-sonnet-4-20250514';
    default: return '';
  }
};

// ========== 记忆管理设置选项卡 ==========
interface MemorySettingsProps {
  config: MemoryConfig;
  aiConfigs: AIConfig[];
  onChange: (config: MemoryConfig) => void;
}

const MemorySettings: React.FC<MemorySettingsProps> = ({ config, aiConfigs, onChange }) => {
  const { colors } = useTheme();
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h3 className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>记忆管理</h3>
          <p className={`text-sm mt-1 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
            启用后，AI专家将记住之前的讨论内容，提供更连贯的分析
          </p>
        </div>
        <label className="relative inline-flex items-center cursor-pointer">
          <input
            type="checkbox"
            checked={config.enabled}
            onChange={(e) => onChange({ ...config, enabled: e.target.checked })}
            className="sr-only peer"
          />
          <div className={`w-11 h-6 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-accent ${colors.isDark ? 'bg-slate-700' : 'bg-slate-400'}`}></div>
        </label>
      </div>

      {config.enabled && (
        <div className={`space-y-4 pt-4 border-t ${colors.isDark ? 'border-slate-700' : 'border-slate-300'}`}>
          {/* LLM 选择 */}
          <div>
            <label className={`block text-sm mb-2 ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>
              摘要模型
              <span className={`ml-2 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>(用于生成记忆摘要)</span>
            </label>
            <select
              value={config.aiConfigId || ''}
              onChange={(e) => onChange({ ...config, aiConfigId: e.target.value })}
              className={`w-full fin-input rounded-lg px-3 py-2 text-sm ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
            >
              <option value="">使用默认模型</option>
              {aiConfigs.map(ai => (
                <option key={ai.id} value={ai.id}>
                  {ai.name} ({ai.provider}) - {ai.modelName}
                </option>
              ))}
            </select>
            <p className={`text-xs mt-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
              建议选择较快的模型以减少延迟，留空则使用会议默认模型
            </p>
          </div>

          <div>
            <label className={`block text-sm mb-2 ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>
              保留最近讨论轮次
              <span className={`ml-2 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>({config.maxRecentRounds}轮)</span>
            </label>
            <input
              type="range"
              min="1"
              max="10"
              value={config.maxRecentRounds}
              onChange={(e) => onChange({ ...config, maxRecentRounds: parseInt(e.target.value) })}
              className={`w-full h-2 rounded-lg appearance-none cursor-pointer accent-[var(--accent)] ${colors.isDark ? 'bg-slate-700' : 'bg-slate-300'}`}
            />
            <div className={`flex justify-between text-xs mt-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
              <span>1轮</span>
              <span>10轮</span>
            </div>
          </div>

          <div>
            <label className={`block text-sm mb-2 ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>
              触发压缩阈值
              <span className={`ml-2 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>({config.compressThreshold}轮)</span>
            </label>
            <input
              type="range"
              min="3"
              max="15"
              value={config.compressThreshold}
              onChange={(e) => onChange({ ...config, compressThreshold: parseInt(e.target.value) })}
              className={`w-full h-2 rounded-lg appearance-none cursor-pointer accent-[var(--accent)] ${colors.isDark ? 'bg-slate-700' : 'bg-slate-300'}`}
            />
            <p className={`text-xs mt-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
              超过此轮次后，旧讨论将被压缩为摘要
            </p>
          </div>

          <div>
            <label className={`block text-sm mb-2 ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>
              最大关键事实数
              <span className={`ml-2 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>({config.maxKeyFacts}条)</span>
            </label>
            <input
              type="range"
              min="5"
              max="50"
              step="5"
              value={config.maxKeyFacts}
              onChange={(e) => onChange({ ...config, maxKeyFacts: parseInt(e.target.value) })}
              className={`w-full h-2 rounded-lg appearance-none cursor-pointer accent-[var(--accent)] ${colors.isDark ? 'bg-slate-700' : 'bg-slate-300'}`}
            />
          </div>

          <div>
            <label className={`block text-sm mb-2 ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>
              摘要最大长度
              <span className={`ml-2 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>({config.maxSummaryLength}字)</span>
            </label>
            <input
              type="range"
              min="100"
              max="500"
              step="50"
              value={config.maxSummaryLength}
              onChange={(e) => onChange({ ...config, maxSummaryLength: parseInt(e.target.value) })}
              className={`w-full h-2 rounded-lg appearance-none cursor-pointer accent-[var(--accent)] ${colors.isDark ? 'bg-slate-700' : 'bg-slate-300'}`}
            />
          </div>
        </div>
      )}
    </div>
  );
};

// ========== MCP 设置选项卡 ==========
interface MCPSettingsProps {
  servers: MCPServerConfig[];
  mcpStatus: Record<string, MCPServerStatus>;
  mcpTools: Record<string, MCPToolInfo[]>;
  selectedMCP: MCPServerConfig | null;
  onSelectMCP: (mcp: MCPServerConfig | null) => void;
  onServersChange: (servers: MCPServerConfig[]) => void;
  onTestConnection: (id: string) => Promise<MCPServerStatus>;
}

const MCPSettings: React.FC<MCPSettingsProps> = ({
  servers, mcpStatus, mcpTools, selectedMCP, onSelectMCP, onServersChange, onTestConnection
}) => {
  const { colors } = useTheme();
  if (selectedMCP) {
    return (
      <MCPEditForm
        server={selectedMCP}
        status={mcpStatus[selectedMCP.id]}
        tools={mcpTools[selectedMCP.id] || []}
        onBack={() => onSelectMCP(null)}
        onChange={(updated) => {
          onServersChange(servers.map(s => s.id === updated.id ? updated : s));
          onSelectMCP(updated);
        }}
        onDelete={() => {
          onServersChange(servers.filter(s => s.id !== selectedMCP.id));
          onSelectMCP(null);
        }}
        onTestConnection={() => onTestConnection(selectedMCP.id)}
      />
    );
  }

  const handleAddNew = () => {
    const newServer: MCPServerConfig = {
      id: `mcp-${Date.now()}`,
      name: '新 MCP 服务',
      transportType: 'http',
      endpoint: '',
      command: '',
      args: [],
      toolFilter: [],
      enabled: true,
    };
    onServersChange([...servers, newServer]);
    onSelectMCP(newServer);
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between mb-3">
        <h3 className={`text-sm font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>MCP 服务器</h3>
        <button
          onClick={handleAddNew}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-gradient-to-br from-[var(--accent)] to-[var(--accent-2)] text-white rounded-lg "
        >
          <Plus className="h-3.5 w-3.5" />
          添加
        </button>
      </div>
      <p className={`text-xs mb-4 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>配置 MCP 服务器以扩展 Agent 能力</p>
      {servers.length === 0 ? (
        <p className={`text-sm text-center py-8 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>暂无 MCP 服务器配置</p>
      ) : (
        servers.map(server => (
          <MCPListItem
            key={server.id}
            server={server}
            status={mcpStatus[server.id]}
            toolCount={(mcpTools[server.id] || []).length}
            onClick={() => onSelectMCP(server)}
          />
        ))
      )}
    </div>
  );
};

const MCPListItem: React.FC<{
  server: MCPServerConfig;
  status?: MCPServerStatus;
  toolCount: number;
  onClick: () => void;
}> = ({ server, status, toolCount, onClick }) => {
  const { colors } = useTheme();
  // 状态指示器颜色
  const getStatusColor = () => {
    if (!server.enabled) return 'bg-slate-600';
    if (!status) return 'bg-yellow-500 animate-pulse'; // 检测中
    return status.connected ? 'bg-accent' : 'bg-red-500';
  };

  const getStatusText = () => {
    if (!server.enabled) return '已禁用';
    if (!status) return '检测中...';
    return status.connected ? '已连接' : status.error || '连接失败';
  };

  return (
    <div
      onClick={onClick}
      className={`flex items-center gap-3 p-3 fin-panel-soft rounded-lg transition-colors border fin-divider cursor-pointer ${colors.isDark ? 'hover:bg-slate-800/60' : 'hover:bg-slate-100/60'}`}
    >
      <div className="w-10 h-10 rounded-full flex items-center justify-center text-lg shrink-0 bg-purple-500/20 text-purple-400">
        <Plug className="h-5 w-5" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className={`text-sm font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{server.name}</span>
          <span className={`text-xs px-1.5 py-0.5 fin-chip rounded ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>{server.transportType}</span>
        </div>
        <p className={`text-xs truncate ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
          {server.transportType === 'command' ? server.command : server.endpoint}
        </p>
      </div>
      <div className="flex items-center gap-3">
        {/* 工具数量 */}
        {status?.connected && toolCount > 0 && (
          <span className={`text-xs flex items-center gap-1 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
            <Wrench className="h-3 w-3" />
            {toolCount}
          </span>
        )}
        <div className={`w-2 h-2 rounded-full ${getStatusColor()}`} title={getStatusText()} />
      </div>
    </div>
  );
};

// ========== MCP 编辑表单 ==========
interface MCPEditFormProps {
  server: MCPServerConfig;
  status?: MCPServerStatus;
  tools: MCPToolInfo[];
  onBack: () => void;
  onChange: (server: MCPServerConfig) => void;
  onDelete: () => void;
  onTestConnection: () => Promise<MCPServerStatus>;
}

const MCPEditForm: React.FC<MCPEditFormProps> = ({ server, status, tools, onBack, onChange, onDelete, onTestConnection }) => {
  const { colors } = useTheme();
  const [edited, setEdited] = useState<MCPServerConfig>(server);
  const [testing, setTesting] = useState(false);

  const handleChange = <K extends keyof MCPServerConfig>(field: K, value: MCPServerConfig[K]) => {
    const updated = { ...edited, [field]: value };
    setEdited(updated);
    onChange(updated);
  };

  return (
    <div className="space-y-4">
      {/* 头部 */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <button
            onClick={onBack}
            className={`p-1.5 rounded-lg transition-colors ${colors.isDark ? 'hover:bg-slate-700/60 text-slate-400 hover:text-white' : 'hover:bg-slate-200/60 text-slate-500 hover:text-slate-800'}`}
          >
            <ChevronLeft className="h-5 w-5" />
          </button>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full flex items-center justify-center bg-purple-500/20 text-purple-400">
              <Plug className="h-5 w-5" />
            </div>
            <h3 className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{edited.name}</h3>
          </div>
        </div>
        <button
          onClick={onDelete}
          className={`p-2 rounded-lg transition-colors ${colors.isDark ? 'hover:bg-red-500/20 text-slate-400 hover:text-red-400' : 'hover:bg-red-500/10 text-slate-500 hover:text-red-500'}`}
        >
          <Trash2 className="h-4 w-4" />
        </button>
      </div>

      {/* 名称 */}
      <FormField label="名称" value={edited.name} onChange={v => handleChange('name', v)} />

      {/* 传输类型 */}
      <div>
        <label className={`block text-sm mb-1.5 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>传输类型</label>
        <select
          value={edited.transportType}
          onChange={e => handleChange('transportType', e.target.value as MCPServerConfig['transportType'])}
          className={`w-full fin-input rounded-lg px-3 py-2 text-sm ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
        >
          <option value="http">HTTP (推荐)</option>
          <option value="sse">SSE</option>
          <option value="command">命令行</option>
        </select>
      </div>

      {/* 根据传输类型显示不同字段 */}
      {edited.transportType === 'command' ? (
        <>
          <FormField label="命令" value={edited.command} onChange={v => handleChange('command', v)} />
          <FormField
            label="参数 (逗号分隔)"
            value={edited.args.join(', ')}
            onChange={v => handleChange('args', v.split(',').map(s => s.trim()).filter(Boolean))}
          />
        </>
      ) : (
        <FormField label="端点 URL" value={edited.endpoint} onChange={v => handleChange('endpoint', v)} />
      )}

      {/* 启用状态 */}
      <div className="flex items-center justify-between pt-2">
        <span className={`text-sm ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>启用此服务</span>
        <button
          onClick={() => handleChange('enabled', !edited.enabled)}
          className={`w-11 h-6 rounded-full transition-colors ${
            edited.enabled ? 'bg-gradient-to-r from-[var(--accent)] to-[var(--accent-2)]' : (colors.isDark ? 'bg-slate-600' : 'bg-slate-400')
          }`}
        >
          <div className={`w-5 h-5 bg-white rounded-full shadow transition-transform ${
            edited.enabled ? 'translate-x-5' : 'translate-x-0.5'
          }`} />
        </button>
      </div>

      {/* 连接测试 */}
      <div className="pt-3 border-t fin-divider">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className={`text-sm ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>连接状态</span>
            {status && (
              <span className={`text-xs px-2 py-0.5 rounded ${
                status.connected
                  ? 'bg-accent/20 text-accent-2'
                  : 'bg-red-500/20 text-red-400'
              }`}>
                {status.connected ? '已连接' : status.error || '连接失败'}
              </span>
            )}
            {!status && edited.enabled && (
              <span className="text-xs px-2 py-0.5 rounded bg-yellow-500/20 text-yellow-400">
                未测试
              </span>
            )}
          </div>
          <button
            onClick={async () => {
              setTesting(true);
              await onTestConnection();
              setTesting(false);
            }}
            disabled={testing || !edited.enabled}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-lg disabled:opacity-50 transition-colors ${colors.isDark ? 'bg-slate-700 hover:bg-slate-600 text-slate-300' : 'bg-slate-200 hover:bg-slate-300 text-slate-600'}`}
          >
            {testing ? (
              <>
                <Loader2 className="h-3 w-3 animate-spin" />
                测试中...
              </>
            ) : (
              '测试连接'
            )}
          </button>
        </div>
      </div>

      {/* 工具列表 */}
      {status?.connected && tools.length > 0 && (
        <div className="pt-3 border-t fin-divider">
          <div className="flex items-center gap-2 mb-3">
            <Wrench className={`h-4 w-4 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`} />
            <span className={`text-sm ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>可用工具</span>
            <span className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>({tools.length})</span>
          </div>
          <div className="space-y-2 max-h-40 overflow-y-auto fin-scrollbar">
            {tools.map(tool => (
              <div key={tool.name} className={`p-2 rounded-lg border fin-divider ${colors.isDark ? 'bg-slate-800/40' : 'bg-slate-100/40'}`}>
                <div className={`text-xs font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{tool.name}</div>
                <div className={`text-xs mt-0.5 line-clamp-2 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>{tool.description}</div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

// ========== 代理设置选项卡 ==========
interface ProxySettingsProps {
  config: ProxyConfig;
  onChange: (config: ProxyConfig) => void;
}

const ProxySettings: React.FC<ProxySettingsProps> = ({ config, onChange }) => {
  const { colors } = useTheme();
  const proxyModes: { value: ProxyMode; label: string; desc: string }[] = [
    { value: 'none', label: '无代理', desc: '直接连接，不使用任何代理' },
    { value: 'system', label: '系统代理', desc: '使用操作系统的代理设置' },
    { value: 'custom', label: '自定义代理', desc: '手动指定代理服务器地址' },
  ];

  return (
    <div className="space-y-6">
      <div>
        <h3 className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>网络代理</h3>
        <p className={`text-sm mt-1 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
          配置应用的网络代理，用于访问 AI 服务和外部 API
        </p>
      </div>

      {/* 代理模式选择 */}
      <div className="space-y-3">
        {proxyModes.map(mode => (
          <label
            key={mode.value}
            className={`flex items-start gap-3 p-3 rounded-lg border cursor-pointer transition-all ${
              config.mode === mode.value
                ? 'border-[var(--accent)] bg-[var(--accent)]/10'
                : (colors.isDark ? 'border-slate-700 hover:border-slate-600' : 'border-slate-300 hover:border-slate-400')
            }`}
          >
            <input
              type="radio"
              name="proxyMode"
              value={mode.value}
              checked={config.mode === mode.value}
              onChange={() => onChange({ ...config, mode: mode.value })}
              className="mt-1 accent-[var(--accent)]"
            />
            <div>
              <div className={`text-sm font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{mode.label}</div>
              <div className={`text-xs mt-0.5 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>{mode.desc}</div>
            </div>
          </label>
        ))}
      </div>

      {/* 自定义代理地址输入 */}
      {config.mode === 'custom' && (
        <div className={`pt-4 border-t ${colors.isDark ? 'border-slate-700' : 'border-slate-300'}`}>
          <label className={`block text-sm mb-2 ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>
            代理服务器地址
          </label>
          <input
            type="text"
            value={config.customUrl}
            onChange={(e) => onChange({ ...config, customUrl: e.target.value })}
            placeholder="http://127.0.0.1:7890"
            className={`w-full fin-input rounded-lg px-3 py-2 text-sm ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
          />
          <p className={`text-xs mt-2 ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>
            支持 HTTP/HTTPS 代理，格式：http://host:port 或 http://user:pass@host:port
          </p>
        </div>
      )}
    </div>
  );
};

// ========== OpenClaw 设置选项卡 ==========
interface OpenClawSettingsProps {
  config: OpenClawConfig;
  onChange: (config: OpenClawConfig) => void;
}

const OpenClawSettings: React.FC<OpenClawSettingsProps> = ({ config, onChange }) => {
  const { colors } = useTheme();
  const [status, setStatus] = useState<{ running: boolean; port: number } | null>(null);
  const [switching, setSwitching] = useState(false);

  useEffect(() => {
    const fetchStatus = () => {
      // @ts-ignore
      window.go?.main?.App?.GetOpenClawStatus?.().then((s: any) => {
        setStatus(s);
        // 状态同步后结束切换中状态
        if (s && s.running === config.enabled) {
          setSwitching(false);
        }
      });
    };
    fetchStatus();
    // 仅在切换中时高频轮询，同步后停止
    if (switching) {
      const timer = setInterval(fetchStatus, 500);
      return () => clearInterval(timer);
    }
  }, [config.enabled, switching]);

  const handleToggle = () => {
    if (switching) return;
    setSwitching(true);
    onChange({ ...config, enabled: !config.enabled });
  };

  // 判断状态：切换中 or 已同步
  const isRunning = status?.running ?? false;
  const isSynced = isRunning === config.enabled;
  const statusText = switching || !isSynced
    ? (config.enabled ? '启动中...' : '关闭中...')
    : (isRunning ? `运行中 (端口 ${status?.port})` : '未运行');

  return (
    <div className="space-y-6">
      <div>
        <h3 className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>OpenClaw 服务</h3>
        <p className={`text-sm mt-1 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
          启用后可通过 HTTP API 供 OpenClaw 等 AI Agent 调用分析能力
        </p>
      </div>

      {/* 启用开关 - Switch 样式 */}
      <div className={`flex items-center justify-between p-3 rounded-lg border ${
        colors.isDark ? 'border-slate-700' : 'border-slate-300'
      }`}>
        <div>
          <div className={`text-sm font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>服务状态</div>
          <div className={`text-xs mt-0.5 ${
            switching || !isSynced ? 'text-yellow-400' : (isRunning ? 'text-green-400' : (colors.isDark ? 'text-slate-400' : 'text-slate-500'))
          }`}>
            {statusText}
          </div>
        </div>
        <button
          onClick={handleToggle}
          disabled={switching}
          className={`relative w-11 h-6 rounded-full transition-colors ${
            switching ? 'bg-yellow-500' : (config.enabled ? 'bg-[var(--accent)]' : (colors.isDark ? 'bg-slate-600' : 'bg-slate-300'))
          } ${switching ? 'cursor-wait' : 'cursor-pointer'}`}
        >
          <div className={`absolute top-1 w-4 h-4 rounded-full bg-white shadow transition-transform ${
            config.enabled ? 'translate-x-6' : 'translate-x-1'
          } ${switching ? 'animate-pulse' : ''}`} />
        </button>
      </div>

      {config.enabled && (
        <div className="space-y-4">
          {/* 端口 */}
          <div>
            <label className={`block text-sm mb-2 ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>端口</label>
            <input
              type="number"
              value={config.port}
              onChange={(e) => onChange({ ...config, port: parseInt(e.target.value) || 8080 })}
              className={`w-full fin-input rounded-lg px-3 py-2 text-sm ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
            />
          </div>
          {/* API Key */}
          <div>
            <label className={`block text-sm mb-2 ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>API Key (可选)</label>
            <input
              type="password"
              value={config.apiKey}
              onChange={(e) => onChange({ ...config, apiKey: e.target.value })}
              placeholder="留空则不鉴权"
              className={`w-full fin-input rounded-lg px-3 py-2 text-sm ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
            />
          </div>
        </div>
      )}
    </div>
  );
};

// ========== 更新设置选项卡 ==========
const UpdateSettings: React.FC = () => {
  const { colors } = useTheme();
  const [currentVersion, setCurrentVersion] = useState<string>('');
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [checking, setChecking] = useState(false);
  const [updating, setUpdating] = useState(false);
  const [progress, setProgress] = useState<UpdateProgress | null>(null);

  useEffect(() => {
    getCurrentVersion().then(setCurrentVersion);
    const cleanup = onUpdateProgress(setProgress);
    return cleanup;
  }, []);

  const handleCheckUpdate = async () => {
    setChecking(true);
    setUpdateInfo(null);
    try {
      const info = await checkForUpdate();
      setUpdateInfo(info);
    } finally {
      setChecking(false);
    }
  };

  const handleUpdate = async () => {
    setUpdating(true);
    setProgress(null);
    try {
      const result = await doUpdate();
      if (result !== 'success') {
        setProgress({ status: 'error', message: result, percent: 0 });
      }
    } catch (e) {
      setProgress({ status: 'error', message: String(e), percent: 0 });
    }
  };

  const handleRestart = async () => {
    await restartApp();
  };

  return (
    <div className="space-y-6">
      <div>
        <h3 className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>软件更新</h3>
        <p className={`text-sm mt-1 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>检查并安装最新版本</p>
      </div>

      <div className="fin-panel rounded-lg p-4 border fin-divider">
        <div className="flex items-center justify-between">
          <div>
            <span className={`text-sm ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>当前版本</span>
            <p className={`font-medium mt-1 ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>v{currentVersion || '...'}</p>
          </div>
          <button
            onClick={handleCheckUpdate}
            disabled={checking || updating}
            className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm disabled:opacity-50 transition-colors ${colors.isDark ? 'bg-slate-700 hover:bg-slate-600 text-white' : 'bg-slate-200 hover:bg-slate-300 text-slate-700'}`}
          >
            {checking ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            {checking ? '检查中...' : '检查更新'}
          </button>
        </div>
      </div>

      {updateInfo && (
        <div className={`fin-panel rounded-lg p-4 border ${updateInfo.hasUpdate ? 'border-accent/50 bg-accent/5' : 'fin-divider'}`}>
          {updateInfo.error ? (
            <div className="text-red-400 text-sm">{updateInfo.error}</div>
          ) : updateInfo.hasUpdate ? (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <div>
                  <span className="text-accent-2 text-sm font-medium">发现新版本</span>
                  <p className={`font-medium mt-1 ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>v{updateInfo.latestVersion}</p>
                </div>
                <button onClick={handleUpdate} disabled={updating}
                  className="flex items-center gap-2 px-4 py-2 bg-gradient-to-br from-[var(--accent)] to-[var(--accent-2)] text-white rounded-lg text-sm disabled:opacity-50">
                  {updating ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
                  {updating ? '更新中...' : '立即更新'}
                </button>
              </div>
              {updateInfo.releaseNotes && (
                <div className="pt-3 border-t fin-divider">
                  <span className={`text-xs ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>更新说明</span>
                  <p className={`text-sm mt-1 whitespace-pre-wrap ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>{updateInfo.releaseNotes}</p>
                </div>
              )}
            </div>
          ) : (
            <div className="flex items-center gap-2 text-accent-2">
              <Check className="h-4 w-4" /><span className="text-sm">已是最新版本</span>
            </div>
          )}
        </div>
      )}

      {progress && (
        <div className="fin-panel rounded-lg p-4 border fin-divider">
          <div className="flex items-center justify-between mb-2">
            <span className={`text-sm ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>{progress.message}</span>
            {progress.status === 'completed' && (
              <button onClick={handleRestart} className="flex items-center gap-2 px-3 py-1.5 bg-accent text-white rounded-lg text-xs">
                <RotateCcw className="h-3 w-3" />重启应用
              </button>
            )}
          </div>
          {progress.percent > 0 && (
            <div className={`w-full rounded-full h-2 ${colors.isDark ? 'bg-slate-700' : 'bg-slate-300'}`}>
              <div className={`h-2 rounded-full transition-all ${progress.status === 'error' ? 'bg-red-500' : progress.status === 'completed' ? 'bg-accent' : 'bg-accent-2'}`}
                style={{ width: `${progress.percent}%` }} />
            </div>
          )}
        </div>
      )}
    </div>
  );
};

// ========== 策略配置选项卡 ==========
interface StrategySettingsProps {
  strategies: Strategy[];
  activeStrategyId: string;
  onStrategiesChange: (strategies: Strategy[]) => void;
  onActiveChange: (id: string) => void;
  onStrategyAiIdChange: (id: string) => void;
  onAgentsReload: () => void;
  mcpServers: MCPServerConfig[];
  aiConfigs: AIConfig[];
  showToast: (type: 'success' | 'error' | 'loading', message: string) => void;
  strategyAiId: string;
}

// 视图类型
type StrategyView = 'list' | 'agents' | 'agent-edit';

const StrategySettings: React.FC<StrategySettingsProps> = ({
  strategies, activeStrategyId, strategyAiId, onStrategiesChange, onActiveChange, onStrategyAiIdChange, onAgentsReload, mcpServers, aiConfigs, showToast
}) => {
  const [view, setView] = useState<StrategyView>('list');
  const [selectedStrategy, setSelectedStrategy] = useState<Strategy | null>(null);
  const [selectedAgent, setSelectedAgent] = useState<StrategyAgent | null>(null);
  const [availableTools, setAvailableTools] = useState<ToolInfo[]>([]);
  const [generating, setGenerating] = useState(false);
  const [prompt, setPrompt] = useState('');
  const [error, setError] = useState('');

  // 加载可用工具
  useEffect(() => {
    getAvailableTools().then(setAvailableTools);
  }, []);

  // 进入策略的专家列表
  const handleSelectStrategy = (strategy: Strategy) => {
    setSelectedStrategy(strategy);
    setView('agents');
  };

  // 进入专家编辑
  const handleSelectAgent = (agent: StrategyAgent) => {
    setSelectedAgent(agent);
    setView('agent-edit');
  };

  // 返回策略列表
  const handleBackToList = () => {
    setSelectedStrategy(null);
    setSelectedAgent(null);
    setView('list');
  };

  // 返回专家列表
  const handleBackToAgents = () => {
    setSelectedAgent(null);
    setView('agents');
  };

  // 更新策略中的专家
  const handleUpdateAgent = async (updatedAgent: StrategyAgent) => {
    if (!selectedStrategy) return;
    const updatedAgents = selectedStrategy.agents.map(a =>
      a.id === updatedAgent.id ? updatedAgent : a
    );
    const updatedStrategy = { ...selectedStrategy, agents: updatedAgents };
    setSelectedStrategy(updatedStrategy);
    setSelectedAgent(updatedAgent);

    // 更新策略列表
    const newStrategies = strategies.map(s =>
      s.id === updatedStrategy.id ? updatedStrategy : s
    );
    onStrategiesChange(newStrategies);

    // 保存到后端
    try {
      await updateStrategy(updatedStrategy);
      showToast('success', '已保存');
      // 如果是当前激活策略，重新加载 agents
      if (selectedStrategy.id === activeStrategyId) {
        onAgentsReload();
      }
    } catch (e) {
      showToast('error', '保存失败');
    }
  };

  const handleGenerate = async () => {
    if (!prompt.trim()) return;
    setGenerating(true);
    setError('');
    try {
      const result = await generateStrategy(prompt);
      if (result.success && result.strategy) {
        onStrategiesChange([...strategies, result.strategy]);
        setPrompt('');
      } else {
        setError(result.error || '生成失败');
      }
    } catch (e) {
      setError(String(e));
    } finally {
      setGenerating(false);
    }
  };

  const handleDelete = async (id: string) => {
    const result = await deleteStrategy(id);
    if (result === 'success') {
      onStrategiesChange(strategies.filter(s => s.id !== id));
    }
  };

  const handleActivate = async (id: string) => {
    const result = await setActiveStrategy(id);
    if (result === 'success') {
      onActiveChange(id);
      onAgentsReload();
    }
  };

  // 专家编辑视图
  if (view === 'agent-edit' && selectedStrategy && selectedAgent) {
    return (
      <StrategyAgentEdit
        agent={selectedAgent}
        strategy={selectedStrategy}
        availableTools={availableTools}
        mcpServers={mcpServers}
        aiConfigs={aiConfigs}
        onBack={handleBackToAgents}
        onChange={handleUpdateAgent}
      />
    );
  }

  // 专家列表视图
  if (view === 'agents' && selectedStrategy) {
    return (
      <StrategyAgentList
        strategy={selectedStrategy}
        isActive={selectedStrategy.id === activeStrategyId}
        onBack={handleBackToList}
        onSelectAgent={handleSelectAgent}
        onAgentToggle={handleUpdateAgent}
      />
    );
  }

  // 策略列表视图
  return (
    <StrategyListView
      strategies={strategies}
      activeStrategyId={activeStrategyId}
      strategyAiId={strategyAiId}
      aiConfigs={aiConfigs}
      generating={generating}
      prompt={prompt}
      error={error}
      onPromptChange={setPrompt}
      onGenerate={handleGenerate}
      onSelectStrategy={handleSelectStrategy}
      onActivate={handleActivate}
      onDelete={handleDelete}
      onStrategyAiIdChange={onStrategyAiIdChange}
    />
  );
};

// 策略列表项组件
interface StrategyListItemProps {
  strategy: Strategy;
  isActive: boolean;
  onSelect: () => void;
  onActivate: () => void;
  onDelete: () => void;
}

const StrategyListItem: React.FC<StrategyListItemProps> = ({
  strategy, isActive, onSelect, onActivate, onDelete
}) => {
  const { colors } = useTheme();
  const agentNames = strategy.agents?.map(a => a.name).join('、') || '无';
  const enabledCount = strategy.agents?.filter(a => a.enabled).length || 0;

  return (
    <div
      onClick={onSelect}
      className={`p-3 rounded-lg border transition-all cursor-pointer ${
        isActive ? 'border-accent/50 bg-accent/10' : (colors.isDark ? 'border-slate-700 hover:border-slate-600' : 'border-slate-300 hover:border-slate-400')
      }`}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div
            className="w-8 h-8 min-w-[2rem] min-h-[2rem] rounded-lg flex items-center justify-center text-white text-sm font-medium shrink-0"
            style={{ backgroundColor: strategy.color }}
          >
            {strategy.name.charAt(0)}
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className={`text-sm font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{strategy.name}</span>
              {strategy.isBuiltin && (
                <span className={`text-xs px-1.5 py-0.5 fin-chip rounded ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>内置</span>
              )}
              {strategy.source === 'ai' && (
                <span className="text-xs px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded">AI</span>
              )}
              {isActive && (
                <span className="text-xs px-1.5 py-0.5 bg-accent/20 text-accent-2 rounded">当前</span>
              )}
            </div>
            <p className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-500'}`}>{strategy.description}</p>
            <p className={`text-xs mt-1 ${colors.isDark ? 'text-slate-600' : 'text-slate-400'}`}>
              专家: {agentNames} ({enabledCount}/{strategy.agents?.length || 0}启用)
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2" onClick={e => e.stopPropagation()}>
          {!isActive && (
            <button
              onClick={onActivate}
              className="px-2 py-1 text-xs text-accent-2 hover:bg-accent/20 rounded"
            >
              启用
            </button>
          )}
          {!strategy.isBuiltin && !isActive && (
            <button
              onClick={onDelete}
              className={`p-1.5 hover:text-red-400 hover:bg-red-500/20 rounded ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}
              title="删除"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
      </div>
    </div>
  );
};

// ========== 策略列表视图 ==========
interface StrategyListViewProps {
  strategies: Strategy[];
  activeStrategyId: string;
  strategyAiId: string;
  aiConfigs: AIConfig[];
  generating: boolean;
  prompt: string;
  error: string;
  onPromptChange: (v: string) => void;
  onGenerate: () => void;
  onSelectStrategy: (s: Strategy) => void;
  onActivate: (id: string) => void;
  onDelete: (id: string) => void;
  onStrategyAiIdChange: (id: string) => void;
}

const StrategyListView: React.FC<StrategyListViewProps> = ({
  strategies, activeStrategyId, strategyAiId, aiConfigs, generating, prompt, error,
  onPromptChange, onGenerate, onSelectStrategy, onActivate, onDelete, onStrategyAiIdChange
}) => {
  const { colors } = useTheme();
  return (
    <div className="space-y-6">
      {/* AI生成策略 */}
      <div>
        <h3 className={`font-medium mb-3 ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>AI生成策略组</h3>
        {/* 生成用模型选择 */}
        <div className="mb-3">
          <label className={`block text-sm mb-1.5 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>生成用模型</label>
          <select
            value={strategyAiId}
            onChange={e => onStrategyAiIdChange(e.target.value)}
            className={`w-full fin-input rounded-lg px-3 py-2 text-sm ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
          >
            <option value="">使用默认模型 {aiConfigs.find(c => c.isDefault) ? `(${aiConfigs.find(c => c.isDefault)!.name})` : ''}</option>
            {aiConfigs.map(c => (
              <option key={c.id} value={c.id}>{c.name} - {c.modelName}</option>
            ))}
          </select>
        </div>
        <textarea
          value={prompt}
          onChange={(e) => onPromptChange(e.target.value)}
          placeholder="描述你想要的投资策略组..."
          rows={3}
          className={`w-full fin-input rounded-lg px-3 py-2 text-sm resize-none ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
        />
        {error && <p className="text-red-400 text-xs mt-1">{error}</p>}
        <button
          onClick={onGenerate}
          disabled={generating || !prompt.trim()}
          className="mt-2 px-4 py-2 bg-gradient-to-br from-[var(--accent)] to-[var(--accent-2)] text-white rounded-lg text-sm disabled:opacity-50 flex items-center gap-2"
        >
          {generating ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
          {generating ? '生成中...' : '生成策略组'}
        </button>
      </div>

      {/* 策略列表 */}
      <div>
        <h3 className={`font-medium mb-3 ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>策略组列表</h3>
        <p className={`text-xs mb-3 ${colors.isDark ? 'text-slate-500' : 'text-slate-500'}`}>点击策略可查看和编辑专家配置</p>
        <div className="space-y-2">
          {strategies.map(s => (
            <StrategyListItem
              key={s.id}
              strategy={s}
              isActive={s.id === activeStrategyId}
              onSelect={() => onSelectStrategy(s)}
              onActivate={() => onActivate(s.id)}
              onDelete={() => onDelete(s.id)}
            />
          ))}
        </div>
      </div>
    </div>
  );
};

// ========== 策略专家列表视图 ==========
interface StrategyAgentListProps {
  strategy: Strategy;
  isActive: boolean;
  onBack: () => void;
  onSelectAgent: (agent: StrategyAgent) => void;
  onAgentToggle: (agent: StrategyAgent) => void;
}

const StrategyAgentList: React.FC<StrategyAgentListProps> = ({
  strategy, isActive, onBack, onSelectAgent, onAgentToggle
}) => {
  const { colors } = useTheme();
  const enabledCount = strategy.agents?.filter(a => a.enabled).length || 0;

  return (
    <div className="space-y-4">
      {/* 头部 */}
      <div className="flex items-center gap-3">
        <button
          onClick={onBack}
          className={`p-1.5 rounded-lg transition-colors ${colors.isDark ? 'hover:bg-slate-700/60 text-slate-400 hover:text-white' : 'hover:bg-slate-200/60 text-slate-500 hover:text-slate-700'}`}
        >
          <ChevronLeft className="h-5 w-5" />
        </button>
        <div
          className="w-10 h-10 min-w-[2.5rem] min-h-[2.5rem] rounded-lg flex items-center justify-center text-white text-sm font-medium shrink-0"
          style={{ backgroundColor: strategy.color }}
        >
          {strategy.name.charAt(0)}
        </div>
        <div>
          <div className="flex items-center gap-2">
            <h3 className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{strategy.name}</h3>
            {isActive && (
              <span className="text-xs px-1.5 py-0.5 bg-accent/20 text-accent-2 rounded">当前策略</span>
            )}
          </div>
          <p className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-500'}`}>{strategy.description}</p>
        </div>
      </div>

      {/* 专家统计 */}
      <div className={`text-sm ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
        共 {strategy.agents?.length || 0} 位专家，{enabledCount} 位已启用
      </div>

      {/* 专家列表 */}
      <div className="space-y-2">
        {strategy.agents?.map(agent => (
          <StrategyAgentListItem
            key={agent.id}
            agent={agent}
            onSelect={() => onSelectAgent(agent)}
            onToggle={() => onAgentToggle({ ...agent, enabled: !agent.enabled })}
          />
        ))}
      </div>
    </div>
  );
};

// 策略专家列表项
interface StrategyAgentListItemProps {
  agent: StrategyAgent;
  onSelect: () => void;
  onToggle: () => void;
}

const StrategyAgentListItem: React.FC<StrategyAgentListItemProps> = ({
  agent, onSelect, onToggle
}) => {
  const { colors } = useTheme();
  return (
    <div
      onClick={onSelect}
      className={`flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition-all ${
        agent.enabled
          ? (colors.isDark ? 'border-slate-700 hover:border-slate-600' : 'border-slate-300 hover:border-slate-400')
          : (colors.isDark ? 'border-slate-800 bg-slate-800/30 opacity-60' : 'border-slate-200 bg-slate-100/30 opacity-60')
      }`}
    >
      <div
        className="w-10 h-10 min-w-[2.5rem] min-h-[2.5rem] rounded-full flex items-center justify-center text-sm shrink-0"
        style={{ backgroundColor: agent.color + '20', color: agent.color }}
      >
        {agent.name.charAt(0)}
      </div>
      <div className="flex-1 min-w-0">
        <div className={`text-sm font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{agent.name}</div>
        <div className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-500'}`}>{agent.role}</div>
      </div>
      <button
        onClick={(e) => { e.stopPropagation(); onToggle(); }}
        className={`w-10 h-5 rounded-full transition-colors ${
          agent.enabled ? 'bg-accent' : (colors.isDark ? 'bg-slate-600' : 'bg-slate-400')
        }`}
      >
        <div className={`w-4 h-4 bg-white rounded-full shadow transition-transform ${
          agent.enabled ? 'translate-x-5' : 'translate-x-0.5'
        }`} />
      </button>
    </div>
  );
};

// ========== 策略专家编辑视图 ==========
interface StrategyAgentEditProps {
  agent: StrategyAgent;
  strategy: Strategy;
  availableTools: ToolInfo[];
  mcpServers: MCPServerConfig[];
  aiConfigs: AIConfig[];
  onBack: () => void;
  onChange: (agent: StrategyAgent) => void;
}

type AgentEditTab = 'basic' | 'tools';

const StrategyAgentEdit: React.FC<StrategyAgentEditProps> = ({
  agent, strategy, availableTools, mcpServers, aiConfigs, onBack, onChange
}) => {
  const [editedAgent, setEditedAgent] = useState<StrategyAgent>(agent);
  const [activeTab, setActiveTab] = useState<AgentEditTab>('basic');

  useEffect(() => {
    setEditedAgent(agent);
  }, [agent]);

  const handleChange = <K extends keyof StrategyAgent>(field: K, value: StrategyAgent[K]) => {
    const updated = { ...editedAgent, [field]: value };
    setEditedAgent(updated);
    onChange(updated);
  };

  const toggleTool = (toolName: string) => {
    const currentTools = editedAgent.tools || [];
    const newTools = currentTools.includes(toolName)
      ? currentTools.filter(t => t !== toolName)
      : [...currentTools, toolName];
    handleChange('tools', newTools);
  };

  const toggleMCPServer = (serverId: string) => {
    const currentServers = editedAgent.mcpServers || [];
    const newServers = currentServers.includes(serverId)
      ? currentServers.filter(s => s !== serverId)
      : [...currentServers, serverId];
    handleChange('mcpServers', newServers);
  };

  const selectedToolsCount = (editedAgent.tools || []).length;
  const selectedMCPCount = (editedAgent.mcpServers || []).length;

  return (
    <div className="space-y-4">
      {/* 头部 */}
      <AgentEditHeader
        agent={editedAgent}
        strategyName={strategy.name}
        onBack={onBack}
        onToggleEnabled={() => handleChange('enabled', !editedAgent.enabled)}
      />

      {/* 标签页切换 */}
      <AgentEditTabs
        activeTab={activeTab}
        selectedToolsCount={selectedToolsCount}
        selectedMCPCount={selectedMCPCount}
        onTabChange={setActiveTab}
      />

      {/* 基础配置 */}
      {activeTab === 'basic' && (
        <AgentBasicConfig
          agent={editedAgent}
          aiConfigs={aiConfigs}
          onChange={handleChange}
        />
      )}

      {/* 工具配置 */}
      {activeTab === 'tools' && (
        <AgentToolsConfig
          agent={editedAgent}
          availableTools={availableTools}
          mcpServers={mcpServers}
          onToggleTool={toggleTool}
          onToggleMCPServer={toggleMCPServer}
        />
      )}
    </div>
  );
};

// 专家编辑头部
interface AgentEditHeaderProps {
  agent: StrategyAgent;
  strategyName: string;
  onBack: () => void;
  onToggleEnabled: () => void;
}

const AgentEditHeader: React.FC<AgentEditHeaderProps> = ({
  agent, strategyName, onBack, onToggleEnabled
}) => {
  const { colors } = useTheme();
  return (
    <div className="flex items-center justify-between">
      <div className="flex items-center gap-3">
        <button
          onClick={onBack}
          className={`p-1.5 rounded-lg transition-colors ${colors.isDark ? 'hover:bg-slate-700/60 text-slate-400 hover:text-white' : 'hover:bg-slate-200/60 text-slate-500 hover:text-slate-700'}`}
        >
          <ChevronLeft className="h-5 w-5" />
        </button>
        <div
          className="w-10 h-10 min-w-[2.5rem] min-h-[2.5rem] rounded-full flex items-center justify-center text-sm shrink-0"
          style={{ backgroundColor: agent.color + '20', color: agent.color }}
        >
          {agent.name.charAt(0)}
        </div>
        <div>
          <h3 className={`font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{agent.name}</h3>
          <p className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-500'}`}>{strategyName} / {agent.role}</p>
        </div>
      </div>
      <button
        onClick={onToggleEnabled}
        className={`w-11 h-6 rounded-full transition-colors ${
          agent.enabled ? 'bg-accent' : (colors.isDark ? 'bg-slate-600' : 'bg-slate-400')
        }`}
      >
        <div className={`w-5 h-5 bg-white rounded-full shadow transition-transform ${
          agent.enabled ? 'translate-x-5' : 'translate-x-0.5'
        }`} />
      </button>
    </div>
  );
};

// 专家编辑标签页
interface AgentEditTabsProps {
  activeTab: AgentEditTab;
  selectedToolsCount: number;
  selectedMCPCount: number;
  onTabChange: (tab: AgentEditTab) => void;
}

const AgentEditTabs: React.FC<AgentEditTabsProps> = ({
  activeTab, selectedToolsCount, selectedMCPCount, onTabChange
}) => {
  const { colors } = useTheme();
  const totalCount = selectedToolsCount + selectedMCPCount;
  return (
    <div className="flex gap-1 p-1 fin-panel rounded-lg border fin-divider">
      <button
        onClick={() => onTabChange('basic')}
        className={`flex-1 flex items-center justify-center gap-2 px-3 py-2 text-sm rounded-md transition-all ${
          activeTab === 'basic'
            ? 'bg-gradient-to-br from-[var(--accent)] to-[var(--accent-2)] text-white'
            : (colors.isDark ? 'text-slate-400 hover:text-white hover:bg-slate-700/60' : 'text-slate-500 hover:text-slate-700 hover:bg-slate-200/60')
        }`}
      >
        <Sliders className="h-4 w-4" />
        基础配置
      </button>
      <button
        onClick={() => onTabChange('tools')}
        className={`flex-1 flex items-center justify-center gap-2 px-3 py-2 text-sm rounded-md transition-all ${
          activeTab === 'tools'
            ? 'bg-gradient-to-br from-[var(--accent)] to-[var(--accent-2)] text-white'
            : (colors.isDark ? 'text-slate-400 hover:text-white hover:bg-slate-700/60' : 'text-slate-500 hover:text-slate-700 hover:bg-slate-200/60')
        }`}
      >
        <Wrench className="h-4 w-4" />
        工具配置
        {totalCount > 0 && (
          <span className="px-1.5 py-0.5 text-xs bg-white/20 rounded-full">
            {totalCount}
          </span>
        )}
      </button>
    </div>
  );
};

// 专家基础配置
interface AgentBasicConfigProps {
  agent: StrategyAgent;
  aiConfigs: AIConfig[];
  onChange: <K extends keyof StrategyAgent>(field: K, value: StrategyAgent[K]) => void;
}

const AgentBasicConfig: React.FC<AgentBasicConfigProps> = ({ agent, aiConfigs, onChange }) => {
  const { colors } = useTheme();
  const [enhancing, setEnhancing] = useState(false);

  const handleEnhance = async () => {
    if (!agent.instruction?.trim()) return;

    setEnhancing(true);
    try {
      const result = await enhancePrompt({
        originalPrompt: agent.instruction,
        agentRole: agent.role,
        agentName: agent.name,
      });

      if (result.success && result.enhancedPrompt) {
        onChange('instruction', result.enhancedPrompt);
      }
    } catch (e) {
      console.error('增强失败:', e);
    } finally {
      setEnhancing(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* AI 配置选择 */}
      <div>
        <label className={`block text-sm mb-1.5 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>AI 模型</label>
        <select
          value={agent.aiConfigId || ''}
          onChange={e => onChange('aiConfigId', e.target.value)}
          className={`w-full fin-input rounded-lg px-3 py-2 text-sm ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
        >
          <option value="">使用默认配置</option>
          {aiConfigs.map(config => (
            <option key={config.id} value={config.id}>
              {config.name} ({config.modelName})
              {config.isDefault ? ' [默认]' : ''}
            </option>
          ))}
        </select>
        <p className={`text-xs mt-1 ${colors.isDark ? 'text-slate-500' : 'text-slate-500'}`}>为该专家指定专用的 AI 模型，留空则使用系统默认配置</p>
      </div>

      {/* 系统指令 */}
      <div>
        <div className="flex items-center justify-between mb-1.5">
          <label className={`block text-sm ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>系统指令 (Prompt)</label>
          <button
            onClick={handleEnhance}
            disabled={enhancing || !agent.instruction?.trim()}
            className="flex items-center gap-1.5 px-2 py-1 text-xs bg-gradient-to-br from-[var(--accent)] to-[var(--accent-2)] text-white rounded-lg disabled:opacity-50 hover:opacity-90 transition-opacity"
          >
            {enhancing ? (
              <>
                <Loader2 className="h-3 w-3 animate-spin" />
                增强中...
              </>
            ) : (
              <>
                <Sparkles className="h-3 w-3" />
                AI 增强
              </>
            )}
          </button>
        </div>
        <textarea
          value={agent.instruction || ""}
          onChange={e => onChange("instruction", e.target.value)}
          rows={10}
          placeholder="定义专家的行为和角色..."
          className={`w-full fin-input rounded-lg px-3 py-2 text-sm resize-none ${colors.isDark ? 'text-white' : 'text-slate-800'}`}
        />
      </div>
    </div>
  );
};

// 专家工具配置
interface AgentToolsConfigProps {
  agent: StrategyAgent;
  availableTools: ToolInfo[];
  mcpServers: MCPServerConfig[];
  onToggleTool: (toolName: string) => void;
  onToggleMCPServer: (serverId: string) => void;
}

const AgentToolsConfig: React.FC<AgentToolsConfigProps> = ({
  agent, availableTools, mcpServers, onToggleTool, onToggleMCPServer
}) => {
  const { colors } = useTheme();
  const selectedTools = agent.tools || [];
  const selectedMCPServers = agent.mcpServers || [];

  return (
    <div className="space-y-6">
      {/* 内置工具 */}
      <div className="space-y-2">
        <div className="flex items-center justify-between mb-2">
          <label className={`text-sm flex items-center gap-1.5 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
            <Wrench className="h-4 w-4" />
            内置工具
            <span className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>({selectedTools.length}/{availableTools.length})</span>
          </label>
        </div>
        <div className="grid grid-cols-1 gap-2 max-h-48 overflow-y-auto fin-scrollbar">
          {availableTools.map(tool => {
            const isSelected = selectedTools.includes(tool.name);
            return (
              <div
                key={tool.name}
                onClick={() => onToggleTool(tool.name)}
                className={`flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition-all ${
                  isSelected
                    ? "border-accent/50 bg-accent/10"
                    : (colors.isDark ? "border-slate-700 hover:border-slate-600 hover:bg-slate-800/40" : "border-slate-300 hover:border-slate-400 hover:bg-slate-100/40")
                }`}
              >
                <div className={`w-5 h-5 rounded flex items-center justify-center shrink-0 ${
                  isSelected ? "bg-accent text-white" : (colors.isDark ? "bg-slate-700 border border-slate-600" : "bg-slate-200 border border-slate-300")
                }`}>
                  {isSelected && <Check className="h-3 w-3" />}
                </div>
                <div className="flex-1 min-w-0">
                  <div className={`text-sm font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{tool.name}</div>
                  <div className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-500'}`}>{tool.description}</div>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* MCP 服务器 */}
      {mcpServers.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center justify-between mb-2">
            <label className={`text-sm flex items-center gap-1.5 ${colors.isDark ? 'text-slate-400' : 'text-slate-500'}`}>
              <Plug className="h-4 w-4" />
              MCP 服务器
              <span className={`text-xs ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>({selectedMCPServers.length}/{mcpServers.length})</span>
            </label>
          </div>
          <div className="grid grid-cols-1 gap-2 max-h-48 overflow-y-auto fin-scrollbar">
            {mcpServers.map(server => {
              const isSelected = selectedMCPServers.includes(server.id);
              return (
                <div
                  key={server.id}
                  onClick={() => onToggleMCPServer(server.id)}
                  className={`flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition-all ${
                    isSelected
                      ? "border-purple-500/50 bg-purple-500/10"
                      : (colors.isDark ? "border-slate-700 hover:border-slate-600 hover:bg-slate-800/40" : "border-slate-300 hover:border-slate-400 hover:bg-slate-100/40")
                  }`}
                >
                  <div className={`w-5 h-5 rounded flex items-center justify-center shrink-0 ${
                    isSelected ? "bg-purple-500 text-white" : (colors.isDark ? "bg-slate-700 border border-slate-600" : "bg-slate-200 border border-slate-300")
                  }`}>
                    {isSelected && <Check className="h-3 w-3" />}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className={`text-sm font-medium ${colors.isDark ? 'text-white' : 'text-slate-800'}`}>{server.name}</div>
                    <div className={`text-xs truncate ${colors.isDark ? 'text-slate-500' : 'text-slate-500'}`}>{server.command} {server.args?.join(' ')}</div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
};
