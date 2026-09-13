'use strict';

const { parseChunkForContent } = require('./sse_parse');

function createDeltaState() {
  return { path: '', op: 'SET' };
}

function expandDelta(state, chunk) {
  if (!Object.prototype.hasOwnProperty.call(chunk, 'v')) return [chunk];
  if (chunk.v && typeof chunk.v.response === 'object' && chunk.p == null) {
    // Older automatic continuations can replay a snapshot without ready.
    state.path = '';
    state.op = 'SET';
  }
  if (typeof chunk.p === 'string') state.path = chunk.p;
  if (typeof chunk.o === 'string') state.op = chunk.o;
  if (state.op === 'BATCH' && Array.isArray(chunk.v)) {
    const nested = createDeltaState();
    return chunk.v.flatMap((item) => {
      if (!item || typeof item !== 'object') return [];
      return expandDelta(nested, item).map((delta) => ({
        ...delta,
        p: state.path ? `${state.path}/${delta.p || ''}` : delta.p,
      }));
    });
  }
  return [{ ...chunk, p: state.path, o: state.op }];
}

const metadataEvents = new Set([
  'ready', 'update_file', 'update_session', 'update_parent_message',
  'title', 'hint', 'toast', 'debug', 'close', 'finish',
]);

function createDeepSeekSSEParser() {
  let state = createDeltaState();
  let event = '';
  return {
    parse(rawLine, thinkingEnabled, currentType, stripReferenceMarkers = true) {
      const line = rawLine.trim();
      const result = { parsed: false, parts: [], chunks: [], finished: false, newType: currentType };
      if (line.startsWith('event:')) {
        event = line.slice(6).trim();
        if (event === 'ready') state = createDeltaState();
        return result;
      }
      if (!line) event = '';
      if (!line.startsWith('data:')) return result;
      const eventName = event;
      event = '';
      const data = line.slice(5).trim();
      if (data === '[DONE]') return { ...result, parsed: true, finished: true };
      let chunk;
      try {
        chunk = JSON.parse(data);
      } catch (_err) {
        return result;
      }
      if (!chunk || typeof chunk !== 'object') return result;
      result.parsed = true;
      if (metadataEvents.has(eventName)) {
        // close/finish may precede automatic continuation. Status ends output.
        result.chunks = [chunk];
        return result;
      }
      result.chunks = expandDelta(state, chunk);
      for (const delta of result.chunks) {
        const parsed = parseChunkForContent(delta, thinkingEnabled, result.newType, stripReferenceMarkers);
        result.parts.push(...parsed.parts);
        result.newType = parsed.newType;
        result.finished = parsed.finished;
        result.contentFilter = parsed.contentFilter;
        result.errorMessage = parsed.errorMessage;
        if (parsed.finished) break;
      }
      return result;
    },
  };
}

module.exports = { createDeepSeekSSEParser };
