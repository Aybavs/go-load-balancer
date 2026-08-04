#!/bin/sh

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
TEST_RUN_DIR=$(mktemp -d)

cleanup() {
	for pid_file in "$TEST_RUN_DIR"/*.pid; do
		[ -f "$pid_file" ] || continue

		pid=$(cat "$pid_file")

		case "$pid" in
			''|*[!0-9]*)
				continue
				;;
		esac

		kill "$pid" 2>/dev/null || true
	done

	rm -rf "$TEST_RUN_DIR"
}

trap cleanup EXIT HUP INT TERM

fail() {
	echo "FAIL: $1" >&2
	exit 1
}

# Test 1: Initially all services should be stopped.
if output=$(RUN_DIR="$TEST_RUN_DIR" "$SCRIPT_DIR/devctl.sh" status 2>&1); then
	status=0
else
	status=$?
fi

[ "$status" -eq 1 ] ||
	fail "expected status to exit 1 when services are stopped, got $status"

for service in lb backend-a backend-b; do
	echo "$output" | grep -q "$service: stopped" ||
		fail "missing stopped status for $service"
done

# Test 2: Invalid PID files should be removed.
printf '%s\n' "not-a-pid" >"$TEST_RUN_DIR/lb.pid"

if output=$(RUN_DIR="$TEST_RUN_DIR" "$SCRIPT_DIR/devctl.sh" status 2>&1); then
	status=0
else
	status=$?
fi

[ "$status" -eq 1 ] ||
	fail "expected status to exit 1 for a stale PID file"

[ ! -e "$TEST_RUN_DIR/lb.pid" ] ||
	fail "expected stale PID file to be removed"

# Test 3: Start should launch all services.
FAKE_BIN="$TEST_RUN_DIR/fake-service"

printf '%s\n' \
	'#!/bin/sh' \
	'trap "exit 0" TERM INT' \
	'while :; do sleep 1; done' \
	>"$FAKE_BIN"

chmod +x "$FAKE_BIN"

if output=$(
	RUN_DIR="$TEST_RUN_DIR" \
	LB_BIN="$FAKE_BIN" \
	BACKEND_BIN="$FAKE_BIN" \
	CONFIG_FILE="/dev/null" \
	"$SCRIPT_DIR/devctl.sh" start 2>&1
); then
	status=0
else
	status=$?
fi

[ "$status" -eq 0 ] ||
	fail "expected start to succeed, got $status: $output"

STARTED_PIDS=""

for service in lb backend-a backend-b; do
	[ -f "$TEST_RUN_DIR/$service.pid" ] ||
		fail "missing PID file for $service"

	pid=$(cat "$TEST_RUN_DIR/$service.pid")
	STARTED_PIDS="$STARTED_PIDS $pid"

	kill -0 "$pid" 2>/dev/null ||
		fail "$service process is not running"
done

# Test 4: Reload should preserve the load balancer PID.
LB_PID_BEFORE=$(cat "$TEST_RUN_DIR/lb.pid")

if output=$(
	RUN_DIR="$TEST_RUN_DIR" \
	"$SCRIPT_DIR/devctl.sh" reload 2>&1
); then
	status=0
else
	status=$?
fi

[ "$status" -eq 0 ] ||
	fail "expected reload to succeed, got $status: $output"

echo "$output" | grep -q "reload signal sent" ||
	fail "reload confirmation message is missing"

LB_PID_AFTER=$(cat "$TEST_RUN_DIR/lb.pid")

[ "$LB_PID_BEFORE" = "$LB_PID_AFTER" ] ||
	fail "load balancer PID changed after reload"

kill -0 "$LB_PID_AFTER" 2>/dev/null ||
	fail "load balancer stopped after reload"

# Test 5: Stop should terminate all services and remove PID files.
if output=$(
	RUN_DIR="$TEST_RUN_DIR" \
	"$SCRIPT_DIR/devctl.sh" stop 2>&1
); then
	status=0
else
	status=$?
fi

[ "$status" -eq 0 ] ||
	fail "expected stop to succeed, got $status: $output"

for service in lb backend-a backend-b; do
	[ ! -e "$TEST_RUN_DIR/$service.pid" ] ||
		fail "PID file still exists for $service after stop"
done

for pid in $STARTED_PIDS; do
	if kill -0 "$pid" 2>/dev/null; then
		fail "process $pid is still running after stop"
	fi
done

# Test 6: Failed startup should return an error and roll back new services.
FAIL_BIN="$TEST_RUN_DIR/failing-service"

printf '%s\n' \
	'#!/bin/sh' \
	'exit 1' \
	>"$FAIL_BIN"

chmod +x "$FAIL_BIN"

if output=$(
	RUN_DIR="$TEST_RUN_DIR" \
	LB_BIN="$FAIL_BIN" \
	BACKEND_BIN="$FAKE_BIN" \
	CONFIG_FILE="/dev/null" \
	"$SCRIPT_DIR/devctl.sh" start 2>&1
); then
	status=0
else
	status=$?
fi

[ "$status" -ne 0 ] ||
	fail "expected start to fail when the load balancer exits during startup; output: $output"

for service in lb backend-a backend-b; do
	[ ! -e "$TEST_RUN_DIR/$service.pid" ] ||
		fail "PID file remains for $service after failed startup"
done

echo "PASS: lifecycle, reload, and startup rollback checks"
