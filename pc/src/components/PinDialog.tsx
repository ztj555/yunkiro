import { useState } from "react";

interface Props {
  onSubmit: (pin: string) => void;
}

function PinDialog({ onSubmit }: Props) {
  const [pin, setPin] = useState("");
  const [error, setError] = useState("");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!/^\d{4}$/.test(pin)) {
      setError("PIN must be exactly 4 digits");
      return;
    }
    setError("");
    onSubmit(pin);
  };

  return (
    <div className="pin-dialog-overlay">
      <div className="pin-dialog">
        <h2>Enter PIN</h2>
        <p>Enter the 4-digit PIN to connect to the relay server.</p>
        <form onSubmit={handleSubmit}>
          <input
            type="text"
            value={pin}
            onChange={(e) => setPin(e.target.value.replace(/\D/g, ""))}
            maxLength={4}
            placeholder="0000"
            autoFocus
          />
          {error && <div className="pin-error">{error}</div>}
          <button type="submit" disabled={pin.length !== 4}>
            Connect
          </button>
        </form>
      </div>
    </div>
  );
}

export default PinDialog;
