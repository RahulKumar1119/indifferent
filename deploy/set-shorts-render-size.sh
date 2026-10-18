#!/bin/bash
set -euo pipefail

# Shorts render task size (issue #137, Part 1)
# ============================================
#
# Registers a new revision of the indifferent-fun-shorts-render ECS task
# definition that is identical to the latest one except for its CPU and memory.
# The state machine runs the task by family name, so new tasks use the newest
# active revision; tasks already running are unaffected.
#
# Why a separate script: deploy/setup-shorts.sh also builds and pushes the
# image and recreates much more. Changing the size should not require that.
# setup-shorts.sh carries the same default (2048 / 4096) so a later full run
# keeps it.
#
# Usage:
#   bash deploy/set-shorts-render-size.sh                          # dry run: 2048 CPU units (2 vCPU), 4096 MB
#   bash deploy/set-shorts-render-size.sh --apply
#   bash deploy/set-shorts-render-size.sh --cpu 1024 --apply       # roll back to 1 vCPU
#   bash deploy/set-shorts-render-size.sh --cpu 2048 --memory 8192 --apply
#
# Safety:
#   - Idempotent: does nothing if the latest revision already has that size.
#   - Only cpu and memory change. Image, roles, logging, environment and
#     everything else are copied from the latest revision as they are.
#   - Rejects CPU/memory pairs Fargate does not accept before calling AWS.
#   - Rolling back registers another revision; no revision is ever deleted.
#
# Cost and capacity: Fargate Spot is billed per vCPU-second and per GB-second,
# so a task that runs about half as long costs about the same. The Spot vCPU
# quota is per account: each clip task uses its full vCPU count, three clips per
# job. At 2 vCPU that is 6 vCPU per job.

REGION="ap-south-1"
PROJECT="indifferent-fun"
FAMILY="${PROJECT}-shorts-render"

CPU=2048
MEMORY=4096
APPLY=false
while [ $# -gt 0 ]; do
  case "$1" in
    --apply) APPLY=true ;;
    --cpu)
      shift
      CPU="${1:-}"
      ;;
    --memory)
      shift
      MEMORY="${1:-}"
      ;;
    *)
      echo "Unknown argument: $1 (use --apply, --cpu N, --memory N)" >&2
      exit 1
      ;;
  esac
  shift
done

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if $APPLY; then echo "MODE: apply"; else echo "MODE: dry run (read-only)"; fi

aws ecs describe-task-definition --task-definition "$FAMILY" --region "$REGION" \
  --query 'taskDefinition' --output json > "$TMP/current.json"

cat > "$TMP/clone.py" <<'PY'
import json
import sys

src, out, cpu, memory = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]

# Fargate's accepted CPU/memory pairs (MB), for the sizes in use here.
VALID = {
    "256": range(512, 2049, 1024),
    "512": range(1024, 4097, 1024),
    "1024": range(2048, 8193, 1024),
    "2048": range(4096, 16385, 1024),
    "4096": range(8192, 30721, 1024),
}
if not cpu.isdigit() or not memory.isdigit():
    sys.exit("cpu and memory must be whole numbers")
if cpu not in VALID:
    sys.exit("cpu %s is not one of %s" % (cpu, ", ".join(VALID)))
if int(memory) not in VALID[cpu]:
    lo, hi = VALID[cpu][0], VALID[cpu][-1]
    sys.exit("Fargate pairs %s CPU units with %d to %d MB in 1024 MB steps, not %s" % (cpu, lo, hi, memory))

td = json.load(open(src))
if td.get("cpu") == cpu and td.get("memory") == memory:
    print("already cpu=%s memory=%s (revision %s)" % (cpu, memory, td.get("revision")))
    sys.exit(3)

# Fields describe-task-definition returns that register-task-definition rejects.
for field in ("taskDefinitionArn", "revision", "status", "requiresAttributes", "compatibilities",
              "registeredAt", "registeredBy", "deregisteredAt"):
    td.pop(field, None)

before = (td.get("cpu"), td.get("memory"))
td["cpu"], td["memory"] = cpu, memory
json.dump(td, open(out, "w"), indent=2)
print("will change cpu/memory from %s/%s to %s/%s" % (before[0], before[1], cpu, memory))
PY

set +e
python3 "$TMP/clone.py" "$TMP/current.json" "$TMP/new.json" "$CPU" "$MEMORY"
RC=$?
set -e
if [ "$RC" -eq 3 ]; then
  echo "Nothing to do."
  exit 0
elif [ "$RC" -ne 0 ]; then
  echo "ERROR: cannot build the new revision." >&2
  exit 1
fi

CURRENT_REV=$(python3 -c "import json; print(json.load(open('$TMP/current.json'))['revision'])")
echo "Latest revision is ${FAMILY}:${CURRENT_REV}. Image, roles, logging and environment are copied unchanged."

if $APPLY; then
  NEW_ARN=$(aws ecs register-task-definition --cli-input-json "file://${TMP}/new.json" --region "$REGION" \
    --query 'taskDefinition.taskDefinitionArn' --output text)
  echo ">> registered ${NEW_ARN}"
  aws ecs describe-task-definition --task-definition "$FAMILY" --region "$REGION" \
    --query 'taskDefinition.{revision:revision,cpu:cpu,memory:memory,status:status}' --output table
  echo "New clip tasks use this revision. To roll back: bash deploy/set-shorts-render-size.sh --cpu 1024 --apply"
else
  echo "[dry-run] would register a new revision with cpu=${CPU} memory=${MEMORY}"
  echo "Re-run with --apply to make the change."
fi
