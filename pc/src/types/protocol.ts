// Protocol message types matching the relay server

export type ConnectionState =
  | "disconnected"
  | "connecting"
  | "connected"
  | "reconnecting";

export interface PhoneInfo {
  device_id: string;
  device_name: string;
}

// Messages PC sends
export interface AuthMessage {
  type: "auth";
  pin: string;
  role: "pc";
  device_id: string;
  device_name: string;
}

export interface DialMessage {
  type: "dial";
  message_id: string;
  phone_number: string;
  device_id: string;
  sim_slot: number;
}

export interface HangupMessage {
  type: "hangup";
  message_id: string;
  device_id: string;
}

export interface SMSMessage {
  type: "sms";
  message_id: string;
  phone_number: string;
  content: string;
  device_id: string;
}

export interface PingMessage {
  type: "ping";
}

// Messages PC receives
export interface AuthResult {
  type: "auth_result";
  success: boolean;
  message: string;
  online_phones?: PhoneInfo[];
}

export interface PhoneOnlineEvent {
  type: "phone_online";
  device_id: string;
  device_name: string;
}

export interface PhoneOfflineEvent {
  type: "phone_offline";
  device_id: string;
  device_name: string;
}

export interface DialResultEvent {
  type: "dial_result";
  message_id: string;
  success: boolean;
  error: string;
}

export interface SMSResultEvent {
  type: "sms_result";
  message_id: string;
  success: boolean;
  error: string;
}

export interface DeviceStatusEvent {
  type: "device_status";
  device_id: string;
  status: "idle" | "in_call" | "ringing";
}

export interface PongMessage {
  type: "pong";
}

export type IncomingMessage =
  | AuthResult
  | PhoneOnlineEvent
  | PhoneOfflineEvent
  | DialResultEvent
  | SMSResultEvent
  | DeviceStatusEvent
  | PongMessage;

export type OutgoingMessage =
  | AuthMessage
  | DialMessage
  | HangupMessage
  | SMSMessage
  | PingMessage;

// Phone state tracked by the frontend
export interface PhoneState {
  device_id: string;
  device_name: string;
  online: boolean;
  status: "idle" | "in_call" | "ringing";
}

// Relay connection status emitted from backend
export interface RelayStatus {
  state: ConnectionState;
  url: string;
  phone_count: number;
}
