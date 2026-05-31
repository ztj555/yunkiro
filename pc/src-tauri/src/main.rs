// Prevents additional console window on Windows in release
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod commands;
mod relay;

fn main() {
    tauri::Builder::default()
        .manage(relay::RelayState::new())
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
