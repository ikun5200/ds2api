'use strict';
const {
  resetIncrementalToolState,
  noteText,
  insideCodeFenceWithState,
} = require('./state');
const { parseStandaloneToolCallsDetailed } = require('./parse');
const { extractJSONObjectFrom, trimWrappingJSONFence } = require('./jsonscan');
const {
  TOOL_SEGMENT_KEYWORDS,
  XML_TOOL_SEGMENT_TAGS,
  earliestKeywordIndex,
} = require('./tool-keywords');
const {
  consumeXMLToolCapture: consumeXMLToolCaptureImpl,
  hasOpenXMLToolTag,
  findPartialXMLToolTagStart,
  looksLikeXMLToolTagFragment,
} = require('./sieve-xml');
function processToolSieveChunk(state, chunk, toolNames) {
  if (!state) {
    return [];
  }
  if (chunk) {
    state.pending += chunk;
  }
  const events = [];
  while (true) {
    if (Array.isArray(state.pendingToolCalls) && state.pendingToolCalls.length > 0) {
      events.push({ type: 'tool_calls', calls: state.pendingToolCalls });
      state.pendingToolRaw = '';
      state.pendingToolCalls = [];
      continue;
    }
    if (state.capturing) {
      if (state.pending) {
        state.capture += state.pending;
        state.pending = '';
      }
      const consumed = consumeToolCapture(state, toolNames);
      if (!consumed.ready) {
        break;
      }
      const captured = state.capture;
      state.capture = '';
      state.capturing = false;
      resetIncrementalToolState(state);

      if (Array.isArray(consumed.calls) && consumed.calls.length > 0) {
        state.pendingToolRaw = captured;
        state.pendingToolCalls = consumed.calls;
        if (consumed.suffix) {
          state.pending = consumed.suffix + state.pending;
        }
        continue;
      }
      if (consumed.prefix) {
        noteText(state, consumed.prefix);
        events.push({ type: 'text', text: consumed.prefix });
      }
      if (consumed.suffix) {
        state.pending += consumed.suffix;
      }
      continue;
    }
    const pending = state.pending || '';
    if (!pending) {
      break;
    }
    const start = findToolSegmentStart(state, pending);
    if (start >= 0) {
      const prefix = pending.slice(0, start);
      if (prefix) {
        noteText(state, prefix);
        events.push({ type: 'text', text: prefix });
      }
      state.pending = '';
      state.capture += pending.slice(start);
      state.capturing = true;
      resetIncrementalToolState(state);
      continue;
    }
    const [safe, hold] = splitSafeContentForToolDetection(pending);
    if (!safe) {
      break;
    }
    state.pending = hold;
    noteText(state, safe);
    events.push({ type: 'text', text: safe });
  }
  return events;
}

function flushToolSieve(state, toolNames) {
  if (!state) {
    return [];
  }
  const events = processToolSieveChunk(state, '', toolNames);
  if (Array.isArray(state.pendingToolCalls) && state.pendingToolCalls.length > 0) {
    events.push({ type: 'tool_calls', calls: state.pendingToolCalls });
    state.pendingToolRaw = '';
    state.pendingToolCalls = [];
  }
  if (state.capturing) {
    const consumed = consumeToolCapture(state, toolNames);
    if (consumed.ready) {
      if (consumed.prefix) {
        noteText(state, consumed.prefix);
        events.push({ type: 'text', text: consumed.prefix });
      }
      if (Array.isArray(consumed.calls) && consumed.calls.length > 0) {
        events.push({ type: 'tool_calls', calls: consumed.calls });
      }
      if (consumed.suffix) {
        noteText(state, consumed.suffix);
        events.push({ type: 'text', text: consumed.suffix });
      }
    } else if (state.capture) {
      const content = state.capture;
      if (!hasOpenXMLToolTag(content) && !looksLikeXMLToolTagFragment(content)) {
        noteText(state, content);
        events.push({ type: 'text', text: content });
      }
    }
    state.capture = '';
    state.capturing = false;
    resetIncrementalToolState(state);
  }
  if (state.pending) {
    if (!hasOpenXMLToolTag(state.pending) && !looksLikeXMLToolTagFragment(state.pending)) {
      noteText(state, state.pending);
      events.push({ type: 'text', text: state.pending });
    }
    state.pending = '';
  }
  return events;
}

function splitSafeContentForToolDetection(s) {
  const text = s || '';
  if (!text) {
    return ['', ''];
  }
  const suspiciousStart = findSuspiciousPrefixStart(text);
  if (suspiciousStart < 0) {
    return [text, ''];
  }
  if (suspiciousStart > 0) {
    return [text.slice(0, suspiciousStart), text.slice(suspiciousStart)];
  }
  return ['', text];
}

function findSuspiciousPrefixStart(s) {
  let start = -1;
  for (const needle of ['{', '[', '```']) {
    const idx = s.lastIndexOf(needle);
    if (idx > start) {
      start = idx;
    }
  }
  // Also check for partial XML tool tag at end of string.
  const xmlIdx = findPartialXMLToolTagStart(s);
  if (xmlIdx >= 0 && xmlIdx > start) {
    start = xmlIdx;
  }
  return start;
}

function findToolSegmentStart(state, s) {
  if (!s) {
    return -1;
  }
  const lower = s.toLowerCase();
  let offset = 0;
  while (true) {
    // Check JSON keywords.
    let { index: bestKeyIdx, keyword: matchedKeyword } = earliestKeywordIndex(lower, TOOL_SEGMENT_KEYWORDS, offset);
    // Also check XML tool tags.
    for (const tag of XML_TOOL_SEGMENT_TAGS) {
      const idx = lower.indexOf(tag, offset);
      if (idx >= 0 && (bestKeyIdx < 0 || idx < bestKeyIdx)) {
        bestKeyIdx = idx;
        matchedKeyword = tag;
      }
    }
    if (bestKeyIdx < 0) {
      return -1;
    }
    // For XML tags, the '<' is itself the segment start.
    if (s[bestKeyIdx] === '<') {
      if (!insideCodeFenceWithState(state, s.slice(0, bestKeyIdx))) {
        return bestKeyIdx;
      }
      offset = bestKeyIdx + matchedKeyword.length;
      continue;
    }
    const keyIdx = bestKeyIdx;
    const start = s.slice(0, keyIdx).lastIndexOf('{');
    let candidateStart = start >= 0 ? start : keyIdx;
    // If the keyword matched inside an XML tag (e.g. "tool_calls" in "<tool_calls>"),
    // back up past the '<' to capture the full tag.
    if (candidateStart > 0 && s[candidateStart - 1] === '<') {
      candidateStart--;
    }
    if (!insideCodeFenceWithState(state, s.slice(0, candidateStart))) {
      return candidateStart;
    }
    offset = keyIdx + matchedKeyword.length;
  }
}

function consumeToolCapture(state, toolNames) {
  const captured = state.capture || '';
  if (!captured) {
    return { ready: false, prefix: '', calls: [], suffix: '' };
  }

  // Try XML tool call extraction first.
  const xmlResult = consumeXMLToolCaptureImpl(captured, toolNames, trimWrappingJSONFence);
  if (xmlResult.ready) {
    return xmlResult;
  }
  // If XML tags are present but block is incomplete, keep buffering.
  if (hasOpenXMLToolTag(captured)) {
    return { ready: false, prefix: '', calls: [], suffix: '' };
  }

  const lower = captured.toLowerCase();
  const { index: keyIdx } = earliestKeywordIndex(lower, TOOL_SEGMENT_KEYWORDS);
  if (keyIdx < 0) {
    return { ready: false, prefix: '', calls: [], suffix: '' };
  }
  const start = captured.slice(0, keyIdx).lastIndexOf('{');
  const actualStart = start >= 0 ? start : keyIdx;
  const obj = extractJSONObjectFrom(captured, actualStart);
  if (!obj.ok) {
    return { ready: false, prefix: '', calls: [], suffix: '' };
  }
  const prefixPart = captured.slice(0, actualStart);
  const suffixPart = captured.slice(obj.end);
  if (insideCodeFenceWithState(state, prefixPart)) {
    return {
      ready: true,
      prefix: captured,
      calls: [],
      suffix: '',
    };
  }
  const parsed = parseStandaloneToolCallsDetailed(captured.slice(actualStart, obj.end), toolNames);
  if (!Array.isArray(parsed.calls) || parsed.calls.length === 0) {
    if (parsed.sawToolCallSyntax && parsed.rejectedByPolicy) {
      return {
        ready: true,
        prefix: prefixPart,
        calls: [],
        suffix: suffixPart,
      };
    }
    return {
      ready: true,
      prefix: captured,
      calls: [],
      suffix: '',
    };
  }
  const trimmedFence = trimWrappingJSONFence(prefixPart, suffixPart);
  return {
    ready: true,
    prefix: trimmedFence.prefix,
    calls: parsed.calls,
    suffix: trimmedFence.suffix,
  };
}

module.exports = {
  processToolSieveChunk,
  flushToolSieve,
};
