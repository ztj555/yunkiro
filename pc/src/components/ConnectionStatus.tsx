import type { ConnectionState } from "../types/protocol";

interface Props {
  state: ConnectionState;
  phoneCount: number;
}

function ConnectionStatus({ state, phoneCount }: Props) {
  const stateLabels: Record<ConnectionState, string> = {
    disconnected: "Disconnected",
    connecting: "Connecting...",
    connected: "Connected",
    reconnecting: "Reconnecting...",
  };

  const dotClass = state === "connected" ? "dot green" : "dot red";

  return (
    <div className="connection-status">
      <span className={dotClass}></span>
      <span className="status-text">{stateLabels[state]}</span>
      <span className="phone-count">{phoneCount} phone(s) online</span>
    </div>
  );
}

export default ConnectionStatus;
