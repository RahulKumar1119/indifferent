#!/bin/bash
set -euo pipefail
# Shared stock-media cache (issue #194)
# =====================================
# Pixabay's API terms require requests to be cached for 24 hours. Each Lambda
# container keeps its own cache, so ten containers can ask Pixabay the same
# question on the same day. This gives them one cache to share
# (backend/internal/stockcache). It creates or updates, in this order:
#
#   1. Table      indifferent-fun-stock-cache: string hash key PK, no sort key, on-demand
#                 billing. It has its own table on purpose. The projects table holds
#                 users' work and has no TTL, and switching one on there would put
#                 that work on a deletion timer.
#   2. TTL        on the attribute expiresAt. DynamoDB deletes expired items some time
#                 after they expire (up to 48 hours), so the code also checks the expiry
#                 on every read; the TTL only keeps the table small.
#   3. API Lambda STOCK_CACHE_TABLE merged into indifferent-fun-api's environment (every
#                 other variable written back as it was; the environment holds secrets
#                 and is never printed). The API's role already covers tables named
#                 indifferent-fun-* (deploy/setup.sh), and this script checks that with
#                 the IAM policy simulator before changing anything.
#
# Not done here: the video worker (Fargate). It reads the same variable, but its task
# definition and task role belong to deploy/setup-storyvideo.sh and are not changed by
# this script. Until they are, the worker keeps its own in-memory cache.
#
# The script works in two phases. Phase 1 only reads, and finds every reason to
# refuse. Phase 2 makes the changes, in dependency order. A refusal therefore never
# leaves a half-made setup behind.
#
# Usage:
#   bash deploy/setup-stock-cache.sh            # dry run: read-only calls, prints the plan
#   bash deploy/setup-stock-cache.sh --apply    # makes the changes
#   bash deploy/setup-stock-cache.sh --unwire-api [--apply]
#                                               # removes ONLY STOCK_CACHE_TABLE from the API
#                                               # Lambda (it goes back to per-container caches)
#
# Order:
#   1. bash deploy/setup-stock-cache.sh            (read the plan)
#   2. bash deploy/setup-stock-cache.sh --apply
#   3. deploy the API code (the script prints the commands; it does not do it)
#   The variable is read at start-up, so it takes effect for new containers. The code
#   works without the table, so the steps may be done in either order.
#
# Safety:
#   - Idempotent: every step checks current state first; re-running changes nothing
#     when nothing needs to change.
#   - The dry run only makes read calls (describe/get/list/simulate).
#   - Nothing is deleted, and an existing table is never altered except for turning
#     its TTL on. Switching the cache off again (--unwire-api) leaves the table and
#     its entries; they expire on their own.
#   - The API Lambda's environment is read and written back through a private file
#     (mode 700 directory, removed on exit), guarded by the function's revision id,
#     with the command's own output suppressed (--query). The script prints counts,
#     never values it does not own. Only one key is added.
#   - Needs: aws, python3.
#
# Flush the cache when something that changes what a provider returns has changed,
# such as Pixabay approving the key for full API access:
#   bash deploy/flush-stockmedia-cache.sh --apply
#
# Undo:
#   bash deploy/setup-stock-cache.sh --unwire-api --apply
#   aws dynamodb delete-table --table-name indifferent-fun-stock-cache --region ap-south-1

# Arguments first, so a typo is refused before anything is asked of AWS.
APPLY=false
UNWIRE=false
for arg in "$@"; do
  case "$arg" in
    "") ;;
    --apply) APPLY=true ;;
    --unwire-api) UNWIRE=true ;;
    *)
      echo "Unknown argument: $arg (use --apply, --unwire-api, or no argument)" >&2
      exit 1
      ;;
  esac
done

REGION="ap-south-1"
PROJECT="indifferent-fun"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)

# These three names are the ones backend/internal/stockcache expects; a Go test
# (TestDeployScriptsAgreeWithTheCode) fails if they drift apart.
TABLE="${PROJECT}-stock-cache"
TTL_ATTR="expiresAt"
API_ENV_KEY="STOCK_CACHE_TABLE"

API_FUNCTION="${PROJECT}-api"
API_ROLE_NAME="${PROJECT}-lambda-role"
TABLE_ARN="arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${TABLE}"
API_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${API_ROLE_NAME}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

if $APPLY; then
  echo "MODE: apply (changes will be made in account ${ACCOUNT_ID}, ${REGION})"
else
  echo "MODE: dry run (read-only; nothing will be created or changed)"
fi
if $UNWIRE; then
  echo "      --unwire-api: only ${API_ENV_KEY} on ${API_FUNCTION} is looked at; nothing else is."
fi
echo

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

# act "<what>" <command...>: runs the command when applying, otherwise prints it.
act() {
  local what="$1"
  shift
  if $APPLY; then
    echo ">> ${what}"
    "$@"
  else
    echo "[dry-run] would: ${what}"
  fi
}

cat > "$TMP/helpers.py" <<'PY'
import json
import os
import subprocess
import sys
import tempfile


def load(path):
    with open(path) as f:
        return json.load(f)


def main():
    cmd = sys.argv[1]

    # simulate ROLE_ARN RESOURCE_ARN ACTION...: asks the IAM policy simulator whether
    # the role may do each action on the resource. Exit 0: all allowed. 1: one is not.
    # 2: the simulator could not be used.
    if cmd == "simulate":
        role, resource, actions = sys.argv[2], sys.argv[3], sys.argv[4:]
        req = {"PolicySourceArn": role, "ActionNames": actions, "ResourceArns": [resource]}
        with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as f:
            json.dump(req, f)
        try:
            p = subprocess.run(["aws", "iam", "simulate-principal-policy", "--cli-input-json", "file://" + f.name, "--output", "json"],
                               capture_output=True, text=True)
        finally:
            os.unlink(f.name)
        if p.returncode != 0:
            sys.stderr.write("%s\n" % (p.stderr.strip().splitlines() or ["the simulator call failed"])[-1])
            sys.exit(2)
        wrong = False
        for r in json.loads(p.stdout)["EvaluationResults"]:
            ok = r["EvalDecision"] == "allowed"
            print("  %-4s %s on %s" % ("ok" if ok else "FAIL", r["EvalActionName"], r["EvalResourceName"]))
            wrong = wrong or not ok
        sys.exit(1 if wrong else 0)

    # apienv ...: the API Lambda's configuration holds secrets (the JWT secret, the
    # OAuth secrets). These commands read it from a private file and write the
    # environment to another; they print a status and counts, never a name or a value
    # that is not ours.
    #   apienv ready CONFIG                   exit 0 when no update is in progress
    #   apienv revision CONFIG                prints the revision id
    #   apienv value CONFIG KEY               prints KEY's value (ours: a table name); exit 3 when unset
    #   apienv merge CONFIG KEY VALUE OUT     OUT is the --environment document: every other variable as it was
    #   apienv unmerge CONFIG KEY OUT         the same without KEY
    if cmd == "apienv":
        sub, cfg = sys.argv[2], load(sys.argv[3])
        env = dict((cfg.get("Environment") or {}).get("Variables") or {})
        if sub == "ready":
            st = cfg.get("LastUpdateStatus", "Successful")
            if st == "InProgress":
                sys.stderr.write("the function is still being updated (LastUpdateStatus %s); try again in a minute\n" % st)
                sys.exit(4)
            sys.exit(0)
        if sub == "revision":
            print(cfg["RevisionId"])
            sys.exit(0)
        key = sys.argv[4]
        if sub == "value":
            if key not in env:
                sys.exit(3)
            print(env[key])
            sys.exit(0)
        if sub in ("merge", "unmerge"):
            if sub == "merge":
                env[key] = sys.argv[5]
                out = sys.argv[6]
            else:
                if key not in env:
                    sys.exit(3)
                del env[key]
                out = sys.argv[5]
            with open(out, "w") as f:
                json.dump({"Variables": env}, f)
            print("kept %d other variable(s) as they were" % (len(env) - (1 if sub == "merge" else 0)))
            sys.exit(0)

    sys.exit("unknown command %s" % cmd)


main()
PY

# ===========================================================================
# The API Lambda's environment (used by phase 1 and 2, and by --unwire-api).
#
# update-function-configuration REPLACES the whole environment, and this one holds
# secrets. So it is read into a file in this script's private temporary directory
# (mode 700, removed on exit), merged there, and written back from there.
# --query keeps the command's own answer (which echoes the environment) from being
# printed, and --revision-id makes the write fail, instead of overwriting, if the
# function changed since it was read.
# ===========================================================================
read_api_config() {
  (umask 077 && aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" \
    --output json >"$TMP/api-config.json" 2>"$TMP/api-config.err") ||
    fail "could not read the configuration of ${API_FUNCTION} (does it exist? is this account right?). Run deploy/setup.sh first."
  python3 "$TMP/helpers.py" apienv ready "$TMP/api-config.json" || fail "${API_FUNCTION} cannot be changed right now."
}

write_api_env() {
  local rev
  rev=$(python3 "$TMP/helpers.py" apienv revision "$TMP/api-config.json")
  aws lambda update-function-configuration --function-name "$API_FUNCTION" --region "$REGION" \
    --revision-id "$rev" --environment "file://${TMP}/api-env.json" --query 'LastModified' --output text
  aws lambda wait function-updated --function-name "$API_FUNCTION" --region "$REGION"
}

if $UNWIRE; then
  echo "=== Remove ${API_ENV_KEY} from ${API_FUNCTION} ==="
  read_api_config
  if python3 "$TMP/helpers.py" apienv value "$TMP/api-config.json" "$API_ENV_KEY" >/dev/null; then
    (umask 077 && python3 "$TMP/helpers.py" apienv unmerge "$TMP/api-config.json" "$API_ENV_KEY" "$TMP/api-env.json")
    act "remove ${API_ENV_KEY} from the environment of ${API_FUNCTION} (the other variables are written back as they were)" write_api_env
    echo "New containers then keep their own caches. The table and its entries are left; they expire on their own."
  else
    echo "${API_ENV_KEY} is not set on ${API_FUNCTION}; nothing to do."
  fi
  exit 0
fi

# ===========================================================================
# PHASE 1: CHECK. Read-only. Every reason to refuse is found here, before the
# first change.
# ===========================================================================
echo "=== 1. Check ==="

aws iam get-role --role-name "$API_ROLE_NAME" >/dev/null 2>&1 ||
  fail "role ${API_ROLE_NAME} (the API Lambda's) does not exist. Run deploy/setup.sh first."
read_api_config

# --- the table
TABLE_STATUS=$(aws dynamodb describe-table --table-name "$TABLE" --region "$REGION" --query 'Table.TableStatus' --output text 2>/dev/null || true)
case "$TABLE_STATUS" in
  ACTIVE)
    KEYS=$(aws dynamodb describe-table --table-name "$TABLE" --region "$REGION" --query 'Table.KeySchema[].[AttributeName,KeyType]' --output text)
    [ "$KEYS" = "$(printf 'PK\tHASH')" ] ||
      fail "table ${TABLE} exists but its key is not a single string hash key named PK, which the cache needs. It was not changed. Use another name or remove the table yourself."
    TABLE_ACTION=have
    echo "Table ${TABLE}: found (key PK)."
    ;;
  CREATING | UPDATING)
    fail "table ${TABLE} is still being changed (${TABLE_STATUS}); run this again in a minute."
    ;;
  "" | None)
    TABLE_ACTION=create
    echo "Table ${TABLE}: not found; it will be created."
    ;;
  *) fail "table ${TABLE} is in state ${TABLE_STATUS}; not touching it." ;;
esac

# --- the TTL
TTL_ACTION=enable
if [ "$TABLE_ACTION" = have ]; then
  TTL=$(aws dynamodb describe-time-to-live --table-name "$TABLE" --region "$REGION" \
    --query 'TimeToLiveDescription.[TimeToLiveStatus,AttributeName]' --output text)
  TTL_STATUS=$(printf '%s' "$TTL" | cut -f1)
  TTL_NAME=$(printf '%s' "$TTL" | cut -f2)
  case "$TTL_STATUS" in
    ENABLED | ENABLING)
      [ "$TTL_NAME" = "$TTL_ATTR" ] ||
        fail "table ${TABLE} already expires items on '${TTL_NAME}', but the cache stores its expiry in '${TTL_ATTR}'. A table has one TTL attribute; it was not changed."
      TTL_ACTION=same
      echo "TTL on ${TTL_ATTR}: in place."
      ;;
    DISABLING) fail "TTL on ${TABLE} is being switched off; run this again in an hour (DynamoDB allows one TTL change an hour)." ;;
    *) echo "TTL on ${TTL_ATTR}: not set; it will be turned on." ;;
  esac
else
  echo "TTL on ${TTL_ATTR}: will be turned on once the table is active."
fi

# --- the API Lambda may use the table
echo
echo "The API Lambda's role may read and write the table?"
set +e
python3 "$TMP/helpers.py" simulate "$API_ROLE_ARN" "$TABLE_ARN" dynamodb:GetItem dynamodb:PutItem
SIM=$?
set -e
case "$SIM" in
  0) ;;
  1) fail "the role ${API_ROLE_NAME} may not read and write ${TABLE}. With the cache switched on every call would fail and be logged, and nothing would be cached across containers. Fix the role's policy first (deploy/setup.sh grants table/${PROJECT}-*)." ;;
  *) echo "WARNING: the IAM policy simulator could not be used, so the role's access to the table is NOT checked." \
    "If it is missing you will see stockcache_*_failed lines in the API's log, and searches keep working from memory." ;;
esac

# --- the API environment
API_ENV_ACTION=set
if CURRENT=$(python3 "$TMP/helpers.py" apienv value "$TMP/api-config.json" "$API_ENV_KEY"); then
  if [ "$CURRENT" = "$TABLE" ]; then
    API_ENV_ACTION=same
    echo "${API_ENV_KEY} on ${API_FUNCTION}: already ${TABLE}."
  else
    fail "${API_ENV_KEY} on ${API_FUNCTION} is already set, to '${CURRENT}', not '${TABLE}'. Not overwriting it. Remove it first with --unwire-api if that is what you want."
  fi
else
  echo "${API_ENV_KEY} on ${API_FUNCTION}: not set; it will be set to ${TABLE}."
  (umask 077 && python3 "$TMP/helpers.py" apienv merge "$TMP/api-config.json" "$API_ENV_KEY" "$TABLE" "$TMP/api-env.json")
fi

# ===========================================================================
# PHASE 2: CHANGE. In dependency order: the table, its TTL, then the variable
# that switches the cache on.
# ===========================================================================
echo
echo "=== 2. Changes ==="

echo
echo "--- Table ${TABLE}"
create_table() {
  aws dynamodb create-table --table-name "$TABLE" \
    --attribute-definitions AttributeName=PK,AttributeType=S \
    --key-schema AttributeName=PK,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --region "$REGION" --query 'TableDescription.TableStatus' --output text
  aws dynamodb wait table-exists --table-name "$TABLE" --region "$REGION"
}
if [ "$TABLE_ACTION" = have ]; then
  echo "Up to date."
else
  act "create table ${TABLE} (key PK, on-demand billing) and wait until it is active" create_table
fi

echo
echo "--- TTL on ${TTL_ATTR}"
enable_ttl() {
  aws dynamodb update-time-to-live --table-name "$TABLE" \
    --time-to-live-specification "Enabled=true,AttributeName=${TTL_ATTR}" \
    --region "$REGION" --query 'TimeToLiveSpecification.AttributeName' --output text
}
if [ "$TTL_ACTION" = same ]; then
  echo "Up to date."
else
  act "turn on TTL for ${TTL_ATTR} on ${TABLE}" enable_ttl
fi

echo
echo "--- API Lambda environment ${API_ENV_KEY}"
if [ "$API_ENV_ACTION" = same ]; then
  echo "Up to date."
else
  act "set ${API_ENV_KEY}=${TABLE} on ${API_FUNCTION} (every other variable is written back as it was; undo: bash deploy/setup-stock-cache.sh --unwire-api --apply)" write_api_env
fi

echo
if $APPLY; then
  echo "Done."
else
  echo "Dry run complete. Re-run with --apply to make these changes."
fi
cat <<EOF

Still to do by hand: deploy the API code, which reads ${API_ENV_KEY} when a container starts:
  cd backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bootstrap ./cmd/api
  (cd /tmp && zip -j api.zip bootstrap)
  aws lambda update-function-code --function-name ${API_FUNCTION} --zip-file fileb:///tmp/api.zip --region ${REGION}
(code only; this does not touch the environment variables).

Check it works: search for the same thing twice from two sessions and look in the API's
log group for 'stockcache_enabled', then 'stockcache_shared_hit'. 'stockcache_*_failed'
lines mean the table cannot be used; searches keep working from memory.

To drop every cached search at once (for example after Pixabay approves the key for full
API access, so results carry larger images):
  bash deploy/flush-stockmedia-cache.sh --apply
EOF
