use futures_util::{SinkExt, StreamExt};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;
use tauri::{AppHandle, Emitter};
use tokio::sync::{mpsc, oneshot, Mutex};
use tokio::time;
use tokio_tungstenite::connect_async;
use tokio_tungstenite::tungstenite::Message;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum ConnectionState {
    #[serde(rename = "disconnected")]
    Disconnected,
    #[serde(rename = "connecting")]
    Connecting,
    #[serde(rename = "connected")]
    Connected,
    #[serde(rename = "reconnecting")]
    Reconnecting,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PhoneInfo {
    pub device_id: String,
    pub device_name: String,
    pub online: bool,
    pub status: String,
}

pub struct RelayInner {
    pub state: ConnectionState,
    pub phones: Vec<PhoneInfo>,
    pub sender: Option<mpsc::UnboundedSender<String>>,
    pub should_reconnect: bool,
    /// Stable per-process identity. Reusing the same device_id across reconnects
    /// means the relay replaces our old slot instead of leaving a ghost
    /// connection alive until its heartbeat times out.
    pub device_id: String,
    /// The phone the user has selected in the UI. The browser-extension bridge
    /// dials this device, falling back to the first online phone when unset.
    pub active_device_id: Option<String>,
    /// Dial/SMS requests originated by the browser extension, keyed by
    /// message_id, awaiting the real result coming back from the phone.
    pub pending_bridge: HashMap<String, oneshot::Sender<BridgeResult>>,
}

/// Result of a bridged dial/sms request, delivered back to the extension.
#[derive(Debug, Clone)]
pub struct BridgeResult {
    pub success: bool,
    pub error: String,
}

pub struct RelayState {
    pub inner: Arc<Mutex<RelayInner>>,
}

impl RelayState {
    pub fn new() -> Self {
        RelayState {
            inner: Arc::new(Mutex::new(RelayInner {
                state: ConnectionState::Disconnected,
                phones: Vec::new(),
                sender: None,
                should_reconnect: false,
                device_id: uuid::Uuid::new_v4().to_string(),
                active_device_id: None,
                pending_bridge: HashMap::new(),
            })),
        }
    }
}

pub async fn connect_relay(
    app: AppHandle,
    inner: Arc<Mutex<RelayInner>>,
    url: String,
    pin: String,
) {
    // Set connecting state
    {
        let mut relay = inner.lock().await;
        relay.state = ConnectionState::Connecting;
        relay.should_reconnect = true;
        let _ = app.emit("relay-status-changed", "connecting");
    }

    let device_id = {
        let relay = inner.lock().await;
        relay.device_id.clone()
    };
    let mut attempt: u32 = 0;

    loop {
        let connect_result = connect_async(&url).await;

        match connect_result {
            Ok((ws_stream, _)) => {
                attempt = 0;
                let (mut write, mut read) = ws_stream.split();

                // Create message channel
                let (tx, mut rx) = mpsc::unbounded_channel::<String>();

                {
                    let mut relay = inner.lock().await;
                    relay.sender = Some(tx.clone());
                }

                // Send auth message
                let auth_msg = serde_json::json!({
                    "type": "auth",
                    "pin": pin,
                    "role": "pc",
                    "device_id": device_id,
                    "device_name": "YunKiro PC"
                });
                let _ = write.send(Message::Text(auth_msg.to_string())).await;

                // Spawn write task
                let write_inner = inner.clone();
                let write_handle = tokio::spawn(async move {
                    while let Some(msg) = rx.recv().await {
                        if write.send(Message::Text(msg)).await.is_err() {
                            break;
                        }
                    }
                    let mut relay = write_inner.lock().await;
                    relay.sender = None;
                });

                // Spawn ping task
                let ping_tx = tx.clone();
                let ping_handle = tokio::spawn(async move {
                    let mut interval = time::interval(Duration::from_secs(30));
                    loop {
                        interval.tick().await;
                        let ping = serde_json::json!({"type": "ping"}).to_string();
                        if ping_tx.send(ping).is_err() {
                            break;
                        }
                    }
                });

                // Read loop
                loop {
                    match read.next().await {
                        Some(Ok(Message::Text(text))) => {
                            handle_message(&app, &inner, &text).await;
                        }
                        Some(Ok(Message::Close(_))) | None => {
                            break;
                        }
                        Some(Err(_)) => {
                            break;
                        }
                        _ => {}
                    }
                }

                // Cleanup
                ping_handle.abort();
                write_handle.abort();
                {
                    let mut relay = inner.lock().await;
                    relay.sender = None;
                }
            }
            Err(_) => {
                // Connection failed
            }
        }

        // Check if we should reconnect
        {
            let relay = inner.lock().await;
            if !relay.should_reconnect {
                break;
            }
        }

        // Reconnect with exponential backoff
        {
            let mut relay = inner.lock().await;
            relay.state = ConnectionState::Reconnecting;
            let _ = app.emit("relay-status-changed", "reconnecting");
        }

        let delay = Duration::from_millis(
            (1000 * 2u64.saturating_pow(attempt)).min(30000),
        );
        attempt += 1;
        time::sleep(delay).await;

        // Check again after sleep
        {
            let relay = inner.lock().await;
            if !relay.should_reconnect {
                break;
            }
        }
    }

    // Final disconnect state
    {
        let mut relay = inner.lock().await;
        relay.state = ConnectionState::Disconnected;
        relay.phones.clear();
        let _ = app.emit("relay-status-changed", "disconnected");
    }
}

async fn handle_message(
    app: &AppHandle,
    inner: &Arc<Mutex<RelayInner>>,
    text: &str,
) {
    let msg: serde_json::Value = match serde_json::from_str(text) {
        Ok(v) => v,
        Err(_) => return,
    };

    let msg_type = msg.get("type").and_then(|t| t.as_str()).unwrap_or("");

    match msg_type {
        "auth_result" => {
            let success = msg.get("success").and_then(|s| s.as_bool()).unwrap_or(false);
            if success {
                let mut relay = inner.lock().await;
                relay.state = ConnectionState::Connected;

                if let Some(phones) = msg.get("online_phones").and_then(|p| p.as_array()) {
                    relay.phones = phones
                        .iter()
                        .filter_map(|p| {
                            Some(PhoneInfo {
                                device_id: p.get("device_id")?.as_str()?.to_string(),
                                device_name: p.get("device_name")?.as_str()?.to_string(),
                                online: true,
                                status: "idle".to_string(),
                            })
                        })
                        .collect();
                }
                let _ = app.emit("relay-status-changed", "connected");
                let _ = app.emit("auth-result", text);
            } else {
                let mut relay = inner.lock().await;
                relay.state = ConnectionState::Disconnected;
                relay.should_reconnect = false;
                let _ = app.emit("relay-status-changed", "disconnected");
                let _ = app.emit("auth-result", text);
            }
        }
        "phone_online" => {
            let device_id = msg.get("device_id").and_then(|d| d.as_str()).unwrap_or("").to_string();
            let device_name = msg.get("device_name").and_then(|d| d.as_str()).unwrap_or("").to_string();

            let mut relay = inner.lock().await;
            if let Some(phone) = relay.phones.iter_mut().find(|p| p.device_id == device_id) {
                phone.online = true;
            } else {
                relay.phones.push(PhoneInfo {
                    device_id: device_id.clone(),
                    device_name: device_name.clone(),
                    online: true,
                    status: "idle".to_string(),
                });
            }
            let payload = serde_json::json!({
                "device_id": device_id,
                "device_name": device_name
            });
            let _ = app.emit("phone-online", payload.to_string());
        }
        "phone_offline" => {
            let device_id = msg.get("device_id").and_then(|d| d.as_str()).unwrap_or("").to_string();
            let device_name = msg.get("device_name").and_then(|d| d.as_str()).unwrap_or("").to_string();

            let mut relay = inner.lock().await;
            if let Some(phone) = relay.phones.iter_mut().find(|p| p.device_id == device_id) {
                phone.online = false;
            }
            let payload = serde_json::json!({
                "device_id": device_id,
                "device_name": device_name
            });
            let _ = app.emit("phone-offline", payload.to_string());
        }
        "device_status" => {
            let device_id = msg.get("device_id").and_then(|d| d.as_str()).unwrap_or("").to_string();
            let status = msg.get("status").and_then(|s| s.as_str()).unwrap_or("idle").to_string();

            let mut relay = inner.lock().await;
            if let Some(phone) = relay.phones.iter_mut().find(|p| p.device_id == device_id) {
                phone.status = status.clone();
            }
            let payload = serde_json::json!({
                "device_id": device_id,
                "status": status
            });
            let _ = app.emit("device-status", payload.to_string());
        }
        "dial_result" => {
            fulfill_bridge(inner, &msg).await;
            let _ = app.emit("dial-result", text);
        }
        "sms_result" => {
            fulfill_bridge(inner, &msg).await;
            let _ = app.emit("sms-result", text);
        }
        "pong" => {
            // Heartbeat response, no action needed
        }
        _ => {}
    }
}

/// If a browser-extension request is waiting on this message_id, deliver the
/// real result to it so the extension reports the true outcome.
async fn fulfill_bridge(inner: &Arc<Mutex<RelayInner>>, msg: &serde_json::Value) {
    let message_id = match msg.get("message_id").and_then(|m| m.as_str()) {
        Some(id) => id.to_string(),
        None => return,
    };
    let tx = {
        let mut relay = inner.lock().await;
        relay.pending_bridge.remove(&message_id)
    };
    if let Some(tx) = tx {
        let success = msg.get("success").and_then(|s| s.as_bool()).unwrap_or(false);
        let error = msg
            .get("error")
            .and_then(|e| e.as_str())
            .unwrap_or("")
            .to_string();
        let _ = tx.send(BridgeResult { success, error });
    }
}
