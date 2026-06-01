use futures_util::{SinkExt, StreamExt};
use serde_json::Value;
use std::sync::Arc;
use std::time::Duration;
use tokio::net::TcpListener;
use tokio::sync::{oneshot, Mutex};
use tokio_tungstenite::accept_async;
use tokio_tungstenite::tungstenite::Message;

use crate::relay::{BridgeResult, RelayInner};

const BRIDGE_PORT: u16 = 8765;

/// How long the bridge waits for the phone's real dial/sms result before
/// reporting a timeout to the extension. Kept below the extension-side timeout.
const BRIDGE_RESULT_TIMEOUT_SECS: u64 = 8;

/// Builds an error dial_result payload for the extension.
fn err_dial(error: &str) -> String {
    serde_json::json!({
        "type": "dial_result",
        "success": false,
        "error": error
    })
    .to_string()
}

/// Starts a local WebSocket server on port 8765 that bridges the browser extension
/// protocol to the relay protocol. The extension sends simplified messages like
/// `{"type":"dial","phone_number":"..."}` and this bridge translates them into
/// full relay protocol messages and forwards responses back.
pub async fn start_bridge(inner: Arc<Mutex<RelayInner>>) {
    let addr = format!("127.0.0.1:{}", BRIDGE_PORT);
    let listener = match TcpListener::bind(&addr).await {
        Ok(l) => l,
        Err(e) => {
            eprintln!("[bridge] Failed to bind on {}: {}", addr, e);
            return;
        }
    };

    loop {
        let (stream, _) = match listener.accept().await {
            Ok(s) => s,
            Err(_) => continue,
        };

        let inner_clone = inner.clone();
        tokio::spawn(async move {
            let ws_stream = match accept_async(stream).await {
                Ok(ws) => ws,
                Err(_) => return,
            };
            handle_extension_client(ws_stream, inner_clone).await;
        });
    }
}

async fn handle_extension_client(
    ws_stream: tokio_tungstenite::WebSocketStream<tokio::net::TcpStream>,
    inner: Arc<Mutex<RelayInner>>,
) {
    let (mut write, mut read) = ws_stream.split();

    while let Some(Ok(msg)) = read.next().await {
        if let Message::Text(text) = msg {
            let response = handle_extension_message(&text, &inner).await;
            if let Some(resp) = response {
                if write.send(Message::Text(resp)).await.is_err() {
                    break;
                }
            }
        }
    }
}

async fn handle_extension_message(
    text: &str,
    inner: &Arc<Mutex<RelayInner>>,
) -> Option<String> {
    let msg: Value = serde_json::from_str(text).ok()?;
    let msg_type = msg.get("type")?.as_str()?;

    match msg_type {
        "dial" => {
            let phone_number = msg.get("phone_number")?.as_str()?.to_string();

            // Resolve the target phone (prefer the PC's active device, else the
            // first online phone) and the relay sender under a short lock.
            let (device_id, sender) = {
                let relay = inner.lock().await;
                let target = relay
                    .active_device_id
                    .as_ref()
                    .and_then(|id| {
                        relay
                            .phones
                            .iter()
                            .find(|p| p.online && &p.device_id == id)
                    })
                    .or_else(|| relay.phones.iter().find(|p| p.online));

                let device_id = match target {
                    Some(p) => p.device_id.clone(),
                    None => return Some(err_dial("No phone connected")),
                };
                let sender = match relay.sender.clone() {
                    Some(s) => s,
                    None => return Some(err_dial("Not connected to relay")),
                };
                (device_id, sender)
            };

            // Register a waiter for the real result BEFORE sending, keyed by
            // message_id, so the phone's dial_result is routed back to us.
            let message_id = uuid::Uuid::new_v4().to_string();
            let (tx, rx) = oneshot::channel::<BridgeResult>();
            {
                let mut relay = inner.lock().await;
                relay.pending_bridge.insert(message_id.clone(), tx);
            }

            let relay_msg = serde_json::json!({
                "type": "dial",
                "message_id": message_id,
                "phone_number": phone_number,
                "device_id": device_id,
                "sim_slot": 0
            });

            if sender.send(relay_msg.to_string()).is_err() {
                let mut relay = inner.lock().await;
                relay.pending_bridge.remove(&message_id);
                return Some(err_dial("Failed to send to relay"));
            }

            // Wait for the phone's real dial_result (bounded), then report the
            // true outcome to the extension instead of a premature success.
            let result =
                tokio::time::timeout(Duration::from_secs(BRIDGE_RESULT_TIMEOUT_SECS), rx).await;

            // Clean up the pending entry (no-op if already fulfilled).
            {
                let mut relay = inner.lock().await;
                relay.pending_bridge.remove(&message_id);
            }

            match result {
                Ok(Ok(br)) => Some(
                    serde_json::json!({
                        "type": "dial_result",
                        "success": br.success,
                        "error": br.error
                    })
                    .to_string(),
                ),
                // Timed out, or the sender was dropped (e.g. relay disconnected).
                _ => Some(err_dial("Dial timed out")),
            }
        }
        "get_status" => {
            let relay = inner.lock().await;
            let connected = relay.sender.is_some();
            let phones: Vec<Value> = relay
                .phones
                .iter()
                .map(|p| {
                    serde_json::json!({
                        "device_id": p.device_id,
                        "device_name": p.device_name,
                        "online": p.online
                    })
                })
                .collect();

            let resp = serde_json::json!({
                "type": "status",
                "connected": connected,
                "phones": phones
            });
            Some(resp.to_string())
        }
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::relay::{BridgeResult, ConnectionState, PhoneInfo, RelayInner};
    use std::collections::HashMap;
    use tokio::sync::mpsc;

    fn online_phone() -> PhoneInfo {
        PhoneInfo {
            device_id: "phone-001".to_string(),
            device_name: "Test Phone".to_string(),
            online: true,
            status: "idle".to_string(),
        }
    }

    fn make_inner(
        phones: Vec<PhoneInfo>,
        sender: Option<mpsc::UnboundedSender<String>>,
    ) -> RelayInner {
        RelayInner {
            state: ConnectionState::Connected,
            phones,
            sender,
            should_reconnect: false,
            device_id: "pc-test".to_string(),
            active_device_id: None,
            pending_bridge: HashMap::new(),
        }
    }

    #[tokio::test]
    async fn test_handle_dial_no_phone() {
        let inner = Arc::new(Mutex::new(make_inner(Vec::new(), None)));

        let msg = r#"{"type":"dial","phone_number":"13800138000"}"#;
        let result = handle_extension_message(msg, &inner).await;
        assert!(result.is_some());
        let parsed: Value = serde_json::from_str(&result.unwrap()).unwrap();
        assert_eq!(parsed["type"], "dial_result");
        assert_eq!(parsed["success"], false);
        assert!(parsed["error"].as_str().unwrap().contains("No phone"));
    }

    #[tokio::test]
    async fn test_handle_dial_with_phone() {
        let (tx, mut rx) = mpsc::unbounded_channel::<String>();
        let inner = Arc::new(Mutex::new(make_inner(vec![online_phone()], Some(tx))));

        // Simulate the relay/phone replying with a successful dial_result.
        let inner2 = inner.clone();
        let relay_task = tokio::spawn(async move {
            if let Some(text) = rx.recv().await {
                let v: Value = serde_json::from_str(&text).unwrap();
                let message_id = v["message_id"].as_str().unwrap().to_string();
                let tx = {
                    let mut relay = inner2.lock().await;
                    relay.pending_bridge.remove(&message_id)
                };
                if let Some(tx) = tx {
                    let _ = tx.send(BridgeResult {
                        success: true,
                        error: String::new(),
                    });
                }
            }
        });

        let msg = r#"{"type":"dial","phone_number":"13800138000"}"#;
        let result = handle_extension_message(msg, &inner).await;
        let _ = relay_task.await;

        assert!(result.is_some());
        let parsed: Value = serde_json::from_str(&result.unwrap()).unwrap();
        assert_eq!(parsed["type"], "dial_result");
        assert_eq!(parsed["success"], true);
    }

    #[tokio::test]
    async fn test_handle_dial_targets_active_device() {
        let (tx, mut rx) = mpsc::unbounded_channel::<String>();
        let phones = vec![
            PhoneInfo {
                device_id: "phone-A".to_string(),
                device_name: "A".to_string(),
                online: true,
                status: "idle".to_string(),
            },
            PhoneInfo {
                device_id: "phone-B".to_string(),
                device_name: "B".to_string(),
                online: true,
                status: "idle".to_string(),
            },
        ];
        let mut base = make_inner(phones, Some(tx));
        base.active_device_id = Some("phone-B".to_string());
        let inner = Arc::new(Mutex::new(base));

        let inner2 = inner.clone();
        let relay_task = tokio::spawn(async move {
            let text = rx.recv().await.unwrap();
            let v: Value = serde_json::from_str(&text).unwrap();
            // The dial must be routed to the active device (phone-B), not phone-A.
            assert_eq!(v["device_id"], "phone-B");
            let message_id = v["message_id"].as_str().unwrap().to_string();
            let tx = {
                let mut relay = inner2.lock().await;
                relay.pending_bridge.remove(&message_id)
            };
            if let Some(tx) = tx {
                let _ = tx.send(BridgeResult {
                    success: true,
                    error: String::new(),
                });
            }
        });

        let msg = r#"{"type":"dial","phone_number":"13800138000"}"#;
        let result = handle_extension_message(msg, &inner).await;
        let _ = relay_task.await;
        let parsed: Value = serde_json::from_str(&result.unwrap()).unwrap();
        assert_eq!(parsed["success"], true);
    }

    #[tokio::test]
    async fn test_handle_get_status() {
        let inner = Arc::new(Mutex::new(make_inner(vec![online_phone()], None)));

        let msg = r#"{"type":"get_status"}"#;
        let result = handle_extension_message(msg, &inner).await;
        assert!(result.is_some());
        let parsed: Value = serde_json::from_str(&result.unwrap()).unwrap();
        assert_eq!(parsed["type"], "status");
        assert_eq!(parsed["connected"], false);
        assert_eq!(parsed["phones"].as_array().unwrap().len(), 1);
    }

    #[tokio::test]
    async fn test_handle_unknown_type() {
        let inner = Arc::new(Mutex::new(make_inner(Vec::new(), None)));

        let msg = r#"{"type":"unknown"}"#;
        let result = handle_extension_message(msg, &inner).await;
        assert!(result.is_none());
    }
}
