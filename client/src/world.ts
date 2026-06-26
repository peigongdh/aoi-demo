export type EntityType = "player" | "npc";
export type EventKind = "enter" | "leave";

export interface EntityState {
  id: string;
  type: EntityType;
  name?: string;
  x: number;
  y: number;
  radius: number;
}

export interface WelcomePayload {
  player_id: string;
  map_width: number;
  map_height: number;
  aoi_type: string;
  aoi_radius: number;
  grid_size: number;
  server_tick: number;
  entity: EntityState;
}

export interface EntityUpdatePayload {
  entities: EntityState[];
  debug_entities?: EntityState[];
  server_tick: number;
  stats?: ServerStats;
}

export interface ServerStats {
  player_count: number;
  npc_count: number;
  entity_count: number;
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

export class WorldModel {
  localPlayerId = "";
  mapWidth = 2000;
  mapHeight = 2000;
  aoiType = "-";
  aoiRadius = 200;
  gridSize = 200;
  serverTick = 0;
  latencyMs = 0;
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

  get localPlayer(): EntityState | undefined {
    return this.visibleEntities.get(this.localPlayerId) ?? this.allEntities.get(this.localPlayerId);
  }

  applyWelcome(payload: WelcomePayload): void {
    this.localPlayerId = payload.player_id;
    this.mapWidth = payload.map_width;
    this.mapHeight = payload.map_height;
    this.aoiType = payload.aoi_type;
    this.aoiRadius = payload.aoi_radius;
    this.gridSize = payload.grid_size;
    this.serverTick = payload.server_tick;
    this.visibleEntities.clear();
    this.allEntities.clear();
    this.eventLog = [];
    this.flashes.clear();
    this.visibleEntities.set(payload.entity.id, payload.entity);
    this.allEntities.set(payload.entity.id, payload.entity);
  }

  applyEnter(entity: EntityState): void {
    this.visibleEntities.set(entity.id, entity);
    this.allEntities.set(entity.id, entity);
    this.pushEvent("enter", entity.id);
    this.flashes.set(entity.id, { type: "enter", until: performance.now() + 650 });
  }

  applyLeave(entityId: string): void {
    this.visibleEntities.delete(entityId);
    this.pushEvent("leave", entityId);
    this.flashes.set(entityId, { type: "leave", until: performance.now() + 650 });
  }

  applyUpdate(payload: EntityUpdatePayload): void {
    this.serverTick = payload.server_tick;
    this.visibleEntities = new Map(payload.entities.map((entity) => [entity.id, entity]));
    if (payload.debug_entities) {
      this.allEntities = new Map(payload.debug_entities.map((entity) => [entity.id, entity]));
    } else {
      this.allEntities = new Map(this.visibleEntities);
    }
    if (payload.stats) {
      this.stats = payload.stats;
    }
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
