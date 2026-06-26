#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)"

CONFIG="${CONFIG:-$ROOT_DIR/server/config/dev.yaml}"
RUN_DIR="${RUN_DIR:-$ROOT_DIR/tmp/worldserver}"
BIN="$RUN_DIR/worldserver"
PID_FILE="${PID_FILE:-$RUN_DIR/worldserver.pid}"
LOG_FILE="${LOG_FILE:-$RUN_DIR/worldserver.log}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8100/health}"
STOP_TIMEOUT="${STOP_TIMEOUT:-10}"

usage() {
	cat <<EOF
Usage: $(basename "$0") <command>

Commands:
  start          Build and start the worldserver
  stop           Gracefully stop the worldserver
  restart        Stop and start the worldserver
  reload         Validate config, then restart to apply it
  status         Show process and health status
  check-config   Validate the configured YAML file
  logs           Tail the worldserver log

Environment:
  CONFIG         Config file path (default: $CONFIG)
  RUN_DIR        Runtime directory (default: $RUN_DIR)
  HEALTH_URL     Health endpoint for status (default: $HEALTH_URL)
  STOP_TIMEOUT   Seconds to wait for graceful stop (default: $STOP_TIMEOUT)
EOF
}

ensure_runtime_dir() {
	mkdir -p "$RUN_DIR"
}

build_binary() {
	ensure_runtime_dir
	(cd "$ROOT_DIR" && go build -o "$BIN" ./server/cmd/worldserver)
}

pid_from_file() {
	if [[ -f "$PID_FILE" ]]; then
		cat "$PID_FILE"
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

clear_stale_pid() {
	local pid
	pid="$(pid_from_file || true)"
	if [[ -n "$pid" ]] && ! is_running "$pid"; then
		rm -f "$PID_FILE"
	fi
}

check_config() {
	build_binary
	"$BIN" --config "$CONFIG" --check-config
}

start_server() {
	clear_stale_pid
	if pid="$(current_pid)"; then
		echo "worldserver already running (pid=$pid)"
		return 0
	fi

	check_config >/dev/null
	ensure_runtime_dir
	nohup "$BIN" --config "$CONFIG" >>"$LOG_FILE" 2>&1 &
	local pid=$!
	printf '%s\n' "$pid" >"$PID_FILE"

	sleep 1
	if is_running "$pid"; then
		echo "worldserver started (pid=$pid)"
		echo "log: $LOG_FILE"
		return 0
	fi

	echo "worldserver failed to start; recent log:"
	tail -n 40 "$LOG_FILE" 2>/dev/null || true
	rm -f "$PID_FILE"
	return 1
}

stop_server() {
	clear_stale_pid
	if ! pid="$(current_pid)"; then
		echo "worldserver is not running"
		return 0
	fi

	kill -TERM "$pid"
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
	if pid="$(current_pid)"; then
		echo "worldserver running (pid=$pid)"
		echo "config: $CONFIG"
		echo "log: $LOG_FILE"
		if command -v curl >/dev/null 2>&1; then
			if curl -fsS --max-time 2 "$HEALTH_URL" >/dev/null; then
				echo "health: ok ($HEALTH_URL)"
			else
				echo "health: failed ($HEALTH_URL)"
				return 1
			fi
		fi
	else
		echo "worldserver stopped"
		return 3
	fi
}

tail_logs() {
	ensure_runtime_dir
	tail -n "${LINES:-80}" -f "$LOG_FILE"
}

command="${1:-}"
case "$command" in
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
		tail_logs
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
