import React, { useState, useEffect, useRef } from 'react';
import { getAgentConfigs, AgentConfig } from '../services/strategyService';
import { StockSession, ChatMessage, sendMeetingMessage, MeetingMessageRequest, getSessionMessages } from '../services/sessionService';
import { MessageSquare, Loader2, Send, User, Users, X, Reply, Trash2, Wrench, CheckCircle2, AlertCircle, Copy, Check, RotateCcw, Pencil, Square } from 'lucide-react';
import { clearSessionMessages } from '../services/sessionService';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';
import { useMentionPicker } from '../hooks/useMentionPicker';
import { useToast } from '../hooks/useToast';
import { useTheme } from '../contexts/ThemeContext';
import { CancelMeeting } from '../../wailsjs/go/main/App';
import 'markstream-react/index.css';

// 进度事件类型
interface ProgressEvent {
  type: 'agent_start' | 'agent_done' | 'tool_call' | 'tool_result' | 'streaming';
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
  lastStatus: string;
}

interface AgentRoomProps {
  session: StockSession | null;
  onSessionUpdate: (session: StockSession) => void;
}

const normalizeMarkdown = (input: string) => input.replace(/\r\n/g, '\n').replace(/\n{3,}/g, '\n\n').trim();

const evidenceLinePattern = /^(?:【\s*)?(?:参考)?(?:数据)?依据(?:\s*】)?\s*[:：]/;
const evidenceInlinePattern = /(?:【\s*)?(?:参考)?(?:数据)?依据(?:\s*】)?\s*[:：]/;
const evidenceHeaderPattern = /^(?:#{1,6}\s*)?(?:\*\*)?(?:【\s*)?(?:参考|数据)?依据(?:\s*】)?(?:\*\*)?\s*[:：]?\s*$/;
const referenceMarkerPattern = /\[(\d+)\]/g;
const referenceLinePattern = /^\[(\d+)\]\s*[：:、\-]?\s*(.+)$/;
const referenceLineAltPattern = /^(\d+)[\).、）]\s*(.+)$/;
const referenceLineCnPattern = /^【\s*(\d+)\s*】\s*(.+)$/;
const inlineHeadingBoundaryPattern = /(盘中路径推演|盘中推演|修正点|事件与结构风险|应对与风控|应对建议|应对（只谈风控）|应对|风险与止损|触发条件|失效条件|操作建议|结论|参考依据)\s*[：:]/;
const inlineHeadingPrefixPattern = /^([^\s:：]{2,20})\s*[：:]\s*(.+)$/;
const bracketHeadingPrefixPattern = /^([【\[][^】\]]{2,20}[】\]])\s*(.+)$/;
const inlineHeadingSemanticPattern = /(结论|理由|风控|失效|触发|建议|策略|操作|仓位|风险|应对|路径|计划|观察|判断|逻辑|机会|节奏|盘口|情绪)/;
const inlineHeadingKeywords = [
  '盘中路径推演',
  '盘中推演',
  '修正点',
  '事件与结构风险',
  '应对与风控',
  '应对建议',
  '应对（只谈风控）',
  '应对',
  '风险与止损',
  '触发条件',
  '失效条件',
  '操作建议',
  '结论',
  '参考依据',
];
const dedupeHeadingKeywords = [
  '触发条件',
  '风险与止损',
  '失效条件',
  '应对与风控',
  '操作与风控',
  '应对建议',
];

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
  currentStockCodeRef.current = session?.stockCode || null;

  // 会议取消标识
  const meetingCancelledRef = useRef<Record<string, boolean>>({});

  // 使用自定义 Hooks
  const { toast, showToast, hideToast } = useToast();
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

  const parseReferenceLine = (line: string) => {
    const trimmed = line.trim();
    if (!trimmed) {
      return null;
    }
    const m =
      trimmed.match(referenceLinePattern) ||
      trimmed.match(referenceLineAltPattern) ||
      trimmed.match(referenceLineCnPattern);
    if (!m || !m[2]) {
      return null;
    }
    return { id: m[1], text: m[2].trim() };
  };

  const parseEmbeddedReferenceSegments = (text: string) => {
    const refs = new Map<string, string>();
    const markerPattern = /\[(\d+)\]|【\s*(\d+)\s*】/g;
    const markers: { id: string; index: number; raw: string }[] = [];
    for (let m = markerPattern.exec(text); m !== null; m = markerPattern.exec(text)) {
      markers.push({
        id: (m[1] || m[2] || '').trim(),
        index: m.index,
        raw: m[0],
      });
    }
    if (markers.length === 0) {
      return refs;
    }
    for (let i = 0; i < markers.length; i += 1) {
      const marker = markers[i];
      const start = marker.index + marker.raw.length;
      const end = i + 1 < markers.length ? markers[i + 1].index : text.length;
      const content = text
        .slice(start, end)
        .replace(/^[\s：:、\-]+/, '')
        .trim();
      if (marker.id && content) {
        refs.set(marker.id, content);
      }
    }
    return refs;
  };

  const normalizeNarrativeBody = (body: string) => {
    return body
      .replace(/\r\n/g, '\n')
      .replace(/([。！？；])\s*(#{1,6}\s*(?:结论|理由|触发与风控|失效条件))/g, '$1\n$2')
      .replace(/([。！？；])\s*(盘中路径推演|盘中推演|修正点|事件与结构风险|应对与风控|应对建议|应对（只谈风控）|应对|风险与止损|触发条件|失效条件|操作建议|结论|参考依据)\s*[：:]/g, '$1\n$2：')
      .replace(/([。！？；])\s*([【\[](?:结论|理由|理由与热点|热点与情绪|盘中路径推演|盘中推演|盘口|盘口\/触发|触发\/风控|触发与风控|风险与止损|应对与风控|触发条件|失效条件|操作建议)[】\]])/g, '$1\n$2 ')
      .replace(/\n{3,}/g, '\n\n')
      .trim();
  };

  const isPrimaryNarrativeHeading = (title: string) => {
    const cleaned = title.replace(/[【】\[\]\s：:]/g, '');
    if (!cleaned) {
      return false;
    }
    return [
      '结论',
      '综合结论',
      '理由',
      '触发与风控',
      '风险与止损',
      '应对与风控',
      '触发条件',
      '失效条件',
      '操作建议',
      '操作策略',
      '盘中路径推演',
      '盘中推演',
      '修正点',
      '事件与结构风险',
      '应对建议',
      '参考依据',
    ].some(key => cleaned.includes(key) || key.includes(cleaned));
  };

  const splitInlineHeadingLine = (line: string) => {
    const bracketMatch = line.match(bracketHeadingPrefixPattern);
    if (bracketMatch) {
      const title = bracketMatch[1].trim();
      const rest = bracketMatch[2].trim();
      const normalizedTitle = title.replace(/[【】\[\]\s]/g, '');
      const isPureNumericRef = /^\d+$/.test(normalizedTitle);
      if (rest.length > 0 && !isPureNumericRef) {
        return { title, rest, primary: isPrimaryNarrativeHeading(title) };
      }
    }

    const m = line.match(inlineHeadingPrefixPattern);
    if (!m) {
      return null;
    }
    const title = m[1].trim();
    const rest = m[2].trim();
    if (!inlineHeadingBoundaryPattern.test(`${title}：`)) {
      if (
        !inlineHeadingKeywords.some(key => title.includes(key) || key.includes(title)) &&
        !inlineHeadingSemanticPattern.test(title)
      ) {
        return null;
      }
    }
    if (rest.length === 0) {
      return null;
    }
    return { title, rest, primary: isPrimaryNarrativeHeading(title) };
  };

  const formatHeadingLabel = (title: string, primary = true) => {
    const cleaned = title.replace(/[：:]\s*$/, '').trim();
    if (!cleaned) {
      return '';
    }
    if (!primary) {
      return cleaned;
    }
    if (
      (cleaned.startsWith('【') && cleaned.endsWith('】')) ||
      (cleaned.startsWith('[') && cleaned.endsWith(']'))
    ) {
      return cleaned;
    }
    return `【${cleaned}】`;
  };

  const narrativeClassByHeading = (title: string) => {
    const cleaned = title.replace(/[【】\[\]\s]/g, '');
    if (!cleaned) {
      return '';
    }
    if (/(风险与止损|触发与风控|应对与风控|失效条件|止损)/.test(cleaned)) {
      return 'agent-narrative-risk';
    }
    if (/(结论|操作建议|操作策略|策略建议)/.test(cleaned)) {
      return 'agent-narrative-keypoint';
    }
    return '';
  };

  const normalizeDedupText = (input: string) => {
    return input
      .replace(/\[(\d+)\]|【\s*\d+\s*】/g, '')
      .replace(/[，。；;、:\s]+/g, '')
      .trim();
  };

  const shouldDedupeHeading = (title: string) => {
    return dedupeHeadingKeywords.some(key => title.includes(key) || key.includes(title));
  };

  const parseReferenceBlock = (lines: string[]) => {
    const refs = new Map<string, string>();
    let refStart = -1;
    let headerIndex = -1;
    let i = lines.length - 1;
    let collecting = false;
    let pendingContinuation: string[] = [];
    while (i >= 0) {
      const line = lines[i];
      const trimmed = line.trim();
      if (!collecting) {
        if (trimmed === '') {
          i -= 1;
          continue;
        }
        if (parseReferenceLine(trimmed)) {
          collecting = true;
        } else {
          break;
        }
      }
      if (trimmed === '') {
        i -= 1;
        continue;
      }
      const parsed = parseReferenceLine(trimmed);
      if (!parsed) {
        if (evidenceHeaderPattern.test(trimmed)) {
          headerIndex = i;
          break;
        }
        pendingContinuation.unshift(trimmed);
        i -= 1;
        continue;
      }
      const chunks = [parsed.text, ...pendingContinuation].map(chunk => chunk.trim()).filter(Boolean);
      refs.set(parsed.id, chunks.join(' '));
      pendingContinuation = [];
      refStart = i;
      i -= 1;
    }
    if (refs.size === 0 || refStart < 0) {
      return { body: lines.join('\n').trim(), refs: new Map<string, string>() };
    }
    const bodyEnd = headerIndex >= 0 ? headerIndex : refStart;
    const body = lines.slice(0, Math.max(0, bodyEnd)).join('\n').trim();
    return { body, refs };
  };

  const splitEvidenceTail = (content: string) => {
    const lines = content.split('\n');
    let evidenceStart = -1;
    for (let i = lines.length - 1; i >= 0; i -= 1) {
      if (evidenceLinePattern.test(lines[i].trim())) {
        evidenceStart = i;
        break;
      }
    }
    if (evidenceStart < 0) {
      return { body: content, evidence: '' };
    }
    return {
      body: lines.slice(0, evidenceStart).join('\n').trim(),
      evidence: lines.slice(evidenceStart).join('\n').trim(),
    };
  };

  const parseEvidenceReferences = (evidenceText: string) => {
    const refs = new Map<string, string>();
    if (!evidenceText.trim()) {
      return refs;
    }

    let currentId = '';
    let currentChunks: string[] = [];
    const flush = () => {
      if (!currentId) {
        return;
      }
      const merged = currentChunks
        .map(chunk => chunk.trim())
        .filter(Boolean)
        .join(' ')
        .trim();
      if (merged) {
        refs.set(currentId, merged);
      }
      currentId = '';
      currentChunks = [];
    };

    for (const line of evidenceText.split('\n')) {
      const trimmed = line.trim();
      if (!trimmed) {
        continue;
      }

      if (evidenceHeaderPattern.test(trimmed)) {
        continue;
      }

      if (evidenceLinePattern.test(trimmed)) {
        const inline = trimmed.replace(evidenceLinePattern, '').trim();
        if (!inline) {
          continue;
        }
        const parsedInline = parseReferenceLine(inline);
        if (parsedInline) {
          flush();
          currentId = parsedInline.id;
          currentChunks = [parsedInline.text];
        } else if (currentId) {
          currentChunks.push(inline);
        }
        continue;
      }

      const parsed = parseReferenceLine(trimmed);
      if (parsed) {
        flush();
        currentId = parsed.id;
        currentChunks = [parsed.text];
        continue;
      }

      if (currentId) {
        currentChunks.push(trimmed);
      }
    }

    flush();
    return refs;
  };

  const hasVisibleReferenceMarkers = (body: string, refs: Map<string, string>) => {
    if (!body || refs.size === 0) {
      return false;
    }
    for (const refId of refs.keys()) {
      if (body.includes(`[${refId}]`) || body.includes(`【${refId}】`)) {
        return true;
      }
    }
    return false;
  };

  const renderCollapsedReferenceBlock = (refs: Map<string, string>) => {
    if (refs.size === 0) {
      return null;
    }
    const refText = Array.from(refs.entries())
      .map(([id, text]) => `[${id}] ${text}`)
      .join('\n');
    return (
      <details className="agent-evidence agent-evidence-collapsed">
        <summary className="agent-evidence-summary">查看依据</summary>
        <div className="agent-evidence-text">{refText}</div>
      </details>
    );
  };

  const extractInlineEvidenceFromMixedContent = (content: string) => {
    const lines = content.split('\n');
    const refs = new Map<string, string>();
    const kept: string[] = [];
    const continuationPattern = /^(?:来源|工具|时间|口径|指标|注[:：]?|\+|[-*]\s)/;

    let skippingEvidence = false;
    let currentId = '';
    let currentChunks: string[] = [];

    const flush = () => {
      if (!currentId) {
        return;
      }
      const merged = currentChunks
        .map(chunk => chunk.trim())
        .filter(Boolean)
        .join(' ')
        .trim();
      if (merged) {
        refs.set(currentId, merged);
      }
      currentId = '';
      currentChunks = [];
    };

    const parseEvidenceInline = (line: string) => {
      const inline = line.replace(evidenceInlinePattern, '').trim();
      if (!inline) {
        return;
      }
      const parsedInline = parseReferenceLine(inline);
      if (parsedInline) {
        flush();
        currentId = parsedInline.id;
        currentChunks = [parsedInline.text];
        return;
      }
      const embeddedRefs = parseEmbeddedReferenceSegments(inline);
      if (embeddedRefs.size > 0) {
        flush();
        embeddedRefs.forEach((value, key) => refs.set(key, value));
        return;
      }
    };

    for (const line of lines) {
      const trimmed = line.trim();
      const inlineEvidenceMatch = line.match(evidenceInlinePattern);

      if (!skippingEvidence) {
        if (evidenceHeaderPattern.test(trimmed) || inlineEvidenceMatch) {
          skippingEvidence = true;
          if (inlineEvidenceMatch) {
            const idx = inlineEvidenceMatch.index ?? 0;
            const prefix = line.slice(0, idx).trimEnd();
            if (prefix) {
              kept.push(prefix);
            }
            parseEvidenceInline(line.slice(idx).trim());
          }
          continue;
        }
        kept.push(line);
        continue;
      }

      if (!trimmed) {
        continue;
      }

      if (evidenceHeaderPattern.test(trimmed) || inlineEvidenceMatch) {
        if (inlineEvidenceMatch) {
          const idx = inlineEvidenceMatch.index ?? 0;
          parseEvidenceInline(line.slice(idx).trim());
        } else if (evidenceLinePattern.test(trimmed)) {
          parseEvidenceInline(trimmed);
        }
        continue;
      }

      const parsed = parseReferenceLine(trimmed);
      if (parsed) {
        flush();
        currentId = parsed.id;
        currentChunks = [parsed.text];
        continue;
      }

      const embeddedRefs = parseEmbeddedReferenceSegments(trimmed);
      if (embeddedRefs.size > 0) {
        flush();
        embeddedRefs.forEach((value, key) => refs.set(key, value));
        continue;
      }

      if (currentId && continuationPattern.test(trimmed)) {
        currentChunks.push(trimmed);
        continue;
      }

      flush();
      skippingEvidence = false;
      kept.push(line);
    }

    flush();

    return {
      body: kept.join('\n').replace(/\n{3,}/g, '\n\n').trim(),
      refs,
    };
  };

  const renderTextWithRefs = (text: string, refs: Map<string, string>, keyPrefix: string) => {
    const pieces: React.ReactNode[] = [];
    let last = 0;
    let matched = false;
    const matcher = new RegExp(referenceMarkerPattern);
    for (let match = matcher.exec(text); match !== null; match = matcher.exec(text)) {
      const refId = match[1];
      const tip = refs.get(refId);
      if (!tip) {
        continue;
      }
      matched = true;
      if (match.index > last) {
        pieces.push(
          <React.Fragment key={`${keyPrefix}-t-${last}`}>
            {text.slice(last, match.index)}
          </React.Fragment>,
        );
      }
      pieces.push(
        <sup
          key={`${keyPrefix}-r-${refId}-${match.index}`}
          className="agent-ref-marker"
          data-tip={tip}
        >
          {refId}
        </sup>,
      );
      last = match.index + match[0].length;
    }
    if (!matched) {
      return text;
    }
    if (last < text.length) {
      pieces.push(<React.Fragment key={`${keyPrefix}-tail`}>{text.slice(last)}</React.Fragment>);
    }
    return pieces;
  };

  const renderInlineWithRefs = (text: string, refs: Map<string, string>, keyPrefix: string) => {
    const tokens: React.ReactNode[] = [];
    const boldPattern = /(\*\*[^*]+\*\*)/g;
    let last = 0;
    let idx = 0;
    for (let match = boldPattern.exec(text); match !== null; match = boldPattern.exec(text)) {
      if (match.index > last) {
        const plain = text.slice(last, match.index);
        const renderedPlain = renderTextWithRefs(plain, refs, `${keyPrefix}-p-${idx}`);
        tokens.push(<React.Fragment key={`${keyPrefix}-p-${idx}`}>{renderedPlain}</React.Fragment>);
        idx += 1;
      }
      const boldRaw = match[0];
      const boldInner = boldRaw.slice(2, -2);
      const renderedBold = renderTextWithRefs(boldInner, refs, `${keyPrefix}-b-${idx}`);
      tokens.push(<strong key={`${keyPrefix}-b-${idx}`}>{renderedBold}</strong>);
      idx += 1;
      last = match.index + boldRaw.length;
    }
    if (last < text.length) {
      const tail = text.slice(last);
      const renderedTail = renderTextWithRefs(tail, refs, `${keyPrefix}-t-${idx}`);
      tokens.push(<React.Fragment key={`${keyPrefix}-t-${idx}`}>{renderedTail}</React.Fragment>);
    }
    if (tokens.length === 0) {
      return renderTextWithRefs(text, refs, `${keyPrefix}-raw`);
    }
    return tokens;
  };

  const renderStructuredNarrative = (body: string, refs: Map<string, string>) => {
    const lines = normalizeNarrativeBody(body).split('\n');
    const nodes: React.ReactNode[] = [];
    let paragraph: string[] = [];
    let paragraphClass = '';
    let listItems: string[] = [];
    let key = 0;
    const seenHeadingSignatures = new Set<string>();

    const flushParagraph = () => {
      if (paragraph.length === 0) {
        return;
      }
      nodes.push(
        <p
          key={`p-${key}`}
          className={`agent-narrative-paragraph${paragraphClass ? ` ${paragraphClass}` : ''}`}
        >
          {paragraph.map((line, index) => (
            <React.Fragment key={`p-${key}-l-${index}`}>
              {index > 0 && <br />}
              {renderInlineWithRefs(line, refs, `p-${key}-${index}`)}
            </React.Fragment>
          ))}
        </p>,
      );
      key += 1;
      paragraph = [];
      paragraphClass = '';
    };

    const flushList = () => {
      if (listItems.length === 0) {
        return;
      }
      nodes.push(
        <ul key={`ul-${key}`} className="agent-narrative-list">
          {listItems.map((item, index) => (
            <li key={`ul-${key}-i-${index}`}>
              {renderInlineWithRefs(item, refs, `ul-${key}-${index}`)}
            </li>
          ))}
        </ul>,
      );
      key += 1;
      listItems = [];
    };

    for (const rawLine of lines) {
      const line = rawLine.trim();
      if (!line) {
        flushParagraph();
        flushList();
        continue;
      }

      const headingMatch = line.match(/^(#{1,6})\s+(.+)$/);
      const boldHeadingMatch = line.match(/^\*\*(.+)\*\*$/);
      const quoteMatch = line.match(/^>\s+(.+)$/);
      const listMatch = line.match(/^[-*]\s+(.+)$/);

      if (headingMatch) {
        flushParagraph();
        flushList();
        nodes.push(
          <h3 key={`h-${key}`} className="agent-narrative-title">
            {renderInlineWithRefs(formatHeadingLabel(headingMatch[2]), refs, `h-${key}`)}
          </h3>,
        );
        key += 1;
        continue;
      }

      if (boldHeadingMatch) {
        flushParagraph();
        flushList();
        nodes.push(
          <h3 key={`bh-${key}`} className="agent-narrative-title">
            {renderInlineWithRefs(formatHeadingLabel(boldHeadingMatch[1]), refs, `bh-${key}`)}
          </h3>,
        );
        key += 1;
        continue;
      }

      if (quoteMatch) {
        flushParagraph();
        flushList();
        nodes.push(
          <blockquote key={`q-${key}`} className="agent-narrative-quote">
            {renderInlineWithRefs(quoteMatch[1], refs, `q-${key}`)}
          </blockquote>,
        );
        key += 1;
        continue;
      }

      if (listMatch) {
        flushParagraph();
        listItems.push(listMatch[1]);
        continue;
      }

      const inlineHeading = splitInlineHeadingLine(line);
      if (inlineHeading) {
        if (shouldDedupeHeading(inlineHeading.title)) {
          const signature = `${normalizeDedupText(inlineHeading.title)}|${normalizeDedupText(inlineHeading.rest)}`;
          if (signature !== '|' && seenHeadingSignatures.has(signature)) {
            continue;
          }
          seenHeadingSignatures.add(signature);
        }

        flushParagraph();
        flushList();
        nodes.push(
          <h3 key={`ih-${key}`} className={inlineHeading.primary ? 'agent-narrative-title' : 'agent-narrative-subtitle'}>
            {renderInlineWithRefs(formatHeadingLabel(inlineHeading.title, inlineHeading.primary), refs, `ih-${key}`)}
          </h3>,
        );
        key += 1;
        if (inlineHeading.rest) {
          paragraphClass = narrativeClassByHeading(inlineHeading.title);
          paragraph.push(inlineHeading.rest);
          flushParagraph();
        }
        continue;
      }

      flushList();
      paragraph.push(line);
    }

    flushParagraph();
    flushList();
    return nodes;
  };

  const renderAgentContent = (raw: string) => {
    const normalized = normalizeMarkdown(raw);
    if (!normalized) {
      return null;
    }

    const byRefBlock = parseReferenceBlock(normalized.split('\n'));
    if (byRefBlock.refs.size > 0) {
      const cleanedBody = byRefBlock.body
        .split('\n')
        .filter(line => !evidenceHeaderPattern.test(line.trim()))
        .join('\n')
        .trim();
      const normalizedBody = normalizeNarrativeBody(cleanedBody);
      const canHoverRefs = hasVisibleReferenceMarkers(normalizedBody, byRefBlock.refs);
      return (
        <>
          <div className="agent-narrative">{renderStructuredNarrative(normalizedBody, byRefBlock.refs)}</div>
          {canHoverRefs ? (
            <div className="agent-evidence-hint">悬停文中角标可查看依据</div>
          ) : (
            renderCollapsedReferenceBlock(byRefBlock.refs)
          )}
        </>
      );
    }

    const legacy = splitEvidenceTail(normalized);
    const compactEvidence = legacy.evidence.replace(evidenceLinePattern, '').trim();
    const cleanedLegacyBody = legacy.body
      .split('\n')
      .filter(line => !evidenceHeaderPattern.test(line.trim()))
      .join('\n')
      .trim();
    const normalizedLegacyBody = normalizeNarrativeBody(cleanedLegacyBody);
    const legacyRefs = parseEvidenceReferences(legacy.evidence);

    if (legacyRefs.size > 0) {
      const canHoverRefs = hasVisibleReferenceMarkers(normalizedLegacyBody, legacyRefs);
      return (
        <>
          {normalizedLegacyBody && (
            <div className="agent-narrative">{renderStructuredNarrative(normalizedLegacyBody, legacyRefs)}</div>
          )}
          {canHoverRefs ? (
            <div className="agent-evidence-hint">悬停文中角标可查看依据</div>
          ) : (
            renderCollapsedReferenceBlock(legacyRefs)
          )}
        </>
      );
    }

    const mixed = extractInlineEvidenceFromMixedContent(normalized);
    if (mixed.refs.size > 0 && mixed.body !== normalized) {
      const normalizedMixedBody = normalizeNarrativeBody(mixed.body);
      const canHoverRefs = hasVisibleReferenceMarkers(normalizedMixedBody, mixed.refs);
      return (
        <>
          {normalizedMixedBody && (
            <div className="agent-narrative">{renderStructuredNarrative(normalizedMixedBody, mixed.refs)}</div>
          )}
          {canHoverRefs ? (
            <div className="agent-evidence-hint">悬停文中角标可查看依据</div>
          ) : (
            renderCollapsedReferenceBlock(mixed.refs)
          )}
        </>
      );
    }

    return (
      <>
        {normalizedLegacyBody && (
          <div className="agent-narrative">{renderStructuredNarrative(normalizedLegacyBody, new Map<string, string>())}</div>
        )}
        {legacy.evidence && (
          <details className="agent-evidence agent-evidence-collapsed">
            <summary className="agent-evidence-summary">查看依据</summary>
            <div className="agent-evidence-text">{compactEvidence || legacy.evidence}</div>
          </details>
        )}
      </>
    );
  };

  // 进度状态
  const [progress, setProgress] = useState<ProgressState>({
    currentAgent: null,
    currentAgentName: null,
    steps: [],
    streamingText: '',
    lastStatus: '',
  });

  // 取消指定股票的会议
  const cancelMeeting = (stockCode: string) => {
    // 调用后端取消 API
    CancelMeeting(stockCode).catch((err: unknown) => {
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
      lastStatus: '',
    });
    showToast('讨论已停止', 'info');
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
      loadAgents();
    });
    return () => {
      EventsOff('strategy:changed');
      if (cleanup) cleanup();
    };
  }, []);

  // 当Session变化时，从后端加载最新消息
  useEffect(() => {
    // 记录之前的 stockCode 用于取消
    const prevStockCode = currentStockCodeRef.current;

    if (session?.stockCode) {
      // 如果切换到新股票，取消之前股票的会议
      if (prevStockCode && prevStockCode !== session.stockCode && simulatingMap[prevStockCode]) {
        cancelMeeting(prevStockCode);
        showToast('已切换股票，之前的会议已取消', 'info');
      }

      // 从后端获取最新消息（包括切换期间产生的新消息）
      getSessionMessages(session.stockCode).then(msgs => {
        setMessages(msgs || []);
      });
    } else {
      setMessages([]);
    }
    setUserQuery('');
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
              lastStatus: '',
            };
          case 'agent_done':
            return {
              ...prev,
              currentAgent: null,
              currentAgentName: null,
              steps: [],
              streamingText: '',
              lastStatus: event.detail ? `${event.agentName}: ${event.detail}` : '',
            };
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
          default:
            return prev;
        }
      });
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
      showToast(errorMsg, 'error');
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
      showToast('复制失败', 'error');
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
                    <div className={`text-sm p-3 rounded-2xl rounded-tl-none leading-relaxed shadow-sm agent-message-content moderator-message-content ${
                      isSummary
                        ? (colors.isDark ? 'bg-gradient-to-br from-amber-900/40 to-orange-900/30 border border-amber-500/30 text-amber-100' : 'bg-gradient-to-br from-amber-100 to-orange-100 border border-amber-400/30 text-amber-900')
                        : (colors.isDark ? 'bg-slate-800/70 border border-amber-500/20 text-slate-200' : 'bg-slate-100 border border-amber-400/20 text-slate-700')
                    }`}>
                      {renderAgentContent(msg.content)}
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
                style={{ backgroundColor: agent?.color || '#475569' }}
              >
                {agent?.avatar || msg.agentName?.charAt(0)}
              </div>
              <div className="flex-1 max-w-[85%]">
                <div className="flex items-baseline gap-2 mb-1">
                  <span className={`text-xs font-bold ${colors.isDark ? 'text-slate-300' : 'text-slate-600'}`}>{msg.agentName || agent?.name}</span>
                  <span className={`text-[9px] uppercase border fin-divider px-1 rounded fin-chip ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>{msg.role || agent?.role}</span>
                </div>
                <div className="relative">
                  <div className={`text-sm p-3 rounded-2xl rounded-tl-none leading-relaxed shadow-sm agent-message-content expert-message-content ${colors.isDark ? 'text-slate-200 bg-slate-800/70 border border-slate-700/40' : 'text-slate-700 bg-white border border-slate-200'}`}>
                    {renderAgentContent(msg.content)}
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
              <div className="flex flex-col items-center gap-1 justify-center">
                <div className="flex items-center gap-2">
                  <Loader2 className="animate-spin h-3 w-3 text-accent-2" />
                  <span className={`text-xs animate-pulse ${colors.isDark ? 'text-slate-500' : 'text-slate-400'}`}>会议进行中...</span>
                </div>
                {progress.lastStatus && (
                  <div className={`text-[11px] ${colors.isDark ? 'text-amber-400/90' : 'text-amber-600'}`}>
                    最近状态: {progress.lastStatus}
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Input Area */}
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

      {/* Toast 错误提示 */}
      {toast.show && (
        <div className="absolute top-4 left-1/2 -translate-x-1/2 z-50 animate-in fade-in slide-in-from-top-2 duration-300">
          <div className={`flex items-center gap-2 px-4 py-3 rounded-lg shadow-lg border ${
            toast.type === 'error'
              ? 'bg-red-900/90 border-red-500/50 text-red-100'
              : toast.type === 'warning'
              ? 'bg-amber-900/90 border-amber-500/50 text-amber-100'
              : 'bg-[var(--accent)]/90 border-accent/50 text-white'
          }`}>
            <AlertCircle size={18} />
            <span className="text-sm">{toast.message}</span>
            <button
              onClick={() => hideToast()}
              className="ml-2 hover:opacity-70"
            >
              <X size={14} />
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
