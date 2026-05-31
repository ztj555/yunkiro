// Prevents additional console window on Windows in release
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod bridge;
mod commands;
mod relay;

use tauri::Manager;

fn main() {
    tauri::Builder::default()
        .manage(relay::RelayState::new())
        .setup(|app| {
            let state: tauri::State<relay::RelayState> = app.state();
            let inner = state.inner.clone();
            // Start the local bridge server for the browser extension
            tokio::spawn(bridge::start_bridge(inner));
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            commands::connect,
            commands::disconnect,
            commands::send_message,
            commands::dial,
            commands::hangup,
            commands::send_sms,
            commands::get_status,
            commands::get_phones,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
