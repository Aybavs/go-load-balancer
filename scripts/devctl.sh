#!/bin/sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

RUN_DIR=${RUN_DIR:-"$ROOT_DIR/.run"}
BIN_DIR=${BIN_DIR:-"$ROOT_DIR/bin"}
LB_BIN=${LB_BIN:-"$BIN_DIR/lb"}
BACKEND_BIN=${BACKEND_BIN:-"$BIN_DIR/backend"}
CONFIG_FILE=${CONFIG_FILE:-"$ROOT_DIR/examples/basic/config.yaml"}
LOG_DIR=${LOG_DIR:-"$RUN_DIR/logs"}
STARTUP_WAIT=${STARTUP_WAIT:-0.5}

STARTED_NEW=0

process_is_running() {
	pid=$1

	kill -0 "$pid" 2>/dev/null || return 1

	state=$(ps -p "$pid" -o stat= 2>/dev/null || true)
	[ -n "$state" ] || return 1

	case "$state" in
		*Z*)
			return 1
			;;
	esac

	return 0
}

is_running() {
	service=$1
	pid_file="$RUN_DIR/$service.pid"

	[ -f "$pid_file" ] || return 1

	pid=$(cat "$pid_file")

	case "$pid" in
		''|*[!0-9]*)
			rm -f "$pid_file"
			return 1
			;;
	esac

	if process_is_running "$pid"; then
		return 0
	fi

	rm -f "$pid_file"
	return 1
}

print_status() {
	service=$1

	if is_running "$service"; then
		pid=$(cat "$RUN_DIR/$service.pid")
		echo "$service: running (pid $pid)"
		return 0
	fi

	echo "$service: stopped"
	return 1
}

status_all() {
	result=0

	for service in lb backend-a backend-b; do
		if ! print_status "$service"; then
			result=1
		fi
	done

	return "$result"
}

start_service() {
	service=$1
	shift

	STARTED_NEW=0

	if is_running "$service"; then
		pid=$(cat "$RUN_DIR/$service.pid")
		echo "$service: already running (pid $pid)"
		return 0
	fi

	mkdir -p "$RUN_DIR" "$LOG_DIR"

	nohup "$@" >>"$LOG_DIR/$service.log" 2>&1 &
	pid=$!

	printf '%s\n' "$pid" >"$RUN_DIR/$service.pid"

	sleep "$STARTUP_WAIT"

	if process_is_running "$pid"; then
		STARTED_NEW=1
		echo "$service: started (pid $pid)"
		return 0
	fi

	rm -f "$RUN_DIR/$service.pid"
	echo "$service: failed to start; see $LOG_DIR/$service.log" >&2
	return 1
}

stop_service() {
	service=$1
	pid_file="$RUN_DIR/$service.pid"

	if ! is_running "$service"; then
		rm -f "$pid_file"
		echo "$service: already stopped"
		return 0
	fi

	pid=$(cat "$pid_file")

	kill -TERM "$pid" 2>/dev/null || true

	attempt=0

	while process_is_running "$pid"; do
		if [ "$attempt" -ge 50 ]; then
			echo "$service: did not stop gracefully; sending KILL" >&2
			kill -KILL "$pid" 2>/dev/null || true
			break
		fi

		sleep 0.1
		attempt=$((attempt + 1))
	done

	rm -f "$pid_file"
	echo "$service: stopped"
}

start_all() {
	started_backend_a=0
	started_backend_b=0

	if ! start_service backend-a \
		"$BACKEND_BIN" -name A -addr :9001; then
		return 1
	fi

	started_backend_a=$STARTED_NEW

	if ! start_service backend-b \
		"$BACKEND_BIN" -name B -addr :9002; then
		if [ "$started_backend_a" -eq 1 ]; then
			stop_service backend-a
		fi
		return 1
	fi

	started_backend_b=$STARTED_NEW

	if ! start_service lb \
		"$LB_BIN" -config "$CONFIG_FILE"; then
		if [ "$started_backend_b" -eq 1 ]; then
			stop_service backend-b
		fi

		if [ "$started_backend_a" -eq 1 ]; then
			stop_service backend-a
		fi

		return 1
	fi
}

stop_all() {
	stop_service lb
	stop_service backend-a
	stop_service backend-b
}

reload_lb() {
	if ! is_running lb; then
		echo "lb: not running" >&2
		return 1
	fi

	pid=$(cat "$RUN_DIR/lb.pid")

	if ! kill -HUP "$pid" 2>/dev/null; then
		echo "lb: failed to send HUP to pid $pid" >&2
		return 1
	fi

	echo "lb: reload signal sent (pid $pid)"
}

command=${1:-}

case "$command" in
	start)
		start_all
		;;
	stop)
		stop_all
		;;
	reload)
		reload_lb
		;;
	status)
		status_all
		;;
	*)
		echo "usage: $0 {start|stop|reload|status}" >&2
		exit 2
		;;
esac
