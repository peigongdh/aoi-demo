# AOI Demo

AOI Demo is a local-network MMO interest-management experiment. It contains a Go world server and a TypeScript Canvas client.

## Run the server

```bash
go run ./server/cmd/worldserver --config ./server/config/dev.yaml
```

The default server listens on `0.0.0.0:8100` and accepts WebSocket clients at `/ws`.

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
