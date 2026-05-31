import { useState, useEffect } from "react";

interface Props {
  selectedPhone: string | null;
  onDial: (number: string, simSlot: number) => void;
  onHangup: () => void;
  detectedNumber: string | null;
  onClearDetected: () => void;
}

function isValidPhoneNumber(number: string): boolean {
  // Chinese mobile number: 11 digits starting with 1
  return /^1\d{10}$/.test(number);
}

function DialPad({
  selectedPhone,
  onDial,
  onHangup,
  detectedNumber,
  onClearDetected,
}: Props) {
  const [number, setNumber] = useState("");
  const [simSlot, setSimSlot] = useState(0);

  useEffect(() => {
    if (detectedNumber) {
      setNumber(detectedNumber);
      onClearDetected();
    }
  }, [detectedNumber, onClearDetected]);

  const handleKeyPress = (key: string) => {
    setNumber((prev) => prev + key);
  };

  const handleBackspace = () => {
    setNumber((prev) => prev.slice(0, -1));
  };

  const handleDial = () => {
    if (isValidPhoneNumber(number) && selectedPhone) {
      onDial(number, simSlot);
    }
  };

  const keys = ["1", "2", "3", "4", "5", "6", "7", "8", "9", "*", "0", "#"];

  return (
    <div className="dial-pad">
      <div className="number-input">
        <input
          type="text"
          value={number}
          onChange={(e) => setNumber(e.target.value)}
          placeholder="Enter phone number"
          maxLength={11}
        />
        <button className="backspace-btn" onClick={handleBackspace}>
          &#x232B;
        </button>
      </div>

      <div className="keypad">
        {keys.map((key) => (
          <button
            key={key}
            className="key-btn"
            onClick={() => handleKeyPress(key)}
          >
            {key}
          </button>
        ))}
      </div>

      <div className="sim-selector">
        <label>
          <input
            type="radio"
            name="sim"
            checked={simSlot === 0}
            onChange={() => setSimSlot(0)}
          />
          SIM 1
        </label>
        <label>
          <input
            type="radio"
            name="sim"
            checked={simSlot === 1}
            onChange={() => setSimSlot(1)}
          />
          SIM 2
        </label>
      </div>

      <div className="dial-actions">
        <button
          className="dial-btn"
          onClick={handleDial}
          disabled={!isValidPhoneNumber(number) || !selectedPhone}
        >
          Dial
        </button>
        <button
          className="hangup-btn"
          onClick={onHangup}
          disabled={!selectedPhone}
        >
          Hangup
        </button>
      </div>

      {!selectedPhone && (
        <div className="dial-warning">Select a phone from the list first</div>
      )}
    </div>
  );
}

export default DialPad;
