import { useState } from "react";

interface Props {
  selectedPhone: string | null;
  onSend: (number: string, content: string) => void;
}

const SMS_TEMPLATES = [
  "",
  "I will be there soon.",
  "Please call me back.",
  "Meeting cancelled.",
  "On my way.",
];

function SMSPanel({ selectedPhone, onSend }: Props) {
  const [number, setNumber] = useState("");
  const [content, setContent] = useState("");

  const handleSend = () => {
    if (number && content && selectedPhone) {
      onSend(number, content);
      setContent("");
    }
  };

  const handleTemplateSelect = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const value = e.target.value;
    if (value) {
      setContent(value);
    }
  };

  return (
    <div className="sms-panel">
      <div className="sms-field">
        <label>Recipient</label>
        <input
          type="text"
          value={number}
          onChange={(e) => setNumber(e.target.value)}
          placeholder="Phone number"
          maxLength={11}
        />
      </div>

      <div className="sms-field">
        <label>Template</label>
        <select onChange={handleTemplateSelect} value="">
          <option value="">-- Select template --</option>
          {SMS_TEMPLATES.filter(Boolean).map((t, i) => (
            <option key={i} value={t}>
              {t}
            </option>
          ))}
        </select>
      </div>

      <div className="sms-field">
        <label>Message</label>
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder="Enter message"
          rows={4}
        />
      </div>

      <button
        className="send-btn"
        onClick={handleSend}
        disabled={!number || !content || !selectedPhone}
      >
        Send SMS
      </button>

      {!selectedPhone && (
        <div className="dial-warning">Select a phone from the list first</div>
      )}
    </div>
  );
}

export default SMSPanel;
