import type { PhoneState } from "../types/protocol";

interface Props {
  phones: PhoneState[];
  selectedId: string | null;
  onSelect: (id: string) => void;
}

function PhoneList({ phones, selectedId, onSelect }: Props) {
  if (phones.length === 0) {
    return (
      <div className="phone-list">
        <div className="phone-list-empty">No phones connected</div>
      </div>
    );
  }

  return (
    <div className="phone-list">
      {phones.map((phone) => (
        <div
          key={phone.device_id}
          className={`phone-item ${selectedId === phone.device_id ? "selected" : ""} ${phone.online ? "online" : "offline"}`}
          onClick={() => onSelect(phone.device_id)}
        >
          <span
            className={`phone-dot ${phone.online ? "green" : "gray"}`}
          ></span>
          <span className="phone-name">{phone.device_name}</span>
          <span className="phone-status">{phone.status}</span>
        </div>
      ))}
    </div>
  );
}

export default PhoneList;
