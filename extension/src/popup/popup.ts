const MAX_HISTORY = 5;
const STORAGE_KEY = 'yunkiro_dial_history';

const statusDot = document.getElementById('status-dot')!;
const phoneInput = document.getElementById('phone-input') as HTMLInputElement;
const dialBtn = document.getElementById('dial-btn') as HTMLButtonElement;
const historyList = document.getElementById('history-list')!;

function updateStatus(connected: boolean): void {
  statusDot.className = connected
    ? 'status-dot status-dot--connected'
    : 'status-dot status-dot--disconnected';
}

function queryStatus(): void {
  chrome.runtime.sendMessage({ action: 'getStatus' }, (response) => {
    if (response) {
      updateStatus(response.connected);
    }
  });
}

async function loadHistory(): Promise<string[]> {
  return new Promise((resolve) => {
    chrome.storage.local.get(STORAGE_KEY, (result) => {
      resolve(result[STORAGE_KEY] || []);
    });
  });
}

async function saveToHistory(number: string): Promise<void> {
  const history = await loadHistory();
  // Remove duplicate if exists
  const idx = history.indexOf(number);
  if (idx !== -1) history.splice(idx, 1);
  // Add to front
  history.unshift(number);
  // Keep last N
  const trimmed = history.slice(0, MAX_HISTORY);
  return new Promise((resolve) => {
    chrome.storage.local.set({ [STORAGE_KEY]: trimmed }, resolve);
  });
}

async function renderHistory(): Promise<void> {
  const history = await loadHistory();
  historyList.innerHTML = '';

  for (const number of history) {
    const li = document.createElement('li');
    const span = document.createElement('span');
    span.className = 'phone-number';
    span.textContent = number;
    const btn = document.createElement('button');
    btn.className = 'redial-btn';
    btn.textContent = 'Redial';
    btn.addEventListener('click', () => dial(number));
    li.appendChild(span);
    li.appendChild(btn);
    historyList.appendChild(li);
  }
}

function dial(phoneNumber: string): void {
  dialBtn.disabled = true;
  chrome.runtime.sendMessage(
    { action: 'dial', phoneNumber },
    async (response) => {
      dialBtn.disabled = false;
      if (response && response.success) {
        await saveToHistory(phoneNumber);
        await renderHistory();
      } else {
        const error = response?.error || 'Failed to dial';
        alert(error);
      }
    }
  );
}

dialBtn.addEventListener('click', () => {
  const number = phoneInput.value.trim();
  if (/^1[3-9]\d{9}$/.test(number)) {
    dial(number);
  } else {
    alert('Please enter a valid Chinese mobile number');
  }
});

phoneInput.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') {
    dialBtn.click();
  }
});

// Initialize
queryStatus();
renderHistory();
