import type { ConnectionStatus } from "./network";
import type { WorldModel } from "./world";

export interface ControlState {
  debugMode: boolean;
  showGrid: boolean;
  showAOI: boolean;
  showAll: boolean;
}

export class DebugPanel {
  controls: ControlState;
  private statusDot: HTMLElement;
  private connectionStatus: HTMLElement;
  private worldStats: HTMLElement;
  private playerStats: HTMLElement;
  private eventLog: HTMLElement;
  private debugInput: HTMLInputElement;
  private gridInput: HTMLInputElement;
  private aoiInput: HTMLInputElement;
  private allInput: HTMLInputElement;
  private lastWorldStats = "";
  private lastPlayerStats = "";
  private lastEventLog = "";

  constructor(root: Document) {
    this.statusDot = must(root, "status-dot");
    this.connectionStatus = must(root, "connection-status");
    this.worldStats = must(root, "world-stats");
    this.playerStats = must(root, "player-stats");
    this.eventLog = must(root, "event-log");
    this.debugInput = mustInput(root, "debug-mode");
    this.gridInput = mustInput(root, "show-grid");
    this.aoiInput = mustInput(root, "show-aoi");
    this.allInput = mustInput(root, "show-all");
    this.controls = this.readControls();

    for (const input of [this.debugInput, this.gridInput, this.aoiInput, this.allInput]) {
      input.addEventListener("change", () => {
        this.controls = this.readControls();
      });
    }
  }

  setStatus(status: ConnectionStatus): void {
    this.connectionStatus.textContent = status[0].toUpperCase() + status.slice(1);
    this.statusDot.dataset.status = status;
  }

  render(world: WorldModel): void {
    const player = world.localPlayer;
    const worldStats = definitionList([
      ["AOI", world.aoiType],
      ["Tick", String(world.serverTick)],
      ["Ping", `${world.latencyMs.toFixed(0)} ms`],
      ["Visible", String(world.visibleCount())],
      ["Received", String(world.totalCount())],
      ["Players", String(world.stats.player_count)],
      ["NPCs", String(world.stats.npc_count)],
      ["AOI ms", world.stats.aoi_query_ms.toFixed(3)],
      ["Tick ms", world.stats.tick_duration_ms.toFixed(3)],
      ["Messages", String(world.stats.messages_out)],
      ["Events", String(world.stats.aoi_events)],
      ["Map", `${world.mapWidth} x ${world.mapHeight}`],
    ]);
    if (worldStats !== this.lastWorldStats) {
      this.worldStats.innerHTML = worldStats;
      this.lastWorldStats = worldStats;
    }

    const playerStats = definitionList([
      ["PlayerId", world.localPlayerId || "-"],
      ["Position", player ? `${player.x.toFixed(1)}, ${player.y.toFixed(1)}` : "-"],
      ["Radius", world.aoiRadius.toFixed(0)],
    ]);
    if (playerStats !== this.lastPlayerStats) {
      this.playerStats.innerHTML = playerStats;
      this.lastPlayerStats = playerStats;
    }

    const eventLog = world.eventLog
      .map((event) => `<li class="${event.type}"><span>${event.tick}</span><b>${event.type}</b><em>${event.entityId}</em></li>`)
      .join("");
    if (eventLog !== this.lastEventLog) {
      this.eventLog.innerHTML = eventLog;
      this.lastEventLog = eventLog;
    }
  }

  private readControls(): ControlState {
    return {
      debugMode: this.debugInput.checked,
      showGrid: this.gridInput.checked,
      showAOI: this.aoiInput.checked,
      showAll: this.allInput.checked,
    };
  }
}

function definitionList(rows: Array<[string, string]>): string {
  return rows.map(([key, value]) => `<dt>${key}</dt><dd>${value}</dd>`).join("");
}

function must(root: Document, id: string): HTMLElement {
  const el = root.getElementById(id);
  if (!el) {
    throw new Error(`Missing #${id}`);
  }
  return el;
}

function mustInput(root: Document, id: string): HTMLInputElement {
  const el = must(root, id);
  if (!(el instanceof HTMLInputElement)) {
    throw new Error(`#${id} is not an input`);
  }
  return el;
}
