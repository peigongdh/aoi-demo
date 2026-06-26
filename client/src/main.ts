import { DebugPanel } from "./debug-panel";
import { NetworkClient, type InputState, type ServerMessage } from "./network";
import { Renderer } from "./renderer";
import { WorldModel } from "./world";
import "./style.css";

const world = new WorldModel();
const panel = new DebugPanel(document);
const canvas = document.getElementById("world-canvas");
if (!(canvas instanceof HTMLCanvasElement)) {
  throw new Error("Missing canvas");
}
const renderer = new Renderer(canvas);
const serverInput = input("server-url");
const connectButton = button("connect");
const disconnectButton = button("disconnect");
const aoiSelect = select("aoi-type");
const applyAOIButton = button("apply-aoi");
const keys = new Set<string>();
let lastPanelRender = 0;

serverInput.value = defaultWebSocketURL();
void refreshAOIOptions();

const network = new NetworkClient(handleMessage, (status) => panel.setStatus(status));

connectButton.addEventListener("click", () => {
  const name = `player_${Math.floor(Math.random() * 9000 + 1000)}`;
  network.connect(serverInput.value.trim(), name);
});

disconnectButton.addEventListener("click", () => network.disconnect());
applyAOIButton.addEventListener("click", () => {
  void applyAOISelection();
});
serverInput.addEventListener("change", () => {
  void refreshAOIOptions();
});

window.addEventListener("resize", () => renderer.resize());
window.addEventListener("keydown", (event) => {
  if (isMovementKey(event.key)) {
    keys.add(event.key.toLowerCase());
    event.preventDefault();
  }
});
window.addEventListener("keyup", (event) => {
  if (isMovementKey(event.key)) {
    keys.delete(event.key.toLowerCase());
    event.preventDefault();
  }
});

window.setInterval(() => network.sendInput(currentInput()), 50);

function frame(now: number): void {
  world.latencyMs = network.latencyMs;
  renderer.render(world, panel.controls, now);
  if (now - lastPanelRender > 100) {
    panel.render(world);
    lastPanelRender = now;
  }
  requestAnimationFrame(frame);
}
requestAnimationFrame(frame);

function handleMessage(message: ServerMessage): void {
  switch (message.type) {
    case "welcome":
      world.applyWelcome(message.payload);
      ensureAOIOption(message.payload.aoi_type);
      aoiSelect.value = message.payload.aoi_type;
      break;
    case "entity_enter":
      world.applyEnter(message.payload.entity);
      break;
    case "entity_leave":
      world.applyLeave(message.payload.entity_id);
      break;
    case "entity_update":
      world.applyUpdate(message.payload);
      break;
    case "pong":
      break;
  }
}

function currentInput(): InputState {
  return {
    up: keys.has("w") || keys.has("arrowup"),
    down: keys.has("s") || keys.has("arrowdown"),
    left: keys.has("a") || keys.has("arrowleft"),
    right: keys.has("d") || keys.has("arrowright"),
  };
}

function defaultWebSocketURL(): string {
  const host = window.location.hostname || "localhost";
  return `ws://${host}:8100/ws`;
}

interface AOIConfigResponse {
  type: string;
  algorithms: string[];
}

async function refreshAOIOptions(): Promise<void> {
  try {
    const config = await fetchAOIConfig();
    setAOIOptions(config.algorithms, config.type);
    world.aoiType = config.type;
  } catch (error) {
    console.warn("Failed to load AOI config", error);
  }
}

async function applyAOISelection(): Promise<void> {
  const originalText = applyAOIButton.textContent || "Apply";
  applyAOIButton.disabled = true;
  applyAOIButton.textContent = "Applying";
  try {
    const config = await updateAOIConfig(aoiSelect.value);
    setAOIOptions(config.algorithms, config.type);
    world.aoiType = config.type;
  } catch (error) {
    console.warn("Failed to switch AOI config", error);
  } finally {
    applyAOIButton.disabled = false;
    applyAOIButton.textContent = originalText;
  }
}

async function fetchAOIConfig(): Promise<AOIConfigResponse> {
  const response = await fetch(`${apiBaseURL()}/api/aoi`);
  return readAOIConfig(response);
}

async function updateAOIConfig(type: string): Promise<AOIConfigResponse> {
  const response = await fetch(`${apiBaseURL()}/api/aoi`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ type }),
  });
  return readAOIConfig(response);
}

async function readAOIConfig(response: Response): Promise<AOIConfigResponse> {
  if (!response.ok) {
    throw new Error(`AOI config request failed: ${response.status}`);
  }
  const config = (await response.json()) as AOIConfigResponse;
  if (!config.type || !Array.isArray(config.algorithms)) {
    throw new Error("AOI config response is invalid");
  }
  return config;
}

function apiBaseURL(): string {
  const raw = serverInput.value.trim() || defaultWebSocketURL();
  const withProtocol = raw.includes("://") ? raw : `ws://${raw}`;
  const url = new URL(withProtocol);
  url.protocol = url.protocol === "wss:" ? "https:" : "http:";
  url.pathname = "";
  url.search = "";
  url.hash = "";
  return url.toString().replace(/\/$/, "");
}

function setAOIOptions(algorithms: string[], selected: string): void {
  const values = [...new Set([...algorithms, selected].filter(Boolean))];
  aoiSelect.replaceChildren(...values.map((type) => new Option(type, type)));
  aoiSelect.value = selected;
}

function ensureAOIOption(type: string): void {
  if (![...aoiSelect.options].some((option) => option.value === type)) {
    aoiSelect.append(new Option(type, type));
  }
}

function isMovementKey(key: string): boolean {
  return ["w", "a", "s", "d", "arrowup", "arrowleft", "arrowdown", "arrowright"].includes(key.toLowerCase());
}

function input(id: string): HTMLInputElement {
  const el = document.getElementById(id);
  if (!(el instanceof HTMLInputElement)) {
    throw new Error(`Missing #${id}`);
  }
  return el;
}

function button(id: string): HTMLButtonElement {
  const el = document.getElementById(id);
  if (!(el instanceof HTMLButtonElement)) {
    throw new Error(`Missing #${id}`);
  }
  return el;
}

function select(id: string): HTMLSelectElement {
  const el = document.getElementById(id);
  if (!(el instanceof HTMLSelectElement)) {
    throw new Error(`Missing #${id}`);
  }
  return el;
}
