'use strict';

const TOOL_SEGMENT_KEYWORDS = [
  'tool_calls',
  '"function"',
  'function.name:',
];

const XML_TOOL_SEGMENT_TAGS = [
  '<tool_calls>', '<tool_calls\n', '<tool_call>', '<tool_call\n',
  '<invoke ', '<invoke>', '<function_call', '<function_calls', '<tool_use>',
];

const XML_TOOL_OPENING_TAGS = [
  '<tool_calls', '<tool_call', '<invoke', '<function_call', '<function_calls', '<tool_use',
];

const XML_TOOL_CLOSING_TAGS = [
  '</tool_calls>', '</tool_call>', '</invoke>', '</function_call>', '</function_calls>', '</tool_use>',
];

function earliestKeywordIndex(text, keywords = TOOL_SEGMENT_KEYWORDS, offset = 0) {
  if (!text) {
    return { index: -1, keyword: '' };
  }
  let index = -1;
  let keyword = '';
  for (const kw of keywords) {
    const candidate = text.indexOf(kw, offset);
    if (candidate >= 0 && (index < 0 || candidate < index)) {
      index = candidate;
      keyword = kw;
    }
  }
  return { index, keyword };
}

module.exports = {
  TOOL_SEGMENT_KEYWORDS,
  XML_TOOL_SEGMENT_TAGS,
  XML_TOOL_OPENING_TAGS,
  XML_TOOL_CLOSING_TAGS,
  earliestKeywordIndex,
};
