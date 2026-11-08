#!/bin/sh
# Compares rules alone with rules + Evon on the same calls. Run from backend/.
#   sh ../deploy/evon/evon-hour.sh baseline      free, before renting the GPU
#   EVON_BASE_URL=... EVON_API_KEY=... sh ../deploy/evon/evon-hour.sh evon
#   EVON_BASE_URL=... EVON_API_KEY=... sh ../deploy/evon/evon-hour.sh modes   (Evon alone, Evon adding to rules)
# Evon runs are resumable, and the most useful sets go first in case time runs out.
# Set C prints totals only, so its calls can't be tuned on by accident.
set -e
[ -f go.mod ] || { echo "Run this from backend/"; exit 1; }
B=../benchmark
run() { # name dir extra-flags
  name=$1; dir=$2; shift 2
  echo; echo "=== $name"
  # -resume needs an existing results file, so only an interrupted Evon run uses it.
  resume=
  if [ -n "$RESUME" ] && [ -f "$B/results/$name.json" ]; then resume=-resume; fi
  BENCHMARK_DIR=$B/$dir go run ./cmd/benchmark "$@" $resume -out "$B/results/$name.json" 2>&1 | grep -vE '^(✅|❌)' || true
}
case "$1" in
baseline)
  export EVON_BASE_URL= EVON_MODEL=
  run rules/staged-rules staged
  run rules/holdout-c-rules-current holdout -totals-only
  run rules/dev-rules-current dev -totals-only
  run rules/cases-rules-current cases -totals-only
  ;;
evon)
  [ -n "$EVON_BASE_URL" ] && [ -n "$EVON_API_KEY" ] || { echo "Set EVON_BASE_URL and EVON_API_KEY"; exit 1; }
  export EVON_MODEL=${EVON_MODEL:-evon}
  EVON_TIMEOUT_SECONDS=${EVON_TIMEOUT_SECONDS:-30}; export EVON_TIMEOUT_SECONDS
  go run ./cmd/gnanicheck -langs hi
  RESUME=1
  run evon/staged-evon staged -mode decides
  run evon/holdout-c-evon holdout -totals-only -mode decides
  run evon/dev-evon dev -totals-only -mode decides
  run evon/cases-evon cases -totals-only -mode decides
  echo; echo "Done. STOP THE POD NOW: billing is per minute."
  ;;
modes)
  # Evon with no rules, then Evon able only to add to the rules. Most useful sets first.
  [ -n "$EVON_BASE_URL" ] && [ -n "$EVON_API_KEY" ] || { echo "Set EVON_BASE_URL and EVON_API_KEY"; exit 1; }
  export EVON_MODEL=${EVON_MODEL:-evon} EVON_INTERVAL_SECONDS=${EVON_INTERVAL_SECONDS:-15}
  EVON_TIMEOUT_SECONDS=${EVON_TIMEOUT_SECONDS:-30}; export EVON_TIMEOUT_SECONDS
  RESUME=1
  run evon/staged-evon-only staged -mode only
  run evon/staged-evon-adds staged -mode adds
  run evon/holdout-c-evon-only holdout -totals-only -mode only
  run evon/holdout-c-evon-adds holdout -totals-only -mode adds
  run evon/simple-evon-only simple -totals-only -mode only
  run evon/simple-evon-adds simple -totals-only -mode adds
  run evon/dev-evon-only dev -totals-only -mode only
  run evon/cases-evon-only cases -totals-only -mode only
  echo; echo "Done. STOP THE POD NOW: billing is per minute."
  ;;
*) sed -n 2,6p "$0"; exit 1 ;;
esac
