import type { EntityState, EntityUpdatePayload, WelcomePayload } from "./world";

type MessageHandler = (message: ServerMessage) => void;
type StatusHandler = (status: ConnectionStatus) => void;

export type ConnectionStatus = "connected" | "connecting" | "disconnected";

export type ServerMessage =
  | { type: "welcome"; payload: WelcomePayload }
  | { type: "entity_enter"; payload: { entity: EntityState } }
  | { type: "entity_leave"; payload: { entity_id: string } }
  | { type: "entity_update"; payload: EntityUpdatePayload }
  | { type: "pong"; payload: { client_time: number; server_time: number } };

export interface InputState {
  up: boolean;
  down: boolean;
  left: boolean;
  right: boolean;
}

export class NetworkClient {
  latencyMs = 0;
  private socket?: WebSocket;
  private seq = 1;
  private pingTimer = 0;
  private onMessage: MessageHandler;
  private onStatus: StatusHandler;

  constructor(onMessage: MessageHandler, onStatus: StatusHandler) {
    this.onMessage = onMessage;
    this.onStatus = onStatus;
  }

  connect(url: string, playerName: string): void {
    this.disconnect();
    this.onStatus("connecting");
    this.socket = new WebSocket(url);

    this.socket.addEventListener("open", () => {
      this.onStatus("connected");
      this.send("join_world", { name: playerName });
      this.pingTimer = window.setInterval(() => this.ping(), 2000);
      this.ping();
    });

    this.socket.addEventListener("message", (event) => {
      const message = JSON.parse(String(event.data)) as ServerMessage;
      if (message.type === "pong") {
        this.latencyMs = Math.max(0, performance.now() - message.payload.client_time);
      }
      this.onMessage(message);
    });

    this.socket.addEventListener("close", () => {
      this.clearPing();
      this.onStatus("disconnected");
    });

    this.socket.addEventListener("error", () => {
      this.clearPing();
      this.onStatus("disconnected");
    });
  }

  disconnect(): void {
    this.clearPing();
    if (this.socket && this.socket.readyState !== WebSocket.CLOSED) {
      this.socket.close();
    }
    this.socket = undefined;
    this.onStatus("disconnected");
  }

  sendInput(input: InputState): void {
    this.send("input", input);
  }

  private ping(): void {
    this.send("ping", { client_time: performance.now() });
  }

  private send(type: string, payload: unknown): void {
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return;
    }
    this.socket.send(JSON.stringify({ type, seq: this.seq++, payload }));
  }

  private clearPing(): void {
    if (this.pingTimer) {
      window.clearInterval(this.pingTimer);
      this.pingTimer = 0;
    }
  }
}
