export type EntityType = "player" | "npc";
export type EventKind = "enter" | "leave";

export interface EntityState {
  id: string;
  type: EntityType;
  name?: string;
  x: number;
  y: number;
  radius: number;
  last_input_seq?: number;
}

export interface WelcomePayload {
  player_id: string;
  map_width: number;
  map_height: number;
  aoi_type: string;
  aoi_radius: number;
  grid_size: number;
  sync_type?: string;
  player_speed?: number;
  server_tick: number;
  entity: EntityState;
}

export interface EntityUpdatePayload {
  entities: EntityState[];
  debug_entities?: EntityState[];
  server_tick: number;
  server_time?: number;
  sync_type?: string;
  full_snapshot?: boolean;
  stats?: ServerStats;
}

export interface ServerStats {
  player_count: number;
  npc_count: number;
  entity_count: number;
  sync_type?: string;
  aoi_query_ms: number;
  tick_duration_ms: number;
  messages_out: number;
  aoi_events: number;
}

export interface EventLogItem {
  id: number;
  tick: number;
  type: EventKind;
  entityId: string;
}

export interface Flash {
  type: EventKind;
  until: number;
}

export interface MovementInput {
  up: boolean;
  down: boolean;
  left: boolean;
  right: boolean;
}

interface EntitySample {
  at: number;
  entity: EntityState;
}

const INTERPOLATION_DELAY_MS = 120;
const MAX_ENTITY_SAMPLES = 12;

export class WorldModel {
  localPlayerId = "";
  mapWidth = 2000;
  mapHeight = 2000;
  aoiType = "-";
  syncType = "-";
  aoiRadius = 200;
  gridSize = 200;
  playerSpeed = 180;
  serverTick = 0;
  latencyMs = 0;
  ackInputSeq = 0;
  reconciliationError = 0;
  visibleEntities = new Map<string, EntityState>();
  allEntities = new Map<string, EntityState>();
  eventLog: EventLogItem[] = [];
  flashes = new Map<string, Flash>();
  stats: ServerStats = {
    player_count: 0,
    npc_count: 0,
    entity_count: 0,
    aoi_query_ms: 0,
    tick_duration_ms: 0,
    messages_out: 0,
    aoi_events: 0,
  };
  private nextEventId = 1;
  private samples = new Map<string, EntitySample[]>();
  private pendingInputs = new Set<number>();
  private predictedLocal?: EntityState;

  get localPlayer(): EntityState | undefined {
    return this.visibleEntities.get(this.localPlayerId) ?? this.allEntities.get(this.localPlayerId);
  }

  applyWelcome(payload: WelcomePayload): void {
    this.localPlayerId = payload.player_id;
    this.mapWidth = payload.map_width;
    this.mapHeight = payload.map_height;
    this.aoiType = payload.aoi_type;
    this.syncType = payload.sync_type || "snapshot";
    this.aoiRadius = payload.aoi_radius;
    this.gridSize = payload.grid_size;
    this.playerSpeed = payload.player_speed || this.playerSpeed;
    this.serverTick = payload.server_tick;
    this.ackInputSeq = payload.entity.last_input_seq || 0;
    this.reconciliationError = 0;
    this.visibleEntities.clear();
    this.allEntities.clear();
    this.eventLog = [];
    this.flashes.clear();
    this.samples.clear();
    this.pendingInputs.clear();
    this.predictedLocal = cloneEntity(payload.entity);
    this.visibleEntities.set(payload.entity.id, payload.entity);
    this.allEntities.set(payload.entity.id, payload.entity);
    this.pushSample(payload.entity, performance.now());
  }

  applyEnter(entity: EntityState): void {
    this.visibleEntities.set(entity.id, entity);
    this.allEntities.set(entity.id, entity);
    this.pushSample(entity, performance.now());
    this.pushEvent("enter", entity.id);
    this.flashes.set(entity.id, { type: "enter", until: performance.now() + 650 });
  }

  applyLeave(entityId: string): void {
    this.visibleEntities.delete(entityId);
    this.samples.delete(entityId);
    this.pushEvent("leave", entityId);
    this.flashes.set(entityId, { type: "leave", until: performance.now() + 650 });
  }

  applyUpdate(payload: EntityUpdatePayload): void {
    if (payload.server_tick < this.serverTick) {
      return;
    }
    const now = performance.now();
    this.serverTick = payload.server_tick;
    this.syncType = payload.sync_type || payload.stats?.sync_type || this.syncType;

    if (payload.full_snapshot !== false) {
      this.visibleEntities = new Map(payload.entities.map((entity) => [entity.id, entity]));
    } else {
      for (const entity of payload.entities) {
        this.visibleEntities.set(entity.id, entity);
      }
    }

    if (payload.debug_entities) {
      this.allEntities = new Map(payload.debug_entities.map((entity) => [entity.id, entity]));
    } else if (payload.full_snapshot !== false) {
      this.allEntities = new Map(this.visibleEntities);
    } else {
      for (const entity of payload.entities) {
        this.allEntities.set(entity.id, entity);
      }
    }
    if (payload.stats) {
      this.stats = payload.stats;
    }

    const sampled = new Set<string>();
    for (const entity of payload.entities) {
      this.pushSample(entity, now);
      sampled.add(entity.id);
    }
    for (const entity of payload.debug_entities || []) {
      if (!sampled.has(entity.id)) {
        this.pushSample(entity, now);
      }
    }

    const local = this.visibleEntities.get(this.localPlayerId);
    if (local) {
      this.reconcileLocal(local);
    }
  }

  recordLocalInput(seq: number | undefined): void {
    if (!seq) {
      return;
    }
    this.pendingInputs.add(seq);
    if (this.pendingInputs.size > 180) {
      const ordered = [...this.pendingInputs].sort((a, b) => a - b);
      for (const oldSeq of ordered.slice(0, this.pendingInputs.size - 180)) {
        this.pendingInputs.delete(oldSeq);
      }
    }
  }

  predictLocal(input: MovementInput, dtMS: number, enabled: boolean): void {
    if (!enabled) {
      this.predictedLocal = undefined;
      this.reconciliationError = 0;
      return;
    }

    const authoritative = this.visibleEntities.get(this.localPlayerId);
    if (!authoritative || dtMS <= 0) {
      return;
    }

    const base = this.predictedLocal ? cloneEntity(this.predictedLocal) : cloneEntity(authoritative);
    const dir = inputDirection(input);
    base.x = clamp(base.x + dir.x * this.playerSpeed * (dtMS / 1000), 0, this.mapWidth);
    base.y = clamp(base.y + dir.y * this.playerSpeed * (dtMS / 1000), 0, this.mapHeight);
    this.predictedLocal = base;
  }

  displayEntity(entity: EntityState, now: number, interpolate: boolean, prediction: boolean): EntityState {
    if (entity.id === this.localPlayerId && prediction && this.predictedLocal) {
      return this.predictedLocal;
    }
    if (!interpolate || entity.id === this.localPlayerId) {
      return entity;
    }
    return this.interpolatedEntity(entity, now);
  }

  pruneFlashes(now: number): void {
    for (const [id, flash] of this.flashes) {
      if (flash.until <= now) {
        this.flashes.delete(id);
      }
    }
  }

  visibleCount(): number {
    return Math.max(0, this.visibleEntities.size - (this.localPlayerId ? 1 : 0));
  }

  totalCount(): number {
    return this.allEntities.size;
  }

  pendingInputCount(): number {
    return this.pendingInputs.size;
  }

  private reconcileLocal(authoritative: EntityState): void {
    const ack = authoritative.last_input_seq || 0;
    if (ack > this.ackInputSeq) {
      this.ackInputSeq = ack;
      for (const seq of this.pendingInputs) {
        if (seq <= ack) {
          this.pendingInputs.delete(seq);
        }
      }
    }

    if (!this.predictedLocal) {
      return;
    }

    const error = distance(this.predictedLocal, authoritative);
    this.reconciliationError = error;
    if (error > 90) {
      this.predictedLocal = cloneEntity(authoritative);
      return;
    }
    if (error > 0.5) {
      this.predictedLocal = {
        ...authoritative,
        x: this.predictedLocal.x * 0.75 + authoritative.x * 0.25,
        y: this.predictedLocal.y * 0.75 + authoritative.y * 0.25,
      };
    }
  }

  private pushSample(entity: EntityState, at: number): void {
    const samples = this.samples.get(entity.id) || [];
    samples.push({ at, entity: cloneEntity(entity) });
    this.samples.set(entity.id, samples.slice(-MAX_ENTITY_SAMPLES));
  }

  private interpolatedEntity(fallback: EntityState, now: number): EntityState {
    const samples = this.samples.get(fallback.id);
    if (!samples || samples.length < 2) {
      return fallback;
    }

    const target = now - INTERPOLATION_DELAY_MS;
    let nextIndex = samples.findIndex((sample) => sample.at >= target);
    if (nextIndex < 0) {
      return samples[samples.length - 1].entity;
    }
    if (nextIndex === 0) {
      return samples[0].entity;
    }

    const previous = samples[nextIndex - 1];
    const next = samples[nextIndex];
    const span = Math.max(1, next.at - previous.at);
    const t = clamp((target - previous.at) / span, 0, 1);
    return {
      ...next.entity,
      x: previous.entity.x + (next.entity.x - previous.entity.x) * t,
      y: previous.entity.y + (next.entity.y - previous.entity.y) * t,
    };
  }

  private pushEvent(type: EventKind, entityId: string): void {
    this.eventLog.unshift({
      id: this.nextEventId++,
      tick: this.serverTick,
      type,
      entityId,
    });
    this.eventLog = this.eventLog.slice(0, 80);
  }
}

function inputDirection(input: MovementInput): { x: number; y: number } {
  let x = 0;
  let y = 0;
  if (input.up) {
    y -= 1;
  }
  if (input.down) {
    y += 1;
  }
  if (input.left) {
    x -= 1;
  }
  if (input.right) {
    x += 1;
  }
  const length = Math.hypot(x, y);
  if (length === 0) {
    return { x: 0, y: 0 };
  }
  return { x: x / length, y: y / length };
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function distance(a: EntityState, b: EntityState): number {
  return Math.hypot(a.x - b.x, a.y - b.y);
}

function cloneEntity(entity: EntityState): EntityState {
  return { ...entity };
}
