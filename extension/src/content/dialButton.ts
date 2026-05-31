import { DetectedNumber } from './numberDetector';

const PHONE_SVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="14" height="14" fill="currentColor"><path d="M6.62 10.79a15.05 15.05 0 0 0 6.59 6.59l2.2-2.2a1 1 0 0 1 1.01-.24c1.12.37 2.33.57 3.58.57a1 1 0 0 1 1 1v3.49a1 1 0 0 1-1 1A17 17 0 0 1 3 4a1 1 0 0 1 1-1h3.5a1 1 0 0 1 1 1c0 1.25.2 2.45.57 3.57a1 1 0 0 1-.25 1.02l-2.2 2.2z"/></svg>`;

/**
 * Create a dial button element for a detected phone number.
 */
export function createDialButton(detected: DetectedNumber): HTMLSpanElement {
  const btn = document.createElement('span');
  btn.className = 'yunkiro-dial-btn';
  btn.title = `Dial ${detected.number}`;
  btn.innerHTML = PHONE_SVG;
  btn.dataset.phone = detected.number;

  btn.addEventListener('click', (e) => {
    e.preventDefault();
    e.stopPropagation();
    dialNumber(detected.number, btn);
  });

  return btn;
}

/**
 * Inject a dial button after the detected number in the DOM.
 */
export function injectDialButton(detected: DetectedNumber): HTMLSpanElement {
  const btn = createDialButton(detected);
  const { textNode, startOffset, number } = detected;
  const endOffset = startOffset + number.length;

  // Split the text node to insert button after the number
  const parent = textNode.parentNode;
  if (!parent) return btn;

  const afterText = textNode.splitText(endOffset);
  parent.insertBefore(btn, afterText);

  return btn;
}

function dialNumber(phoneNumber: string, btn: HTMLSpanElement): void {
  btn.classList.add('yunkiro-dial-btn--loading');

  chrome.runtime.sendMessage(
    { action: 'dial', phoneNumber },
    (response) => {
      btn.classList.remove('yunkiro-dial-btn--loading');

      if (response && response.success) {
        showTooltip(btn, 'Dialing...', 'success');
      } else {
        const error = response?.error || 'Failed to dial';
        showTooltip(btn, error, 'error');
      }
    }
  );
}

function showTooltip(btn: HTMLSpanElement, message: string, type: 'success' | 'error'): void {
  const tooltip = document.createElement('span');
  tooltip.className = `yunkiro-dial-tooltip yunkiro-dial-tooltip--${type}`;
  tooltip.textContent = message;
  btn.appendChild(tooltip);

  setTimeout(() => {
    tooltip.remove();
  }, 2000);
}
