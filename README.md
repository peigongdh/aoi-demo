# AOI Demo

AOI Demo is a local-network MMO interest-management experiment. It contains a Go world server and a TypeScript Canvas client.

## Run the server

```bash
go run ./server/cmd/worldserver --config ./server/config/dev.yaml
```

The default server listens on `0.0.0.0:8100` and accepts WebSocket clients at `/ws`.

You can also manage the server with the helper script:

```bash
./scripts/worldserver.sh build
./scripts/worldserver.sh start
./scripts/worldserver.sh status
./scripts/worldserver.sh reload
./scripts/worldserver.sh restart
./scripts/worldserver.sh stop
./scripts/worldserver.sh foreground
```

Runtime files are written to `tmp/worldserver/`. Use `CONFIG=/path/to/dev.yaml ./scripts/worldserver.sh reload`
to apply another config file after validation. Reload applies config changes with a graceful restart.

The worldserver follows the personal service platform endpoints:

- `GET /api/health` returns fast process health.
- `GET /api/ready` returns readiness and dependency details. AOI Demo currently has no service dependencies.
- `GET /api/version` returns version, build, config path, listen address, and non-sensitive runtime config.

The legacy `GET /health` endpoint is kept for compatibility.

## Run the client

```bash
cd client
npm install
npm run dev
```

Open the Vite URL and connect to `ws://localhost:8100/ws`. For LAN testing, replace `localhost` with the server machine IP, for example `ws://192.168.1.10:8100/ws`.

## AOI algorithm switch

Use the AOI selector in the client toolbar, or edit `server/config/dev.yaml` before starting the server:

```yaml
aoi:
  type: "grid"       # bruteforce, grid, or tower
  grid_size: 200
```

## Sync and network simulation

Use the Sync selector in the client toolbar to switch between `snapshot`, `delta`, `interpolation`, and `priority`.
The server also exposes `GET/POST /api/sync` for runtime switching.

The toolbar network controls simulate outbound server delay and packet loss:

- `Latency` and `Jitter` are one-way milliseconds.
- `Loss %` drops realtime `entity_update` messages only; join and AOI enter/leave messages stay reliable.

The same defaults can be edited in `server/config/dev.yaml`:

```yaml
sync:
  type: "snapshot"
  snapshot_rate: 10
  priority_low_rate: 2
  priority_near_ratio: 0.5

network:
  latency_ms: 0
  jitter_ms: 0
  packet_loss: 0
```
