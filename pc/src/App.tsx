import { useState, useEffect } from "react";
import ConnectionStatus from "./components/ConnectionStatus";
import PhoneList from "./components/PhoneList";
import DialPad from "./components/DialPad";
import SMSPanel from "./components/SMSPanel";
import PinDialog from "./components/PinDialog";
import Settings from "./components/Settings";
import { useRelay } from "./hooks/useRelay";
import { useClipboard } from "./hooks/useClipboard";

type Tab = "dial" | "sms" | "settings";

function App() {
  const [pin, setPin] = useState<string | null>(
    localStorage.getItem("relay_pin")
  );
  const [tab, setTab] = useState<Tab>("dial");
  const [selectedPhone, setSelectedPhone] = useState<string | null>(null);
  const [theme, setTheme] = useState<"light" | "dark">(
    (localStorage.getItem("theme") as "light" | "dark") || "light"
  );

  const relay = useRelay();
  const { detectedNumber, clearDetected } = useClipboard();

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem("theme", theme);
  }, [theme]);

  useEffect(() => {
    if (pin) {
      const url = localStorage.getItem("relay_url") || "ws://127.0.0.1:8080/ws";
      relay.connect(url, pin);
    }
  }, [pin]);

  const handlePinSubmit = (newPin: string) => {
    localStorage.setItem("relay_pin", newPin);
    setPin(newPin);
  };

  const toggleTheme = () => {
    setTheme((t) => (t === "light" ? "dark" : "light"));
  };

  if (!pin) {
    return <PinDialog onSubmit={handlePinSubmit} />;
  }

  return (
    <div className="app">
      <ConnectionStatus
        state={relay.connectionState}
        phoneCount={relay.phones.filter((p) => p.online).length}
      />

      <div className="main-content">
        <PhoneList
          phones={relay.phones}
          selectedId={selectedPhone}
          onSelect={(id) => {
            setSelectedPhone(id);
            relay.setActiveDevice(id);
          }}
        />

        <div className="tabs">
          <button
            className={tab === "dial" ? "tab active" : "tab"}
            onClick={() => setTab("dial")}
          >
            Dial
          </button>
          <button
            className={tab === "sms" ? "tab active" : "tab"}
            onClick={() => setTab("sms")}
          >
            SMS
          </button>
          <button
            className={tab === "settings" ? "tab active" : "tab"}
            onClick={() => setTab("settings")}
          >
            Settings
          </button>
        </div>

        <div className="tab-content">
          {tab === "dial" && (
            <DialPad
              selectedPhone={selectedPhone}
              onDial={(number, simSlot) =>
                selectedPhone && relay.dial(number, selectedPhone, simSlot)
              }
              onHangup={() => selectedPhone && relay.hangup(selectedPhone)}
              detectedNumber={detectedNumber}
              onClearDetected={clearDetected}
            />
          )}
          {tab === "sms" && (
            <SMSPanel
              selectedPhone={selectedPhone}
              onSend={(number, content) =>
                selectedPhone && relay.sendSMS(number, content, selectedPhone)
              }
            />
          )}
          {tab === "settings" && (
            <Settings
              theme={theme}
              onToggleTheme={toggleTheme}
              onDisconnect={relay.disconnect}
              onReconnect={() => {
                const url =
                  localStorage.getItem("relay_url") || "ws://127.0.0.1:8080/ws";
                relay.connect(url, pin);
              }}
            />
          )}
        </div>
      </div>
    </div>
  );
}

export default App;
