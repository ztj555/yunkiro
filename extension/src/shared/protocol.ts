/** Messages sent from extension to PC client */
export interface DialRequest {
  type: 'dial';
  phone_number: string;
}

/** Messages received from PC client */
export interface DialResult {
  type: 'dial_result';
  success: boolean;
  error: string;
}

export interface StatusResponse {
  type: 'status';
  connected: boolean;
  phones: PhoneDevice[];
}

export interface PhoneDevice {
  device_id: string;
  device_name: string;
  online: boolean;
}

/** All possible messages from PC client */
export type IncomingMessage = DialResult | StatusResponse;

/** All possible messages to PC client */
export type OutgoingMessage = DialRequest;

/** Internal messages between extension components */
export interface ExtDialRequest {
  action: 'dial';
  phoneNumber: string;
}

export interface ExtStatusQuery {
  action: 'getStatus';
}

export interface ExtStatusResponse {
  connected: boolean;
  phones: PhoneDevice[];
}

export interface ExtDialResponse {
  success: boolean;
  error?: string;
}

export type ExtMessage = ExtDialRequest | ExtStatusQuery;

export function createDialRequest(phoneNumber: string): DialRequest {
  return {
    type: 'dial',
    phone_number: phoneNumber,
  };
}

export function parseIncomingMessage(data: string): IncomingMessage | null {
  try {
    const msg = JSON.parse(data);
    if (msg.type === 'dial_result' || msg.type === 'status') {
      return msg as IncomingMessage;
    }
    return null;
  } catch {
    return null;
  }
}
