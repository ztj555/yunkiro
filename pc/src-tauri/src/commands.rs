use crate::relay::{self, ConnectionState, PhoneInfo, RelayState};
use serde::Serialize;
use tauri::{AppHandle, State};

#[derive(Serialize)]
pub struct StatusResponse {
    pub state: ConnectionState,
    pub phone_count: usize,
}

#[tauri::command]
pub async fn connect(
    app: AppHandle,
    state: State<'_, RelayState>,
    url: String,
    pin: String,
) -> Result<(), String> {
    let inner = state.inner.clone();

    // Disconnect existing connection
    {
        let mut relay = inner.lock().await;
        relay.should_reconnect = false;
        if let Some(sender) = relay.sender.take() {
            drop(sender);
        }
    }

    // Small delay to let the old connection close
    tokio::time::sleep(std::time::Duration::from_millis(100)).await;

    // Start new connection in background
    let connect_inner = inner.clone();
    tokio::spawn(async move {
        relay::connect_relay(app, connect_inner, url, pin).await;
    });

    Ok(())
}

#[tauri::command]
pub async fn disconnect(state: State<'_, RelayState>) -> Result<(), String> {
    let mut relay = state.inner.lock().await;
    relay.should_reconnect = false;
    if let Some(sender) = relay.sender.take() {
        drop(sender);
    }
    relay.state = ConnectionState::Disconnected;
    relay.phones.clear();
    relay.active_device_id = None;
    // Drop any waiting bridge requests; their receivers will resolve as
    // "Dial timed out" instead of hanging.
    relay.pending_bridge.clear();
    Ok(())
}

/// Sets the phone the browser-extension bridge should dial. Called by the
/// frontend whenever the user selects a phone in the list.
#[tauri::command]
pub async fn set_active_device(
    state: State<'_, RelayState>,
    device_id: Option<String>,
) -> Result<(), String> {
    let mut relay = state.inner.lock().await;
    relay.active_device_id = device_id;
    Ok(())
}

#[tauri::command]
pub async fn send_message(
    state: State<'_, RelayState>,
    message: String,
) -> Result<(), String> {
    let relay = state.inner.lock().await;
    if let Some(ref sender) = relay.sender {
        sender
            .send(message)
            .map_err(|e| format!("Failed to send: {}", e))?;
    } else {
        return Err("Not connected".to_string());
    }
    Ok(())
}

#[tauri::command]
pub async fn dial(
    state: State<'_, RelayState>,
    phone_number: String,
    device_id: String,
    sim_slot: u32,
) -> Result<(), String> {
    let msg = serde_json::json!({
        "type": "dial",
        "message_id": uuid::Uuid::new_v4().to_string(),
        "phone_number": phone_number,
        "device_id": device_id,
        "sim_slot": sim_slot
    });

    let relay = state.inner.lock().await;
    if let Some(ref sender) = relay.sender {
        sender
            .send(msg.to_string())
            .map_err(|e| format!("Failed to send: {}", e))?;
    } else {
        return Err("Not connected".to_string());
    }
    Ok(())
}

#[tauri::command]
pub async fn hangup(
    state: State<'_, RelayState>,
    device_id: String,
) -> Result<(), String> {
    let msg = serde_json::json!({
        "type": "hangup",
        "message_id": uuid::Uuid::new_v4().to_string(),
        "device_id": device_id
    });

    let relay = state.inner.lock().await;
    if let Some(ref sender) = relay.sender {
        sender
            .send(msg.to_string())
            .map_err(|e| format!("Failed to send: {}", e))?;
    } else {
        return Err("Not connected".to_string());
    }
    Ok(())
}

#[tauri::command]
pub async fn send_sms(
    state: State<'_, RelayState>,
    phone_number: String,
    content: String,
    device_id: String,
) -> Result<(), String> {
    let msg = serde_json::json!({
        "type": "sms",
        "message_id": uuid::Uuid::new_v4().to_string(),
        "phone_number": phone_number,
        "content": content,
        "device_id": device_id
    });

    let relay = state.inner.lock().await;
    if let Some(ref sender) = relay.sender {
        sender
            .send(msg.to_string())
            .map_err(|e| format!("Failed to send: {}", e))?;
    } else {
        return Err("Not connected".to_string());
    }
    Ok(())
}

#[tauri::command]
pub async fn get_status(state: State<'_, RelayState>) -> Result<StatusResponse, String> {
    let relay = state.inner.lock().await;
    Ok(StatusResponse {
        state: relay.state.clone(),
        phone_count: relay.phones.iter().filter(|p| p.online).count(),
    })
}

#[tauri::command]
pub async fn get_phones(state: State<'_, RelayState>) -> Result<Vec<PhoneInfo>, String> {
    let relay = state.inner.lock().await;
    Ok(relay.phones.clone())
}
