# AOI Demo

AOI Demo is a local-network MMO interest-management experiment. It contains a Go world server and a TypeScript Canvas client.

## Run the server

```bash
go run ./server/cmd/worldserver --config ./server/config/dev.yaml
```

The default server listens on `0.0.0.0:8100` and accepts WebSocket clients at `/ws`.

You can also manage the server with the helper script:

```bash
./scripts/worldserver.sh start
./scripts/worldserver.sh status
./scripts/worldserver.sh reload
./scripts/worldserver.sh restart
./scripts/worldserver.sh stop
```

Runtime files are written to `tmp/worldserver/`. Use `CONFIG=/path/to/dev.yaml ./scripts/worldserver.sh reload`
to apply another config file after validation. Reload applies config changes with a graceful restart.

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
