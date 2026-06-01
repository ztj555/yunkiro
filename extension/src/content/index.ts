import './styles.css';
import { detectPhoneNumbers, DetectedNumber } from './numberDetector';
import { injectDialButton } from './dialButton';

const processedNodes = new WeakSet<Node>();

/**
 * Scan a root node for phone numbers and inject dial buttons.
 */
function scanAndInject(root: Node): void {
  const detected = detectPhoneNumbers(root);

  // Process in reverse order so text node offsets remain valid
  const sorted = detected.sort((a, b) => {
    if (a.textNode === b.textNode) {
      return b.startOffset - a.startOffset;
    }
    return 0;
  });

  for (const item of sorted) {
    if (processedNodes.has(item.textNode)) continue;
    processedNodes.add(item.textNode);
    injectDialButton(item);
  }
}

/**
 * Observe DOM mutations to detect dynamically loaded content.
 */
function observeMutations(): void {
  const observer = new MutationObserver((mutations) => {
    for (const mutation of mutations) {
      for (const node of mutation.addedNodes) {
        if (node.nodeType === Node.ELEMENT_NODE || node.nodeType === Node.TEXT_NODE) {
          scanAndInject(node);
        }
      }
    }
  });

  observer.observe(document.body, {
    childList: true,
    subtree: true,
  });
}

/**
 * Initialize content script.
 */
function init(): void {
  scanAndInject(document.body);
  observeMutations();
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
