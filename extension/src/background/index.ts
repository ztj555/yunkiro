import { WebSocketConnection, ConnectionState } from '../shared/connection';
import {
  createDialRequest,
  parseIncomingMessage,
  ExtDialResponse,
  ExtStatusResponse,
  PhoneDevice,
  IncomingMessage,
} from '../shared/protocol';

const WS_URL = 'ws://localhost:8765';

let connection: WebSocketConnection;
let connectionState: ConnectionState = 'disconnected';
let phones: PhoneDevice[] = [];
let pendingDialCallbacks: Array<(response: ExtDialResponse) => void> = [];

function initConnection(): void {
  connection = new WebSocketConnection(WS_URL, {
    onOpen() {
      connectionState = 'connected';
    },
    onClose() {
      connectionState = 'disconnected';
      phones = [];
    },
    onMessage(data: string) {
      handleIncomingMessage(data);
    },
    onError() {
      // Errors are handled by onClose
    },
  });
  connection.connect();
}

function handleIncomingMessage(data: string): void {
  const msg: IncomingMessage | null = parseIncomingMessage(data);
  if (!msg) return;

  switch (msg.type) {
    case 'dial_result': {
      const callback = pendingDialCallbacks.shift();
      if (callback) {
        callback({ success: msg.success, error: msg.error || undefined });
      }
      break;
    }
    case 'status': {
      phones = msg.phones;
      break;
    }
  }
}

function handleDial(phoneNumber: string): Promise<ExtDialResponse> {
  return new Promise((resolve) => {
    if (connectionState !== 'connected') {
      resolve({ success: false, error: 'Not connected to PC client' });
      return;
    }

    pendingDialCallbacks.push(resolve);

    const request = createDialRequest(phoneNumber);
    connection.send(JSON.stringify(request));

    // Timeout after 10 seconds
    setTimeout(() => {
      const idx = pendingDialCallbacks.indexOf(resolve);
      if (idx !== -1) {
        pendingDialCallbacks.splice(idx, 1);
        resolve({ success: false, error: 'Request timed out' });
      }
    }, 10000);
  });
}

function handleGetStatus(): ExtStatusResponse {
  return {
    connected: connectionState === 'connected',
    phones,
  };
}

// Listen for messages from content scripts and popup
chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message.action === 'dial') {
    handleDial(message.phoneNumber).then(sendResponse);
    return true; // async response
  }

  if (message.action === 'getStatus') {
    sendResponse(handleGetStatus());
    return false;
  }

  return false;
});

// Initialize connection on service worker start
initConnection();
