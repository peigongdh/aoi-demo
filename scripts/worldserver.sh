#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"

CONFIG="${CONFIG:-$ROOT_DIR/server/config/dev.yaml}"
RUN_DIR="${RUN_DIR:-$ROOT_DIR/tmp/worldserver}"
BIN="$RUN_DIR/worldserver"
PID_FILE="${PID_FILE:-$RUN_DIR/worldserver.pid}"
LOG_FILE="${LOG_FILE:-$RUN_DIR/worldserver.log}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8100/api/health}"
READY_URL="${READY_URL:-http://127.0.0.1:8100/api/ready}"
CLIENT_URL="${CLIENT_URL:-http://127.0.0.1:8101/}"
STOP_TIMEOUT="${STOP_TIMEOUT:-10}"
LABEL="${LABEL:-local.aoi-demo-worldserver}"
PLIST_SOURCE="${PLIST_SOURCE:-$ROOT_DIR/launchd/$LABEL.plist}"
PLIST_TARGET="${PLIST_TARGET:-$HOME/Library/LaunchAgents/$LABEL.plist}"
CLIENT_LABEL="${CLIENT_LABEL:-local.aoi-demo-client}"
CLIENT_PLIST_SOURCE="${CLIENT_PLIST_SOURCE:-$ROOT_DIR/launchd/$CLIENT_LABEL.plist}"
CLIENT_PLIST_TARGET="${CLIENT_PLIST_TARGET:-$HOME/Library/LaunchAgents/$CLIENT_LABEL.plist}"
CLIENT_PID_FILE="${CLIENT_PID_FILE:-$ROOT_DIR/tmp/client/client.pid}"
CLIENT_LOG_FILE="${CLIENT_LOG_FILE:-$ROOT_DIR/tmp/client/client.log}"
DOMAIN="gui/$(id -u)"

usage() {
	cat <<EOF
Usage: $(basename "$0") <command>

Commands:
  build          Build the worldserver binary
  start          Build and start the worldserver and LAN client
  stop           Gracefully stop the worldserver and LAN client
  restart        Stop and start the worldserver and LAN client
  reload         Validate config, then restart to apply it
  status         Show process and health status
  check-config   Validate the configured YAML file
  logs [-f]      Show recent worldserver and client logs; use -f/--follow to follow
  foreground     Run the worldserver in the foreground for debugging

Environment:
  CONFIG         Config file path (default: $CONFIG)
  RUN_DIR        Runtime directory (default: $RUN_DIR)
  HEALTH_URL     Health endpoint for status (default: $HEALTH_URL)
  READY_URL      Readiness endpoint for status (default: $READY_URL)
  CLIENT_URL     Client endpoint for status (default: $CLIENT_URL)
  LINES          Log lines shown by logs (default: 80)
  STOP_TIMEOUT   Seconds to wait for graceful stop (default: $STOP_TIMEOUT)
  VERSION        Version string embedded into /api/version (default: dev)
  COMMIT         Commit string embedded into /api/version (default: git short SHA)
  BUILD_TIME     Build time embedded into /api/version (default: current UTC time)
EOF
}

ensure_runtime_dir() {
	mkdir -p "$RUN_DIR"
}

install_plist() {
	ensure_runtime_dir
	mkdir -p "$(dirname "$CLIENT_PID_FILE")"
	mkdir -p "$(dirname "$PLIST_TARGET")"
	cp "$PLIST_SOURCE" "$PLIST_TARGET"
}

install_client_plist() {
	mkdir -p "$(dirname "$CLIENT_PID_FILE")" "$(dirname "$CLIENT_PLIST_TARGET")"
	cp "$CLIENT_PLIST_SOURCE" "$CLIENT_PLIST_TARGET"
}

is_loaded() {
	launchctl print "$DOMAIN/$LABEL" >/dev/null 2>&1
}

is_client_loaded() {
	launchctl print "$DOMAIN/$CLIENT_LABEL" >/dev/null 2>&1
}

build_binary() {
	ensure_runtime_dir
	local build_version="${VERSION:-dev}"
	local build_commit="${COMMIT:-unknown}"
	local build_time="${BUILD_TIME:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"

	if [[ "$build_commit" == "unknown" ]] && command -v git >/dev/null 2>&1; then
		build_commit="$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || printf 'unknown')"
	fi

	(cd "$ROOT_DIR" && go build -ldflags "-X main.version=$build_version -X main.commit=$build_commit -X main.buildTime=$build_time" -o "$BIN" ./server/cmd/worldserver)
	echo "built $BIN (version=$build_version commit=$build_commit)"
}

pid_from_file() {
	if [[ -f "$PID_FILE" ]]; then
		cat "$PID_FILE"
	fi
}

client_pid_from_file() {
	if [[ -f "$CLIENT_PID_FILE" ]]; then
		cat "$CLIENT_PID_FILE"
	fi
}

is_running() {
	local pid="${1:-}"
	[[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

current_pid() {
	local pid
	pid="$(pid_from_file || true)"
	if is_running "$pid"; then
		printf '%s\n' "$pid"
		return 0
	fi
	return 1
}

current_client_pid() {
	local pid
	pid="$(client_pid_from_file || true)"
	if is_running "$pid"; then
		printf '%s\n' "$pid"
		return 0
	fi
	return 1
}

clear_stale_pid() {
	local pid
	pid="$(pid_from_file || true)"
	if [[ -n "$pid" ]] && ! is_running "$pid"; then
		rm -f "$PID_FILE"
	fi
	pid="$(client_pid_from_file || true)"
	if [[ -n "$pid" ]] && ! is_running "$pid"; then
		rm -f "$CLIENT_PID_FILE"
	fi
}

check_config() {
	build_binary
	"$BIN" --config "$CONFIG" --check-config
}

foreground_server() {
	check_config >/dev/null
	echo "worldserver foreground: $BIN --config $CONFIG"
	exec "$BIN" --config "$CONFIG"
}

start_server() {
	clear_stale_pid
	if pid="$(current_pid)"; then
		echo "worldserver already running (pid=$pid)"
	else
		check_config >/dev/null
		install_plist
		if is_loaded; then
			launchctl kickstart -k "$DOMAIN/$LABEL"
		else
			launchctl bootstrap "$DOMAIN" "$PLIST_TARGET"
			launchctl kickstart -k "$DOMAIN/$LABEL"
		fi

		local waited=0
		while (( waited < STOP_TIMEOUT )); do
			clear_stale_pid
			if pid="$(current_pid)"; then
				echo "worldserver started (pid=$pid)"
				echo "log: $LOG_FILE"
				break
			fi
			sleep 1
			waited=$((waited + 1))
		done

		if ! current_pid >/dev/null; then
			echo "worldserver failed to start; launchd status:"
			launchctl print "$DOMAIN/$LABEL" 2>/dev/null || true
			echo "recent log:"
			tail -n 40 "$LOG_FILE" 2>/dev/null || true
			return 1
		fi
	fi

	start_client
}

stop_server() {
	clear_stale_pid
	if ! pid="$(current_pid)"; then
		if is_loaded; then
			launchctl bootout "$DOMAIN/$LABEL" || true
			rm -f "$PID_FILE"
		fi
		echo "worldserver is not running"
	else
		if is_loaded; then
			launchctl bootout "$DOMAIN/$LABEL" || true
		else
			kill -TERM "$pid"
		fi
		local waited=0
		while is_running "$pid"; do
			if (( waited >= STOP_TIMEOUT )); then
				echo "worldserver did not stop within ${STOP_TIMEOUT}s (pid=$pid)"
				return 1
			fi
			sleep 1
			waited=$((waited + 1))
		done

		rm -f "$PID_FILE"
		echo "worldserver stopped"
	fi
	stop_client
}

restart_server() {
	stop_server
	start_server
}

reload_server() {
	check_config >/dev/null
	if pid="$(current_pid)"; then
		echo "config ok; reloading by graceful restart (pid=$pid)"
		restart_server
	else
		echo "config ok; worldserver is not running, starting it"
		start_server
	fi
}

status_server() {
	clear_stale_pid
	local status=0
	if pid="$(current_pid)"; then
		echo "worldserver running (pid=$pid)"
		echo "config: $CONFIG"
		echo "log: $LOG_FILE"
		if command -v curl >/dev/null 2>&1; then
			if curl -fsS --max-time 2 "$HEALTH_URL" >/dev/null; then
				echo "health: ok ($HEALTH_URL)"
			else
				echo "health: failed ($HEALTH_URL)"
				status=1
			fi
			if curl -fsS --max-time 2 "$READY_URL" >/dev/null; then
				echo "ready: ok ($READY_URL)"
			else
				echo "ready: failed ($READY_URL)"
				status=1
			fi
		fi
	elif is_loaded; then
		echo "worldserver launchd job is loaded but no live pid was recorded"
		launchctl print "$DOMAIN/$LABEL" | sed -n '1,40p'
		status=1
	else
		echo "worldserver stopped"
		status=3
	fi
	status_client || status=$?
	return "$status"
}

tail_logs() {
	ensure_runtime_dir
	local follow_arg=""
	case "${1:-}" in
		-f|--follow)
			follow_arg="-f"
			;;
		"")
			;;
		*)
			echo "unknown logs option: $1" >&2
			echo "usage: $(basename "$0") logs [-f|--follow]" >&2
			return 2
			;;
	esac
	if [[ -n "$follow_arg" ]]; then
		tail -n "${LINES:-80}" "$follow_arg" "$LOG_FILE" "$CLIENT_LOG_FILE"
	else
		tail -n "${LINES:-80}" "$LOG_FILE" "$CLIENT_LOG_FILE"
	fi
}

start_client() {
	clear_stale_pid
	if pid="$(current_client_pid)"; then
		echo "client already running (pid=$pid)"
		return 0
	fi

	install_client_plist
	if is_client_loaded; then
		launchctl kickstart -k "$DOMAIN/$CLIENT_LABEL"
	else
		launchctl bootstrap "$DOMAIN" "$CLIENT_PLIST_TARGET"
		launchctl kickstart -k "$DOMAIN/$CLIENT_LABEL"
	fi

	local waited=0
	while (( waited < STOP_TIMEOUT )); do
		clear_stale_pid
		if pid="$(current_client_pid)"; then
			echo "client started (pid=$pid)"
			echo "log: $CLIENT_LOG_FILE"
			return 0
		fi
		sleep 1
		waited=$((waited + 1))
	done

	echo "client failed to start; launchd status:"
	launchctl print "$DOMAIN/$CLIENT_LABEL" 2>/dev/null || true
	echo "recent client log:"
	tail -n 40 "$CLIENT_LOG_FILE" 2>/dev/null || true
	return 1
}

stop_client() {
	clear_stale_pid
	if ! pid="$(current_client_pid)"; then
		if is_client_loaded; then
			launchctl bootout "$DOMAIN/$CLIENT_LABEL" || true
			rm -f "$CLIENT_PID_FILE"
		fi
		echo "client is not running"
		return 0
	fi

	if is_client_loaded; then
		launchctl bootout "$DOMAIN/$CLIENT_LABEL" || true
	else
		kill -TERM "$pid"
	fi
	local waited=0
	while is_running "$pid"; do
		if (( waited >= STOP_TIMEOUT )); then
			echo "client did not stop within ${STOP_TIMEOUT}s (pid=$pid)"
			return 1
		fi
		sleep 1
		waited=$((waited + 1))
	done
	rm -f "$CLIENT_PID_FILE"
	echo "client stopped"
}

status_client() {
	clear_stale_pid
	if pid="$(current_client_pid)"; then
		echo "client running (pid=$pid)"
		echo "url: $CLIENT_URL"
		echo "log: $CLIENT_LOG_FILE"
		if command -v curl >/dev/null 2>&1; then
			if curl -fsS --max-time 2 "$CLIENT_URL" >/dev/null; then
				echo "client health: ok ($CLIENT_URL)"
			else
				echo "client health: failed ($CLIENT_URL)"
				return 1
			fi
		fi
	elif is_client_loaded; then
		echo "client launchd job is loaded but no live pid was recorded"
		launchctl print "$DOMAIN/$CLIENT_LABEL" | sed -n '1,40p'
		return 1
	else
		echo "client stopped"
		return 3
	fi
}

command="${1:-}"
if [[ $# -gt 0 ]]; then
	shift
fi
case "$command" in
	build)
		build_binary
		;;
	start)
		start_server
		;;
	stop)
		stop_server
		;;
	restart)
		restart_server
		;;
	reload)
		reload_server
		;;
	status)
		status_server
		;;
	check-config)
		check_config
		;;
	logs)
		tail_logs "$@"
		;;
	foreground)
		foreground_server
		;;
	-h|--help|help|"")
		usage
		;;
	*)
		echo "unknown command: $command" >&2
		usage >&2
		exit 2
		;;
esac
