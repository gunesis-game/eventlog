#!/usr/bin/env bash
# Runs `go test -v` with the given extra arguments over every package, keeps
# the full verbose output in the job log, and reports the passed, failed, and
# skipped test counts, with the name of every skipped and failed test, as a
# GitHub Actions notice. The notice appears on the run's summary page and in
# its check-run annotations, which are readable without opening the log.
# Exits with go test's status.
set -u

label="$1"
shift
output="$(mktemp)"

go test -v -count=1 "$@" ./... 2>&1 | tee "$output"
status=${PIPESTATUS[0]}

names() {
	grep -E -- "^ *--- $1: " "$output" | sed -E "s/^ *--- $1: ([^ ]+).*/\1/" | paste -sd ',' - | sed 's/,/, /g'
}
count() {
	grep -cE -- "^ *--- $1: " "$output"
}

passed=$(count PASS)
failed=$(count FAIL)
skipped=$(count SKIP)
summary="passed $passed, failed $failed, skipped $skipped"
if [ "$skipped" -gt 0 ]; then
	summary="$summary (skipped: $(names SKIP))"
fi
if [ "$failed" -gt 0 ]; then
	summary="$summary (failed: $(names FAIL))"
fi

echo "::notice title=$label::$summary"
if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
	echo "**$label:** $summary" >>"$GITHUB_STEP_SUMMARY"
fi
rm -f "$output"
exit "$status"
