import { useState } from "react";

interface Props {
  theme: "light" | "dark";
  onToggleTheme: () => void;
  onDisconnect: () => void;
  onReconnect: () => void;
}

function Settings({ theme, onToggleTheme, onDisconnect, onReconnect }: Props) {
  const [url, setUrl] = useState(
    localStorage.getItem("relay_url") || "ws://127.0.0.1:8080/ws"
  );

  const handleSaveUrl = () => {
    localStorage.setItem("relay_url", url);
  };

  const handleClearPin = () => {
    localStorage.removeItem("relay_pin");
    window.location.reload();
  };

  return (
    <div className="settings">
      <div className="settings-section">
        <h3>Relay Server</h3>
        <div className="settings-field">
          <label>URL</label>
          <input
            type="text"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
          />
          <button onClick={handleSaveUrl}>Save</button>
        </div>
        <div className="settings-actions">
          <button onClick={onDisconnect}>Disconnect</button>
          <button onClick={onReconnect}>Reconnect</button>
        </div>
      </div>

      <div className="settings-section">
        <h3>Appearance</h3>
        <div className="settings-field">
          <label>Theme</label>
          <button onClick={onToggleTheme}>
            {theme === "light" ? "Switch to Dark" : "Switch to Light"}
          </button>
        </div>
      </div>

      <div className="settings-section">
        <h3>Authentication</h3>
        <button className="danger-btn" onClick={handleClearPin}>
          Clear PIN &amp; Logout
        </button>
      </div>
    </div>
  );
}

export default Settings;
