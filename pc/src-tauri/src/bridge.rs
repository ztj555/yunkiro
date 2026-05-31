use futures_util::{SinkExt, StreamExt};
use serde_json::Value;
use std::sync::Arc;
use tokio::net::TcpListener;
use tokio::sync::Mutex;
use tokio_tungstenite::accept_async;
use tokio_tungstenite::tungstenite::Message;

use crate::relay::RelayInner;

const BRIDGE_PORT: u16 = 8765;

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
            let phone_number = msg.get("phone_number")?.as_str()?;
            let relay = inner.lock().await;

            // Find the first online phone to dial
            let target_phone = relay.phones.iter().find(|p| p.online);
            let device_id = match target_phone {
                Some(phone) => phone.device_id.clone(),
                None => {
                    let err = serde_json::json!({
                        "type": "dial_result",
                        "success": false,
                        "error": "No phone connected"
                    });
                    return Some(err.to_string());
                }
            };

            // Build relay protocol message and send to relay
            let message_id = uuid::Uuid::new_v4().to_string();
            let relay_msg = serde_json::json!({
                "type": "dial",
                "message_id": message_id,
                "phone_number": phone_number,
                "device_id": device_id,
                "sim_slot": 0
            });

            if let Some(ref sender) = relay.sender {
                if sender.send(relay_msg.to_string()).is_err() {
                    let err = serde_json::json!({
                        "type": "dial_result",
                        "success": false,
                        "error": "Failed to send to relay"
                    });
                    return Some(err.to_string());
                }
                // Note: The actual dial_result will come asynchronously from the relay.
                // For now we acknowledge receipt. A full implementation would correlate
                // by message_id and forward the relay's dial_result back.
                let ack = serde_json::json!({
                    "type": "dial_result",
                    "success": true,
                    "error": ""
                });
                return Some(ack.to_string());
            } else {
                let err = serde_json::json!({
                    "type": "dial_result",
                    "success": false,
                    "error": "Not connected to relay"
                });
                return Some(err.to_string());
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
    use crate::relay::{ConnectionState, PhoneInfo, RelayInner};
    use tokio::sync::mpsc;

    #[tokio::test]
    async fn test_handle_dial_no_phone() {
        let inner = Arc::new(Mutex::new(RelayInner {
            state: ConnectionState::Connected,
            phones: Vec::new(),
            sender: None,
            should_reconnect: false,
        }));

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
        let (tx, _rx) = mpsc::unbounded_channel::<String>();
        let inner = Arc::new(Mutex::new(RelayInner {
            state: ConnectionState::Connected,
            phones: vec![PhoneInfo {
                device_id: "phone-001".to_string(),
                device_name: "Test Phone".to_string(),
                online: true,
                status: "idle".to_string(),
            }],
            sender: Some(tx),
            should_reconnect: false,
        }));

        let msg = r#"{"type":"dial","phone_number":"13800138000"}"#;
        let result = handle_extension_message(msg, &inner).await;
        assert!(result.is_some());
        let parsed: Value = serde_json::from_str(&result.unwrap()).unwrap();
        assert_eq!(parsed["type"], "dial_result");
        assert_eq!(parsed["success"], true);
    }

    #[tokio::test]
    async fn test_handle_get_status() {
        let inner = Arc::new(Mutex::new(RelayInner {
            state: ConnectionState::Connected,
            phones: vec![PhoneInfo {
                device_id: "phone-001".to_string(),
                device_name: "Test Phone".to_string(),
                online: true,
                status: "idle".to_string(),
            }],
            sender: None,
            should_reconnect: false,
        }));

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
        let inner = Arc::new(Mutex::new(RelayInner {
            state: ConnectionState::Disconnected,
            phones: Vec::new(),
            sender: None,
            should_reconnect: false,
        }));

        let msg = r#"{"type":"unknown"}"#;
        let result = handle_extension_message(msg, &inner).await;
        assert!(result.is_none());
    }
}
