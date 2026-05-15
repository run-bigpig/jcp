import type { ChatMessage } from '../services/sessionService';

export interface ChatExportOptions {
  stockCode?: string;
  stockName?: string;
  exportedAt?: Date;
}

const formatTime = (timestamp?: number): string => {
  if (!timestamp) return '';
  return new Date(timestamp).toLocaleString();
};

const getMessageName = (message: ChatMessage): string => {
  if (message.agentId === 'system') return '系统提示';
  return message.agentName || message.agentId || '未知发言人';
};

const getMessageMeta = (message: ChatMessage): string[] => {
  const meta: string[] = [];
  const time = formatTime(message.timestamp);
  if (time) meta.push(`时间：${time}`);
  if (message.role) meta.push(`角色：${message.role}`);
  if (message.error) meta.push(`错误：${message.error}`);
  return meta;
};

const sanitizeFilenamePart = (value: string): string => {
  return value
    .trim()
    .replace(/[\\/:*?"<>|]/g, '-')
    .replace(/\s+/g, '-')
    .slice(0, 80);
};

export const buildChatExportFilename = (
  options: ChatExportOptions,
  extension: 'md'
): string => {
  const exportedAt = options.exportedAt || new Date();
  const datePart = exportedAt.toISOString().slice(0, 10);
  const stockPart = sanitizeFilenamePart(
    [options.stockCode, options.stockName].filter(Boolean).join('-') || 'chat'
  );
  return `${stockPart}-selected-chat-${datePart}.${extension}`;
};

export const createChatExportMarkdown = (
  messages: ChatMessage[],
  options: ChatExportOptions = {}
): string => {
  const exportedAt = options.exportedAt || new Date();
  const titleParts = ['韭菜讨论中心导出'];
  if (options.stockCode || options.stockName) {
    titleParts.push([options.stockCode, options.stockName].filter(Boolean).join(' '));
  }

  const lines: string[] = [
    `# ${titleParts.join(' - ')}`,
    '',
    `导出时间：${exportedAt.toLocaleString()}`,
    `消息数量：${messages.length}`,
    '',
    '---',
    '',
  ];

  messages.forEach((message, index) => {
    lines.push(`## ${index + 1}. ${getMessageName(message)}`);
    const meta = getMessageMeta(message);
    if (meta.length > 0) {
      lines.push(meta.join('  '));
      lines.push('');
    }
    lines.push(message.content || '');
    lines.push('');
    lines.push('---');
    lines.push('');
  });

  return lines.join('\n').trimEnd() + '\n';
};

export const downloadMarkdown = (markdown: string, filename: string): void => {
  const blob = new Blob([markdown], { type: 'text/markdown;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  URL.revokeObjectURL(url);
};

const escapeHtml = (value: string): string => {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
};

export const openChatExportPrintWindow = (
  messages: ChatMessage[],
  options: ChatExportOptions = {}
): boolean => {
  const printWindow = window.open('', '_blank', 'width=900,height=700');
  if (!printWindow) return false;

  const exportedAt = options.exportedAt || new Date();
  const subtitle = [options.stockCode, options.stockName].filter(Boolean).join(' ');
  const messageHtml = messages.map((message, index) => {
    const meta = getMessageMeta(message);
    return `
      <section class="message">
        <h2>${index + 1}. ${escapeHtml(getMessageName(message))}</h2>
        ${meta.length > 0 ? `<div class="meta">${escapeHtml(meta.join(' · '))}</div>` : ''}
        <pre>${escapeHtml(message.content || '')}</pre>
      </section>
    `;
  }).join('');

  printWindow.document.write(`
    <!doctype html>
    <html>
      <head>
        <meta charset="utf-8" />
        <title>韭菜讨论中心导出</title>
        <style>
          body {
            color: #111827;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
            line-height: 1.6;
            margin: 32px;
          }
          h1 {
            font-size: 24px;
            margin: 0 0 8px;
          }
          .summary {
            color: #4b5563;
            font-size: 12px;
            margin-bottom: 24px;
          }
          .message {
            border-top: 1px solid #d1d5db;
            break-inside: avoid;
            padding: 18px 0;
          }
          h2 {
            font-size: 16px;
            margin: 0 0 6px;
          }
          .meta {
            color: #6b7280;
            font-size: 12px;
            margin-bottom: 10px;
          }
          pre {
            font-family: inherit;
            margin: 0;
            white-space: pre-wrap;
            word-break: break-word;
          }
          @media print {
            body { margin: 18mm; }
          }
        </style>
      </head>
      <body>
        <h1>韭菜讨论中心导出</h1>
        <div class="summary">
          ${subtitle ? `${escapeHtml(subtitle)} · ` : ''}
          导出时间：${escapeHtml(exportedAt.toLocaleString())} · 消息数量：${messages.length}
        </div>
        ${messageHtml}
        <script>
          window.onload = () => {
            window.focus();
            window.print();
          };
        </script>
      </body>
    </html>
  `);
  printWindow.document.close();
  return true;
};
