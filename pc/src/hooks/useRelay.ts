import { useState, useCallback, useEffect, useRef } from "react";
import type { ConnectionState, PhoneState, IncomingMessage } from "../types/protocol";

// Check if we're running inside Tauri
function isTauri(): boolean {
  return typeof window !== "undefined" && "__TAURI_INTERNALS__" in window;
}

interface RelayHook {
  connectionState: ConnectionState;
  phones: PhoneState[];
  connect: (url: string, pin: string) => void;
  disconnect: () => void;
  dial: (number: string, deviceId: string, simSlot: number) => void;
  hangup: (deviceId: string) => void;
  sendSMS: (number: string, content: string, deviceId: string) => void;
}

export function useRelay(): RelayHook {
  const [connectionState, setConnectionState] =
    useState<ConnectionState>("disconnected");
  const [phones, setPhones] = useState<PhoneState[]>([]);
  const wsRef = useRef<WebSocket | null>(null);
  const pingIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectAttemptRef = useRef(0);
  const urlRef = useRef("");
  const pinRef = useRef("");

  const cleanup = useCallback(() => {
    if (pingIntervalRef.current) {
      clearInterval(pingIntervalRef.current);
      pingIntervalRef.current = null;
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
  }, []);

  const generateId = (): string => {
    return crypto.randomUUID ? crypto.randomUUID() : Math.random().toString(36).slice(2);
  };

  const handleMessage = useCallback((data: string) => {
    let msg: IncomingMessage;
    try {
      msg = JSON.parse(data);
    } catch {
      return;
    }

    switch (msg.type) {
      case "auth_result":
        if (msg.success) {
          setConnectionState("connected");
          reconnectAttemptRef.current = 0;
          if (msg.online_phones) {
            setPhones(
              msg.online_phones.map((p) => ({
                device_id: p.device_id,
                device_name: p.device_name,
                online: true,
                status: "idle" as const,
              }))
            );
          }
        } else {
          setConnectionState("disconnected");
        }
        break;
      case "phone_online":
        setPhones((prev) => {
          const existing = prev.find((p) => p.device_id === msg.device_id);
          if (existing) {
            return prev.map((p) =>
              p.device_id === msg.device_id ? { ...p, online: true } : p
            );
          }
          return [
            ...prev,
            {
              device_id: msg.device_id,
              device_name: msg.device_name,
              online: true,
              status: "idle" as const,
            },
          ];
        });
        break;
      case "phone_offline":
        setPhones((prev) =>
          prev.map((p) =>
            p.device_id === msg.device_id ? { ...p, online: false } : p
          )
        );
        break;
      case "device_status":
        setPhones((prev) =>
          prev.map((p) =>
            p.device_id === msg.device_id ? { ...p, status: msg.status } : p
          )
        );
        break;
      case "dial_result":
      case "sms_result":
        // Could emit notifications here
        break;
      case "pong":
        break;
    }
  }, []);

  const connectWs = useCallback(
    (url: string, pin: string) => {
      if (isTauri()) {
        // In Tauri, use invoke commands
        import("@tauri-apps/api/core").then(({ invoke }) => {
          invoke("connect", { url, pin }).catch(console.error);
        });
        return;
      }

      // Browser fallback: direct WebSocket for dev/testing
      cleanup();
      setConnectionState("connecting");

      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => {
        const deviceId = localStorage.getItem("device_id") || generateId();
        localStorage.setItem("device_id", deviceId);
        ws.send(
          JSON.stringify({
            type: "auth",
            pin,
            role: "pc",
            device_id: deviceId,
            device_name: "PC Browser",
          })
        );

        // Start ping interval
        pingIntervalRef.current = setInterval(() => {
          if (ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: "ping" }));
          }
        }, 30000);
      };

      ws.onmessage = (event) => {
        handleMessage(event.data);
      };

      ws.onclose = () => {
        cleanup();
        setConnectionState("reconnecting");
        const attempt = reconnectAttemptRef.current;
        const delay = Math.min(1000 * Math.pow(2, attempt), 30000);
        reconnectAttemptRef.current = attempt + 1;
        reconnectTimeoutRef.current = setTimeout(() => {
          connectWs(urlRef.current, pinRef.current);
        }, delay);
      };

      ws.onerror = () => {
        // onclose will fire after this
      };
    },
    [cleanup, handleMessage]
  );

  const connect = useCallback(
    (url: string, pin: string) => {
      urlRef.current = url;
      pinRef.current = pin;
      reconnectAttemptRef.current = 0;
      connectWs(url, pin);
    },
    [connectWs]
  );

  const disconnect = useCallback(() => {
    reconnectAttemptRef.current = 999; // prevent reconnect
    cleanup();
    setConnectionState("disconnected");
    setPhones([]);
  }, [cleanup]);

  const sendCommand = useCallback((msg: object) => {
    if (isTauri()) {
      import("@tauri-apps/api/core").then(({ invoke }) => {
        invoke("send_message", { message: JSON.stringify(msg) }).catch(
          console.error
        );
      });
      return;
    }
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(msg));
    }
  }, []);

  const dial = useCallback(
    (number: string, deviceId: string, simSlot: number) => {
      sendCommand({
        type: "dial",
        message_id: generateId(),
        phone_number: number,
        device_id: deviceId,
        sim_slot: simSlot,
      });
    },
    [sendCommand]
  );

  const hangup = useCallback(
    (deviceId: string) => {
      sendCommand({
        type: "hangup",
        message_id: generateId(),
        device_id: deviceId,
      });
    },
    [sendCommand]
  );

  const sendSMS = useCallback(
    (number: string, content: string, deviceId: string) => {
      sendCommand({
        type: "sms",
        message_id: generateId(),
        phone_number: number,
        content,
        device_id: deviceId,
      });
    },
    [sendCommand]
  );

  // Listen for Tauri events when running in Tauri
  useEffect(() => {
    if (!isTauri()) return;

    let unlisten: (() => void)[] = [];

    import("@tauri-apps/api/event").then(({ listen }) => {
      const setupListeners = async () => {
        unlisten.push(
          await listen<string>("relay-status-changed", (event) => {
            setConnectionState(event.payload as ConnectionState);
          })
        );
        unlisten.push(
          await listen<string>("phone-online", (event) => {
            const data = JSON.parse(event.payload);
            setPhones((prev) => {
              const existing = prev.find(
                (p) => p.device_id === data.device_id
              );
              if (existing) {
                return prev.map((p) =>
                  p.device_id === data.device_id ? { ...p, online: true } : p
                );
              }
              return [
                ...prev,
                {
                  device_id: data.device_id,
                  device_name: data.device_name,
                  online: true,
                  status: "idle" as const,
                },
              ];
            });
          })
        );
        unlisten.push(
          await listen<string>("phone-offline", (event) => {
            const data = JSON.parse(event.payload);
            setPhones((prev) =>
              prev.map((p) =>
                p.device_id === data.device_id ? { ...p, online: false } : p
              )
            );
          })
        );
        unlisten.push(
          await listen<string>("device-status", (event) => {
            const data = JSON.parse(event.payload);
            setPhones((prev) =>
              prev.map((p) =>
                p.device_id === data.device_id
                  ? { ...p, status: data.status }
                  : p
              )
            );
          })
        );
        unlisten.push(
          await listen<string>("auth-result", (event) => {
            const data = JSON.parse(event.payload);
            if (data.success) {
              setConnectionState("connected");
              if (data.online_phones) {
                setPhones(
                  data.online_phones.map(
                    (p: { device_id: string; device_name: string }) => ({
                      device_id: p.device_id,
                      device_name: p.device_name,
                      online: true,
                      status: "idle" as const,
                    })
                  )
                );
              }
            }
          })
        );
      };
      setupListeners();
    });

    return () => {
      unlisten.forEach((fn) => fn());
    };
  }, []);

  return {
    connectionState,
    phones,
    connect,
    disconnect,
    dial,
    hangup,
    sendSMS,
  };
}
