export interface DetectedNumber {
  number: string;
  textNode: Text;
  startOffset: number;
}

const PHONE_REGEX = /1[3-9]\d{9}/g;
const EXCLUDED_TAGS = new Set([
  'INPUT', 'TEXTAREA', 'SCRIPT', 'STYLE', 'CODE', 'PRE', 'NOSCRIPT',
]);

/**
 * Check if a text node is inside an excluded element.
 */
function isExcluded(node: Text): boolean {
  let parent = node.parentElement;
  while (parent) {
    if (EXCLUDED_TAGS.has(parent.tagName)) {
      return true;
    }
    if (parent.isContentEditable) {
      return true;
    }
    parent = parent.parentElement;
  }
  return false;
}

/**
 * Check if a matched phone number is part of a longer digit sequence.
 */
function isPartOfLongerNumber(text: string, matchStart: number, matchEnd: number): boolean {
  if (matchStart > 0 && /\d/.test(text[matchStart - 1])) {
    return true;
  }
  if (matchEnd < text.length && /\d/.test(text[matchEnd])) {
    return true;
  }
  return false;
}

/**
 * Scan a single text node for Chinese mobile phone numbers.
 */
function scanTextNode(textNode: Text): DetectedNumber[] {
  const results: DetectedNumber[] = [];
  const text = textNode.textContent || '';

  if (text.length < 11) return results;

  PHONE_REGEX.lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = PHONE_REGEX.exec(text)) !== null) {
    const matchStart = match.index;
    const matchEnd = matchStart + match[0].length;

    if (isPartOfLongerNumber(text, matchStart, matchEnd)) {
      continue;
    }

    results.push({
      number: match[0],
      textNode,
      startOffset: matchStart,
    });
  }

  return results;
}

/**
 * Walks all text nodes under the given root and detects phone numbers.
 */
export function detectPhoneNumbers(root: Node): DetectedNumber[] {
  const results: DetectedNumber[] = [];
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, null);

  let node: Text | null;
  while ((node = walker.nextNode() as Text | null)) {
    if (isExcluded(node)) continue;
    const detected = scanTextNode(node);
    results.push(...detected);
  }

  return results;
}
