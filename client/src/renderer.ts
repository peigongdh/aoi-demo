import type { EntityState, Flash, WorldModel } from "./world";

export interface RenderOptions {
  debugMode: boolean;
  showGrid: boolean;
  showAOI: boolean;
  showAll: boolean;
}

interface Viewport {
  scale: number;
  offsetX: number;
  offsetY: number;
}

export class Renderer {
  private canvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;
  private width = 1;
  private height = 1;
  private dpr = 1;

  constructor(canvas: HTMLCanvasElement) {
    const ctx = canvas.getContext("2d");
    if (!ctx) {
      throw new Error("Canvas 2D context is not available");
    }
    this.canvas = canvas;
    this.ctx = ctx;
    this.resize();
  }

  resize(force = false): void {
    const rect = this.canvas.getBoundingClientRect();
    const dpr = Math.max(1, window.devicePixelRatio || 1);
    const width = Math.max(1, Math.floor(rect.width));
    const height = Math.max(1, Math.floor(rect.height));
    if (!force && width === this.width && height === this.height && dpr === this.dpr) {
      return;
    }
    this.width = width;
    this.height = height;
    this.dpr = dpr;
    this.canvas.width = Math.floor(width * dpr);
    this.canvas.height = Math.floor(height * dpr);
    this.ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  }

  render(world: WorldModel, options: RenderOptions, now: number): void {
    this.resize();
    world.pruneFlashes(now);
    const ctx = this.ctx;
    const viewport = this.viewport(world);

    ctx.clearRect(0, 0, this.width, this.height);
    ctx.fillStyle = "#f6f7f2";
    ctx.fillRect(0, 0, this.width, this.height);

    this.drawMap(world, viewport, options);
    this.drawAOI(world, viewport, options);
    this.drawEntities(world, viewport, options);
  }

  private viewport(world: WorldModel): Viewport {
    const pad = 24;
    const scale = Math.min((this.width - pad * 2) / world.mapWidth, (this.height - pad * 2) / world.mapHeight);
    const safeScale = Number.isFinite(scale) && scale > 0 ? scale : 1;
    return {
      scale: safeScale,
      offsetX: (this.width - world.mapWidth * safeScale) / 2,
      offsetY: (this.height - world.mapHeight * safeScale) / 2,
    };
  }

  private drawMap(world: WorldModel, viewport: Viewport, options: RenderOptions): void {
    const ctx = this.ctx;
    const left = viewport.offsetX;
    const top = viewport.offsetY;
    const width = world.mapWidth * viewport.scale;
    const height = world.mapHeight * viewport.scale;

    ctx.fillStyle = "#ffffff";
    ctx.fillRect(left, top, width, height);
    ctx.strokeStyle = "#222222";
    ctx.lineWidth = 2;
    ctx.strokeRect(left, top, width, height);

    if (!options.debugMode || !options.showGrid) {
      return;
    }

    ctx.save();
    ctx.strokeStyle = "#d0d5cc";
    ctx.lineWidth = 1;
    const step = world.gridSize * viewport.scale;
    for (let x = left + step; x < left + width - 0.5; x += step) {
      ctx.beginPath();
      ctx.moveTo(x, top);
      ctx.lineTo(x, top + height);
      ctx.stroke();
    }
    for (let y = top + step; y < top + height - 0.5; y += step) {
      ctx.beginPath();
      ctx.moveTo(left, y);
      ctx.lineTo(left + width, y);
      ctx.stroke();
    }
    ctx.restore();
  }

  private drawAOI(world: WorldModel, viewport: Viewport, options: RenderOptions): void {
    if (!options.showAOI) {
      return;
    }
    const player = world.localPlayer;
    if (!player) {
      return;
    }

    const ctx = this.ctx;
    const pos = this.toScreen(player, viewport);
    ctx.save();
    ctx.fillStyle = "rgba(41, 128, 185, 0.08)";
    ctx.strokeStyle = "rgba(41, 128, 185, 0.55)";
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.arc(pos.x, pos.y, world.aoiRadius * viewport.scale, 0, Math.PI * 2);
    ctx.fill();
    ctx.stroke();
    ctx.restore();
  }

  private drawEntities(world: WorldModel, viewport: Viewport, options: RenderOptions): void {
    const visible = world.visibleEntities;
    const source = options.debugMode && options.showAll ? world.allEntities : world.visibleEntities;
    const entities = [...source.values()].sort((a, b) => {
      if (a.id === world.localPlayerId) {
        return 1;
      }
      if (b.id === world.localPlayerId) {
        return -1;
      }
      return a.id.localeCompare(b.id);
    });

    for (const entity of entities) {
      const isVisible = visible.has(entity.id);
      const isGhost = options.debugMode && options.showAll && !isVisible;
      this.drawEntity(entity, viewport, world.localPlayerId, world.flashes.get(entity.id), isGhost, options.debugMode);
    }
  }

  private drawEntity(
    entity: EntityState,
    viewport: Viewport,
    localPlayerId: string,
    flash: Flash | undefined,
    isGhost: boolean,
    showLabel: boolean,
  ): void {
    const ctx = this.ctx;
    const pos = this.toScreen(entity, viewport);
    const radius = Math.max(4, entity.radius * viewport.scale);
    const isLocal = entity.id === localPlayerId;

    ctx.save();
    ctx.globalAlpha = isGhost ? 0.22 : 1;
    ctx.fillStyle = this.fillFor(entity, isLocal);
    ctx.strokeStyle = this.strokeFor(entity, isLocal, isGhost);
    ctx.lineWidth = isLocal ? 3 : 2;
    ctx.beginPath();
    ctx.arc(pos.x, pos.y, radius, 0, Math.PI * 2);
    ctx.fill();
    ctx.stroke();

    if (flash) {
      const progress = Math.max(0, (flash.until - performance.now()) / 650);
      ctx.globalAlpha = progress;
      ctx.strokeStyle = flash.type === "enter" ? "#24a148" : "#da1e28";
      ctx.lineWidth = 2;
      ctx.beginPath();
      ctx.arc(pos.x, pos.y, radius + (1 - progress) * 18, 0, Math.PI * 2);
      ctx.stroke();
    }

    if (showLabel && !isGhost) {
      ctx.globalAlpha = 1;
      ctx.font = "11px Inter, ui-sans-serif, system-ui";
      ctx.textAlign = "center";
      ctx.fillStyle = "#2b2f33";
      ctx.fillText(entity.name || entity.id, pos.x, pos.y - radius - 7);
    }
    ctx.restore();
  }

  private fillFor(entity: EntityState, isLocal: boolean): string {
    if (isLocal) {
      return "#2563eb";
    }
    if (entity.type === "player") {
      return "#16a34a";
    }
    return "#f97316";
  }

  private strokeFor(entity: EntityState, isLocal: boolean, isGhost: boolean): string {
    if (isGhost) {
      return "#7b8288";
    }
    if (isLocal) {
      return "#0f3f9f";
    }
    if (entity.type === "player") {
      return "#0f6f38";
    }
    return "#9a3f00";
  }

  private toScreen(entity: EntityState, viewport: Viewport): { x: number; y: number } {
    return {
      x: viewport.offsetX + entity.x * viewport.scale,
      y: viewport.offsetY + entity.y * viewport.scale,
    };
  }
}
