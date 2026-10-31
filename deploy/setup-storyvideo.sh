#!/bin/bash
set -euo pipefail

# Story2Shorts video pipeline infrastructure (issue #193, slice 3b)
# ==================================================================
#
# Everything the story-video job needs to run, on top of the shorts stack
# (deploy/setup-shorts.sh) and the notifier (deploy/setup-notifications.sh).
# Creates or updates, in this order:
#
#   0. Preflight        the bucket, table, cluster, security group, subnets, the
#                       Step Functions role and the notifier Lambda exist; in
#                       --apply mode, the DEPLOYED notifier knows kind "story"
#   1. Stock API keys   SSM SecureString parameters under /indifferent-fun/story-render/,
#                       created from deploy/.env if missing. The keys never go in
#                       the task definition, a command line, or this script's output.
#   2. IAM              task role (S3 on two prefixes, DynamoDB get/update on one
#                       table, Polly), task execution role (ECR, logs, the stock-key
#                       parameters), and additions to the Step Functions role. Every
#                       policy is checked with the IAM policy simulator BEFORE any
#                       role or policy is created or changed
#   3. S3 lifecycle     expire storyvideo-work/ after 7 days. MERGED into the
#                       bucket's existing rules (put-bucket-lifecycle-configuration
#                       replaces the whole configuration), and refused if any
#                       existing rule could delete the finished videos in storyvideo/
#   4. ECR + image      indifferent-fun-story-render, tagged with the git commit
#   5. Log group        /ecs/indifferent-fun-story-render (30 days)
#   6. Task definition  indifferent-fun-story-render (a new revision only if it changed)
#   7. State machine    indifferent-fun-story-pipeline, rendered by the same Go code
#                       the tests check (backend/cmd/storypipeline), validated by AWS
#   8. API Lambda       STORYVIDEO_STATE_MACHINE_ARN merged into indifferent-fun-api's
#                       environment (every other variable written back as it was; the
#                       environment holds secrets and is never printed), and one
#                       permission added to its role: states:StopExecution on this
#                       state machine's executions
#
# The script works in two phases. Phase 1 only reads, and finds every reason to
# refuse (a missing prerequisite, a lifecycle rule that could delete the videos, a
# wrong IAM permission, an old notifier, a dirty tree, a definition AWS rejects).
# Phase 2 makes the changes, in dependency order. A refusal therefore never leaves
# a half-made stack behind.
#
# Usage:
#   bash deploy/setup-storyvideo.sh            # dry run: read-only calls, prints the plan
#   bash deploy/setup-storyvideo.sh --apply    # makes the changes
#   bash deploy/setup-storyvideo.sh --unwire-api [--apply]
#                                              # removes ONLY STORYVIDEO_STATE_MACHINE_ARN from the
#                                              # API Lambda (POST /story-videos then answers 503)
#
# Order of deployment:
#   1. bash deploy/setup-notifications.sh --apply   (redeploys the notifier; this
#      slice added kind "story". --apply of THIS script refuses to go on without it)
#   2. bash deploy/setup-storyvideo.sh              (read the plan)
#   3. bash deploy/setup-storyvideo.sh --apply
#   4. deploy the API code (the script prints the commands; it does not do it)
#
# Safety:
#   - Idempotent: every step checks current state first; re-running changes nothing
#     when nothing needs to change.
#   - The dry run only makes read calls (describe/get/list/head/validate/simulate)
#     and renders the definition locally. It creates and changes nothing in AWS. It
#     also runs the IAM policy simulator against the policies it WOULD attach, so
#     a wrong action or ARN shows up before anything is created.
#   - Nothing is deleted. Previous versions of what is changed (the bucket's
#     lifecycle configuration, a live state machine definition) are saved to
#     /tmp/<name>-before-<timestamp>.json first.
#   - Image tags are immutable and are the git commit, so an old task definition
#     revision still points at the exact image it was made with.
#   - The API Lambda's environment is the one thing here that holds secrets. It is read
#     and written back through a private file (mode 700 directory, removed on exit),
#     guarded by the function's revision id, with the command's own output suppressed
#     (--query), and the script prints counts, never values. Only one key is added.
#   - Needs: aws, docker (apply only), go, python3, git.
#
# Environment overrides:
#   SHORTS_VPC_ID, SHORTS_SUBNET_IDS   same network as the shorts tasks
#   STORY_RENDER_CPU, STORY_RENDER_MEMORY   default 2048 / 4096
#   STORY_RENDER_TAG                   image tag instead of the git commit (you then
#                                      own whether it matches the code)
#   ALLOW_DIRTY=1                      build the image with uncommitted changes in backend/
#   DEPLOY_ENV_FILE                    where the stock keys are read from (default deploy/.env)
#
# Undo (see also the saved files):
#   aws stepfunctions delete-state-machine --state-machine-arn <arn of indifferent-fun-story-pipeline>
#   aws ecs deregister-task-definition --task-definition indifferent-fun-story-render:<revision>
#   bash deploy/setup-storyvideo.sh --unwire-api --apply
#   aws iam delete-role-policy --role-name indifferent-fun-lambda-role --policy-name indifferent-fun-story-api-permissions
#   aws iam delete-role-policy --role-name indifferent-fun-sfn-role --policy-name indifferent-fun-story-sfn-permissions
#   aws iam delete-role-policy --role-name indifferent-fun-story-render-task-role --policy-name indifferent-fun-story-render-permissions
#   aws iam delete-role-policy --role-name indifferent-fun-story-render-exec-role --policy-name indifferent-fun-story-render-secrets
#   aws iam detach-role-policy --role-name indifferent-fun-story-render-exec-role --policy-arn arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy
#   aws iam delete-role --role-name indifferent-fun-story-render-exec-role
#   aws iam delete-role --role-name indifferent-fun-story-render-task-role
#   aws logs delete-log-group --log-group-name /ecs/indifferent-fun-story-render
#   aws ecr delete-repository --repository-name indifferent-fun-story-render --force
#   aws ssm delete-parameters --names /indifferent-fun/story-render/PIXABAY_API_KEY /indifferent-fun/story-render/PEXELS_API_KEY
#   aws s3api put-bucket-lifecycle-configuration --bucket indifferent-fun-assets \
#       --lifecycle-configuration file:///tmp/indifferent-fun-assets-lifecycle-before-<timestamp>.json
#       (the script prints the exact command, and the delete-bucket-lifecycle one if there was no configuration before)

# Arguments first, so a typo is refused before anything is asked of AWS.
APPLY=false
UNWIRE=false
for arg in "$@"; do
  case "$arg" in
    "") ;; # an empty argument is no argument, as before
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

BUCKET="${PROJECT}-assets"
TABLE="${PROJECT}-projects"
CLUSTER="${PROJECT}-shorts"
SG_NAME="${PROJECT}-shorts-task-sg"
VPC_ID="${SHORTS_VPC_ID:-vpc-1369b878}"
SUBNET_IDS="${SHORTS_SUBNET_IDS:-subnet-f01f6e8b,subnet-880ef0e3,subnet-7bf7d837}"
NOTIFIER="${PROJECT}-notifier"
SFN_ROLE_NAME="${PROJECT}-sfn-role"
API_FUNCTION="${PROJECT}-api"
API_ROLE_NAME="${PROJECT}-lambda-role"
API_ENV_KEY="STORYVIDEO_STATE_MACHINE_ARN"

FAMILY="${PROJECT}-story-render"
CONTAINER="story-render"
REPO="${PROJECT}-story-render"
LOG_GROUP="/ecs/${PROJECT}-story-render"
EXEC_ROLE_NAME="${PROJECT}-story-render-exec-role"
TASK_ROLE_NAME="${PROJECT}-story-render-task-role"
SM_NAME="${PROJECT}-story-pipeline"
SSM_PREFIX="/${PROJECT}/story-render"
CPU="${STORY_RENDER_CPU:-2048}"
MEMORY="${STORY_RENDER_MEMORY:-4096}"

# The two key prefixes in the bucket (backend/internal/storyjob/job.go). They must
# not overlap: the lifecycle rule expires one and must never touch the other.
WORK_PREFIX="storyvideo-work/"
OUT_PREFIX="storyvideo/"

# Environment variable names the worker reads for the stock providers
# (backend/internal/stockmedia/env.go). One is enough; a provider without a key is left out.
SECRET_ENV_NAMES=(PEXELS_API_KEY PIXABAY_API_KEY)

REGISTRY="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"
CLUSTER_ARN="arn:aws:ecs:${REGION}:${ACCOUNT_ID}:cluster/${CLUSTER}"
TASKDEF_ARN="arn:aws:ecs:${REGION}:${ACCOUNT_ID}:task-definition/${FAMILY}"
NOTIFIER_ARN="arn:aws:lambda:${REGION}:${ACCOUNT_ID}:function:${NOTIFIER}"
TABLE_ARN="arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${TABLE}"
SFN_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${SFN_ROLE_NAME}"
API_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${API_ROLE_NAME}"
EXEC_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${EXEC_ROLE_NAME}"
TASK_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${TASK_ROLE_NAME}"
SM_ARN="arn:aws:states:${REGION}:${ACCOUNT_ID}:stateMachine:${SM_NAME}"
EVENTS_RULE_ARN="arn:aws:events:${REGION}:${ACCOUNT_ID}:rule/StepFunctionsGetEventsForECSTaskRule"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BACKEND_DIR="${REPO_ROOT}/backend"
ENV_FILE="${DEPLOY_ENV_FILE:-${SCRIPT_DIR}/.env}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

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
# The command's arguments are never printed, so a secret passed to a function
# cannot reach the output.
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

# Helpers in Python, which the other deploy scripts use for JSON as well.
cat > "$TMP/helpers.py" <<'PY'
import json
import subprocess
import sys


def load(path):
    with open(path) as f:
        return json.load(f)


def main():
    cmd = sys.argv[1]

    # same A B: exit 0 when the two JSON files hold the same value.
    if cmd == "same":
        sys.exit(0 if load(sys.argv[2]) == load(sys.argv[3]) else 1)

    # lifecycle LIVE NEW BEFORE MINSIZE WORK_PREFIX OUT_PREFIX
    # Merges this job's two rules into the bucket's lifecycle configuration.
    # Exit 0: NEW holds the merged rules. Exit 3: already in place. Exit 4: an
    # existing rule could delete the finished videos; nothing is written.
    if cmd == "lifecycle":
        live_path, new_path, before_path, minsize_path, work, out = sys.argv[2:8]
        live = load(live_path)
        rules = live.get("Rules", [])
        mine = [
            {
                "ID": "storyvideo-work-expiry",
                "Status": "Enabled",
                "Filter": {"Prefix": work},
                "Expiration": {"Days": 7},
                "NoncurrentVersionExpiration": {"NoncurrentDays": 1},
                "AbortIncompleteMultipartUpload": {"DaysAfterInitiation": 1},
            },
            {
                # The bucket is versioned: expiring the current version leaves a
                # delete marker, which this removes once nothing is behind it.
                "ID": "storyvideo-work-delete-markers",
                "Status": "Enabled",
                "Filter": {"Prefix": work},
                "Expiration": {"ExpiredObjectDeleteMarker": True},
            },
        ]
        mine_ids = {r["ID"] for r in mine}

        def prefix_of(rule):
            """The key prefix a rule applies to; "" when it can apply anywhere."""
            f = rule.get("Filter")
            if f is None:
                return rule.get("Prefix", "")
            if "Prefix" in f:
                return f["Prefix"]
            if "And" in f:
                return f["And"].get("Prefix", "")
            return ""  # a tag, a size, or no filter at all: not limited to a prefix

        acts = ("Expiration", "NoncurrentVersionExpiration", "Transition", "Transitions", "NoncurrentVersionTransitions")
        conflicts = []
        for r in rules:
            if r.get("ID") in mine_ids or r.get("Status") != "Enabled":
                continue
            p = prefix_of(r)
            # Could this rule reach a key under the output prefix?
            if (out.startswith(p) or p.startswith(out)) and any(k in r for k in acts):
                conflicts.append("%s (prefix %r)" % (r.get("ID", "<no id>"), p))
        if conflicts:
            sys.stderr.write(
                "These existing lifecycle rules could expire or move objects under %s, "
                "which holds the finished videos: %s\n" % (out, ", ".join(conflicts))
            )
            sys.exit(4)

        merged = [r for r in rules if r.get("ID") not in mine_ids] + mine
        key = lambda r: json.dumps(r, sort_keys=True)
        if sorted(map(key, rules)) == sorted(map(key, merged)):
            sys.exit(3)
        json.dump({"Rules": merged}, open(new_path, "w"), indent=2)
        if rules:
            json.dump({"Rules": rules}, open(before_path, "w"), indent=2)
        # Put replaces the whole configuration, so keep this setting as it was.
        open(minsize_path, "w").write(live.get("TransitionDefaultMinimumObjectSize", ""))
        sys.exit(0)

    # containers OUT   (reads IMAGE, CONTAINER, BUCKET, TABLE, REGION, LOG_GROUP, SSM_PREFIX, ACCOUNT_ID, SECRET_NAMES)
    if cmd == "containers":
        import os

        e = os.environ
        secrets = [
            {
                "name": n,
                "valueFrom": "arn:aws:ssm:%s:%s:parameter%s/%s" % (e["REGION"], e["ACCOUNT_ID"], e["SSM_PREFIX"], n),
            }
            for n in e["SECRET_NAMES"].split()
        ]
        c = {
            "name": e["CONTAINER"],
            "image": e["IMAGE"],
            "essential": True,
            "environment": [
                {"name": "S3_BUCKET", "value": e["BUCKET"]},
                {"name": "DYNAMODB_TABLE", "value": e["TABLE"]},
            ],
            "secrets": secrets,
            "logConfiguration": {
                "logDriver": "awslogs",
                "options": {
                    "awslogs-group": e["LOG_GROUP"],
                    "awslogs-region": e["REGION"],
                    "awslogs-stream-prefix": "render",
                },
            },
        }
        json.dump([c], open(sys.argv[2], "w"), indent=2)
        sys.exit(0)

    # taskdef LIVE CONTAINERS CPU MEMORY EXEC_ROLE TASK_ROLE: exit 0 when the live
    # task definition already is what would be registered.
    if cmd == "taskdef":
        live_path, containers_path, cpu, memory, exec_role, task_role = sys.argv[2:8]
        td = load(live_path)["taskDefinition"]
        want = load(containers_path)

        def canon(c):
            return {
                "name": c["name"],
                "image": c["image"],
                "essential": c.get("essential", True),
                "environment": sorted((x["name"], x["value"]) for x in c.get("environment", [])),
                "secrets": sorted((x["name"], x["valueFrom"]) for x in c.get("secrets", [])),
                "log": c.get("logConfiguration", {}),
            }

        same = (
            str(td.get("cpu")) == cpu
            and str(td.get("memory")) == memory
            and td.get("executionRoleArn") == exec_role
            and td.get("taskRoleArn") == task_role
            and td.get("runtimePlatform", {}).get("cpuArchitecture") == "X86_64"
            and len(td["containerDefinitions"]) == len(want) == 1
            and canon(td["containerDefinitions"][0]) == canon(want[0])
        )
        sys.exit(0 if same else 1)

    # simulate SPEC: runs the IAM policy simulator. Exit 0: every check came out as
    # expected. 1: a check did not. 2: the simulator could not be used.
    if cmd == "simulate":
        import os
        import tempfile

        spec = load(sys.argv[2])
        policies = [json.dumps(load(p), separators=(",", ":")) for p in spec["policy_files"]]
        wrong, unavailable = [], False
        for chk in spec["checks"]:
            req = {"ActionNames": chk["actions"], "ResourceArns": [chk["resource"]], "PolicyInputList": policies}
            if spec.get("role"):
                op, req["PolicySourceArn"] = "simulate-principal-policy", spec["role"]
            else:
                op = "simulate-custom-policy"
            with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False) as f:
                json.dump(req, f)
            try:
                p = subprocess.run(["aws", "iam", op, "--cli-input-json", "file://" + f.name, "--output", "json"], capture_output=True, text=True)
            finally:
                os.unlink(f.name)
            if p.returncode != 0:
                sys.stderr.write("%s\n" % (p.stderr.strip().splitlines() or ["the simulator call failed"])[-1])
                unavailable = True
                break
            for r in json.loads(p.stdout)["EvaluationResults"]:
                got = r["EvalDecision"] == "allowed"
                ok = got == chk["allow"]
                print("  %-4s %s %s on %s" % ("ok" if ok else "FAIL", "allows" if chk["allow"] else "denies", r["EvalActionName"], r["EvalResourceName"]))
                if not ok:
                    wrong.append(r["EvalActionName"])
        if unavailable:
            sys.exit(2)
        sys.exit(1 if wrong else 0)

    # apienv ...: the API Lambda's configuration holds secrets (the JWT secret, the
    # OAuth secrets). These commands read it from a private file and write the
    # environment to another; they print a status and counts, never a name that is
    # not ours and never a value that is not ours.
    #   apienv ready CONFIG                   exit 0 when no update is in progress
    #   apienv revision CONFIG                prints the revision id
    #   apienv status CONFIG KEY VALUE        exit 0 when KEY already is VALUE, 3 when it is missing or different
    #   apienv present CONFIG KEY             exit 0 when KEY is set
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
        if sub == "present":
            sys.exit(0 if key in env else 3)
        if sub == "status":
            sys.exit(0 if env.get(key) == sys.argv[5] else 3)
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
# secrets (the JWT secret, the OAuth secrets). So it is read into a file in this
# script's private temporary directory (mode 700, removed on exit), merged there,
# and written back from there. Nothing from it reaches the screen: --query keeps
# the command's own answer (which echoes the environment) from being printed, and
# the helpers print only counts. --revision-id makes the write fail, instead of
# overwriting, if the function changed since it was read.
# ===========================================================================
API_ARN_VALUE="$SM_ARN"

# read_api_config writes the function's configuration to $TMP/api-config.json.
read_api_config() {
  (umask 077 && aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" \
    --output json >"$TMP/api-config.json" 2>"$TMP/api-config.err") ||
    fail "could not read the configuration of ${API_FUNCTION} (does it exist? is this account right?). Run deploy/setup.sh first."
  python3 "$TMP/helpers.py" apienv ready "$TMP/api-config.json" || fail "${API_FUNCTION} cannot be changed right now."
}

# write_api_env sends $TMP/api-env.json, guarded by the revision that was read.
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
  if python3 "$TMP/helpers.py" apienv present "$TMP/api-config.json" "$API_ENV_KEY"; then
    (umask 077 && python3 "$TMP/helpers.py" apienv unmerge "$TMP/api-config.json" "$API_ENV_KEY" "$TMP/api-env.json")
    act "remove ${API_ENV_KEY} from the environment of ${API_FUNCTION} (the other variables are written back as they were)" write_api_env
    echo "POST /story-videos then answers 503 (the other story video routes keep working)."
  else
    echo "${API_ENV_KEY} is not set on ${API_FUNCTION}; nothing to do."
  fi
  exit 0
fi

# ===========================================================================
# PHASE 1: CHECK. Read-only (a dry run stops after the plan). Every reason to
# refuse is found here, before the first change, so a refusal never leaves a
# half-made stack behind.
# ===========================================================================

# ---------------------------------------------------------------------------
echo "=== 0. Preflight ==="

aws s3api head-bucket --bucket "$BUCKET" --region "$REGION" >/dev/null 2>&1 ||
  fail "bucket ${BUCKET} does not exist or is not accessible. Run deploy/setup.sh first."
aws dynamodb describe-table --table-name "$TABLE" --region "$REGION" --query 'Table.TableStatus' --output text >/dev/null 2>&1 ||
  fail "table ${TABLE} does not exist or is not accessible. Run deploy/setup.sh first."

CLUSTER_STATUS=$(aws ecs describe-clusters --clusters "$CLUSTER" --region "$REGION" --query 'clusters[0].status' --output text 2>/dev/null || true)
CLUSTER_PROVIDERS=$(aws ecs describe-clusters --clusters "$CLUSTER" --region "$REGION" --query 'clusters[0].capacityProviders' --output text 2>/dev/null || true)
[ "$CLUSTER_STATUS" = "ACTIVE" ] || fail "ECS cluster ${CLUSTER} is not active (${CLUSTER_STATUS:-missing}). Run deploy/setup-shorts.sh first."
for cp in FARGATE FARGATE_SPOT; do
  echo "$CLUSTER_PROVIDERS" | tr -s '[:space:]' '\n' | grep -qx "$cp" ||
    fail "cluster ${CLUSTER} has no ${cp} capacity provider (it has: ${CLUSTER_PROVIDERS:-none})."
done

SECURITY_GROUP_ID=$(aws ec2 describe-security-groups \
  --filters "Name=group-name,Values=${SG_NAME}" "Name=vpc-id,Values=${VPC_ID}" \
  --region "$REGION" --query 'SecurityGroups[0].GroupId' --output text 2>/dev/null || true)
case "$SECURITY_GROUP_ID" in
  sg-*) ;;
  *) fail "security group ${SG_NAME} not found in ${VPC_ID}. Run deploy/setup-shorts.sh first." ;;
esac

# Word splitting is the point here: one argument per subnet id.
# shellcheck disable=SC2086
SUBNETS_FOUND=$(aws ec2 describe-subnets --subnet-ids ${SUBNET_IDS//,/ } \
  --filters "Name=vpc-id,Values=${VPC_ID}" --region "$REGION" --query 'length(Subnets)' --output text 2>/dev/null || true)
SUBNETS_WANT=$(echo "$SUBNET_IDS" | tr ',' '\n' | grep -c . || true)
[ "$SUBNETS_FOUND" = "$SUBNETS_WANT" ] ||
  fail "expected ${SUBNETS_WANT} subnets in ${VPC_ID} from SHORTS_SUBNET_IDS (${SUBNET_IDS}), found ${SUBNETS_FOUND:-none}."

aws iam get-role --role-name "$SFN_ROLE_NAME" >/dev/null 2>&1 ||
  fail "role ${SFN_ROLE_NAME} does not exist. Run deploy/setup.sh first."
aws lambda get-function-configuration --function-name "$NOTIFIER" --region "$REGION" --query 'FunctionName' --output text >/dev/null 2>&1 ||
  fail "Lambda ${NOTIFIER} does not exist. Run deploy/setup-notifications.sh --apply first."

aws iam get-role --role-name "$API_ROLE_NAME" >/dev/null 2>&1 ||
  fail "role ${API_ROLE_NAME} (the API Lambda's) does not exist. Run deploy/setup.sh first."
read_api_config

echo "Bucket, table, cluster (FARGATE + FARGATE_SPOT), security group ${SECURITY_GROUP_ID}, ${SUBNETS_FOUND} subnets, ${SFN_ROLE_NAME}, ${NOTIFIER}, ${API_FUNCTION} and ${API_ROLE_NAME}: found."

# The state machine sends the job to the notifier with kind "story". A notifier
# deployed before this slice answers 'unknown kind' and no email would ever be
# sent, with nothing failing to say so. A story payload with no jobId is
# rejected by validation before the notifier touches anything.
if $APPLY; then
  aws lambda invoke --function-name "$NOTIFIER" \
    --payload '{"kind":"story","outcome":"completed","userId":"deploy-check","executionId":"deploy-check"}' \
    --cli-binary-format raw-in-base64-out --region "$REGION" "$TMP/probe.json" >/dev/null
  if grep -q 'jobId is required for kind story' "$TMP/probe.json"; then
    echo "The deployed notifier knows kind \"story\"."
  elif grep -q 'unknown kind' "$TMP/probe.json"; then
    fail "the deployed notifier is older than this change and does not know kind \"story\". Run deploy/setup-notifications.sh --apply first."
  else
    echo "Unexpected response from the notifier:" >&2
    cat "$TMP/probe.json" >&2
    exit 1
  fi
else
  echo "[dry-run] would: check that the deployed notifier knows kind \"story\" (an invalid test invocation; skipped in a dry run)"
fi

# ---------------------------------------------------------------------------
echo
echo "=== 1. Stock API keys (${SSM_PREFIX}/) ==="

# Reads NAME=value from the env file without running it. The last assignment
# wins; one pair of surrounding quotes and a trailing CR are removed.
env_file_value() {
  [ -f "$ENV_FILE" ] || return 0
  sed -n "s/^[[:space:]]*\(export[[:space:]]\{1,\}\)\{0,1\}$1=//p" "$ENV_FILE" | tail -n 1 | tr -d '\r' |
    sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//' -e 's/^"\(.*\)"$/\1/' -e "s/^'\(.*\)'\$/\1/"
}

# True when the env file holds a real value (not empty, not the .env.example placeholder).
has_real_value() {
  case "$(env_file_value "$1")" in
    "" | your-*) return 1 ;;
  esac
}

SECRET_NAMES_PRESENT=()   # parameters that exist, or will be created
SECRETS_TO_CREATE=()
for name in "${SECRET_ENV_NAMES[@]}"; do
  type=$(aws ssm get-parameter --name "${SSM_PREFIX}/${name}" --region "$REGION" --query 'Parameter.Type' --output text 2>/dev/null || true)
  if [ "$type" = "SecureString" ]; then
    echo "${name}: parameter exists (left as it is)."
    SECRET_NAMES_PRESENT+=("$name")
  elif [ -n "$type" ]; then
    fail "${SSM_PREFIX}/${name} exists as type ${type}, not SecureString. Replace it with a SecureString before running this."
  elif has_real_value "$name"; then
    echo "${name}: no parameter yet; one will be created from ${ENV_FILE}."
    SECRET_NAMES_PRESENT+=("$name")
    SECRETS_TO_CREATE+=("$name")
  else
    echo "${name}: no parameter, and no real value in ${ENV_FILE} (left out)."
  fi
done
[ "${#SECRET_NAMES_PRESENT[@]}" -gt 0 ] ||
  fail "no stock key: put PIXABAY_API_KEY (or PEXELS_API_KEY) in ${ENV_FILE}, or create the SSM parameter yourself. Without one the task cannot fetch pictures."
SECRET_NAMES="${SECRET_NAMES_PRESENT[*]}"

# put_secret NAME: the value goes through a private file, never an argument.
put_secret() {
  local file="$TMP/secret"
  (umask 077 && printf '%s' "$(env_file_value "$1")" >"$file")
  aws ssm put-parameter --name "${SSM_PREFIX}/$1" --type SecureString \
    --description "Stock media API key for the story-render task (issue #193)" \
    --value "file://${file}" --region "$REGION" --query 'Version' --output text
  rm -f "$file"
}

# ---------------------------------------------------------------------------
echo
echo "=== 2. IAM: write the policies and check them with the simulator ==="

cat >"$TMP/trust.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {"Service": "ecs-tasks.amazonaws.com"},
    "Action": "sts:AssumeRole"
  }]
}
EOF

# The task: only what the worker calls (see backend/cmd/storyrender). It has no
# s3:ListBucket because the worker treats a 403 on HeadObject as "not there".
cat >"$TMP/task-policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ReadAndWriteTheJobsFilesOnly",
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:PutObject"],
      "Resource": [
        "arn:aws:s3:::${BUCKET}/${WORK_PREFIX}*",
        "arn:aws:s3:::${BUCKET}/${OUT_PREFIX}*"
      ]
    },
    {
      "Sid": "ReadAndUpdateTheJobRecord",
      "Effect": "Allow",
      "Action": ["dynamodb:GetItem", "dynamodb:UpdateItem"],
      "Resource": "${TABLE_ARN}"
    },
    {
      "Sid": "Narrate",
      "Effect": "Allow",
      "Action": "polly:SynthesizeSpeech",
      "Resource": "*"
    }
  ]
}
EOF

# The execution role pulls the image, writes logs (the managed policy) and reads
# the stock keys into the container at start (this).
SECRET_ARNS=""
for n in "${SECRET_NAMES_PRESENT[@]}"; do
  SECRET_ARNS="${SECRET_ARNS}${SECRET_ARNS:+, }\"arn:aws:ssm:${REGION}:${ACCOUNT_ID}:parameter${SSM_PREFIX}/${n}\""
done
cat >"$TMP/exec-policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Sid": "ReadTheStockKeys",
    "Effect": "Allow",
    "Action": "ssm:GetParameters",
    "Resource": [${SECRET_ARNS}]
  }]
}
EOF

# What Step Functions needs on top of what setup-shorts.sh gave its role (ecs:RunTask
# and the events rule for runTask.sync, and the shorts roles): to pass THESE two
# roles to the task, and to read and mark the job record itself.
cat >"$TMP/sfn-policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PassTheStoryRenderRoles",
      "Effect": "Allow",
      "Action": "iam:PassRole",
      "Resource": ["${EXEC_ROLE_ARN}", "${TASK_ROLE_ARN}"]
    },
    {
      "Sid": "ReadAndMarkTheJobRecord",
      "Effect": "Allow",
      "Action": ["dynamodb:GetItem", "dynamodb:UpdateItem"],
      "Resource": "${TABLE_ARN}"
    }
  ]
}
EOF

# The IAM policy simulator, on the policies as they WILL be attached (so it works
# in a dry run too). For the Step Functions role the simulation covers what the
# role already has plus the new policy, so it also shows that the shorts
# permissions (ecs:RunTask, the events rule, lambda:InvokeFunction) are in place.
# The negative checks confirm the task role cannot reach anything else.
sim() { # label specfile
  local rc
  echo "${1}:"
  set +e
  python3 "$TMP/helpers.py" simulate "$2"
  rc=$?
  set -e
  case "$rc" in
    0) ;;
    1) fail "${1}: a permission is not what it should be (see FAIL above). Nothing has been created or changed." ;;
    *) echo "  WARNING: the simulator could not be used (it needs iam:SimulatePrincipalPolicy), so these permissions are NOT checked." ;;
  esac
}

cat >"$TMP/sim-task.json" <<EOF
{
  "policy_files": ["${TMP}/task-policy.json"],
  "checks": [
    {"actions": ["s3:GetObject", "s3:PutObject"], "resource": "arn:aws:s3:::${BUCKET}/${WORK_PREFIX}u/j/scene-01-voice.mp3", "allow": true},
    {"actions": ["s3:GetObject", "s3:PutObject"], "resource": "arn:aws:s3:::${BUCKET}/${OUT_PREFIX}u/j/video.mp4", "allow": true},
    {"actions": ["dynamodb:GetItem", "dynamodb:UpdateItem"], "resource": "${TABLE_ARN}", "allow": true},
    {"actions": ["polly:SynthesizeSpeech"], "resource": "*", "allow": true},
    {"actions": ["s3:GetObject", "s3:PutObject"], "resource": "arn:aws:s3:::${BUCKET}/temp/p/audio/q1.mp3", "allow": false},
    {"actions": ["s3:GetObject"], "resource": "arn:aws:s3:::${BUCKET}/storyvideo-other/x", "allow": false},
    {"actions": ["s3:DeleteObject"], "resource": "arn:aws:s3:::${BUCKET}/${OUT_PREFIX}u/j/video.mp4", "allow": false},
    {"actions": ["dynamodb:PutItem", "dynamodb:DeleteItem", "dynamodb:Query"], "resource": "${TABLE_ARN}", "allow": false},
    {"actions": ["dynamodb:GetItem"], "resource": "arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${PROJECT}-users", "allow": false}
  ]
}
EOF
sim "task role ${TASK_ROLE_NAME}" "$TMP/sim-task.json"

cat >"$TMP/sim-exec.json" <<EOF
{
  "policy_files": ["${TMP}/exec-policy.json"],
  "checks": [
    {"actions": ["ssm:GetParameters"], "resource": "arn:aws:ssm:${REGION}:${ACCOUNT_ID}:parameter${SSM_PREFIX}/${SECRET_NAMES_PRESENT[0]}", "allow": true},
    {"actions": ["ssm:GetParameters"], "resource": "arn:aws:ssm:${REGION}:${ACCOUNT_ID}:parameter/${PROJECT}/other/JWT_SECRET", "allow": false}
  ]
}
EOF
sim "execution role ${EXEC_ROLE_NAME} (its own policy; the managed one adds ECR and logs)" "$TMP/sim-exec.json"

cat >"$TMP/sim-sfn.json" <<EOF
{
  "role": "${SFN_ROLE_ARN}",
  "policy_files": ["${TMP}/sfn-policy.json"],
  "checks": [
    {"actions": ["ecs:RunTask"], "resource": "${TASKDEF_ARN}:1", "allow": true},
    {"actions": ["ecs:StopTask", "ecs:DescribeTasks"], "resource": "arn:aws:ecs:${REGION}:${ACCOUNT_ID}:task/${CLUSTER}/abc", "allow": true},
    {"actions": ["events:PutTargets", "events:PutRule", "events:DescribeRule"], "resource": "${EVENTS_RULE_ARN}", "allow": true},
    {"actions": ["iam:PassRole"], "resource": "${EXEC_ROLE_ARN}", "allow": true},
    {"actions": ["iam:PassRole"], "resource": "${TASK_ROLE_ARN}", "allow": true},
    {"actions": ["dynamodb:GetItem", "dynamodb:UpdateItem"], "resource": "${TABLE_ARN}", "allow": true},
    {"actions": ["lambda:InvokeFunction"], "resource": "${NOTIFIER_ARN}", "allow": true}
  ]
}
EOF
sim "Step Functions role ${SFN_ROLE_NAME}" "$TMP/sim-sfn.json"

# The API Lambda's role (deploy/setup.sh) already reads and writes the table and
# the bucket, and may start executions of any ${PROJECT}-* state machine. What it
# lacks is stopping one: states:StopExecution is authorised on the EXECUTION, an
# ARN of the form ...:execution:<machine>:<name>, and setup.sh grants it only on
# ...:stateMachine:..., which never matches. (The Service Authorization
# Reference lists StartExecution under the state machine and StopExecution under
# the execution.) This adds it, for this state machine's executions only.
cat >"$TMP/api-policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [{
    "Sid": "StopStoryVideoExecutions",
    "Effect": "Allow",
    "Action": "states:StopExecution",
    "Resource": "arn:aws:states:${REGION}:${ACCOUNT_ID}:execution:${SM_NAME}:*"
  }]
}
EOF
# Simulated as the role's existing policies plus this one, so the checks that
# the rest is already granted (and that the new grant is narrow) are real.
cat >"$TMP/sim-api.json" <<EOF
{
  "role": "${API_ROLE_ARN}",
  "policy_files": ["${TMP}/api-policy.json"],
  "checks": [
    {"actions": ["states:StartExecution"], "resource": "${SM_ARN}", "allow": true},
    {"actions": ["states:StopExecution"], "resource": "arn:aws:states:${REGION}:${ACCOUNT_ID}:execution:${SM_NAME}:01939f6c-8a40-7e21-b2c4-aa11bb22cc33", "allow": true},
    {"actions": ["states:StopExecution"], "resource": "arn:aws:states:${REGION}:${ACCOUNT_ID}:execution:some-other-machine:01939f6c-8a40-7e21-b2c4-aa11bb22cc33", "allow": false},
    {"actions": ["s3:GetObject"], "resource": "arn:aws:s3:::${BUCKET}/${OUT_PREFIX}u/j/video.mp4", "allow": true},
    {"actions": ["s3:DeleteObjectVersion"], "resource": "arn:aws:s3:::${BUCKET}/${OUT_PREFIX}u/j/video.mp4", "allow": true},
    {"actions": ["s3:DeleteObjectVersion"], "resource": "arn:aws:s3:::${BUCKET}/${WORK_PREFIX}u/j/scene-01-voice.mp3", "allow": true},
    {"actions": ["s3:ListBucketVersions"], "resource": "arn:aws:s3:::${BUCKET}", "allow": true},
    {"actions": ["dynamodb:GetItem", "dynamodb:PutItem", "dynamodb:UpdateItem", "dynamodb:DeleteItem", "dynamodb:Query"], "resource": "${TABLE_ARN}", "allow": true}
  ]
}
EOF
sim "API Lambda role ${API_ROLE_NAME}" "$TMP/sim-api.json"

# ---------------------------------------------------------------------------
echo
echo "=== 3. S3 lifecycle: expire ${WORK_PREFIX} in ${BUCKET} ==="
if aws s3api get-bucket-lifecycle-configuration --bucket "$BUCKET" --region "$REGION" --output json >"$TMP/lifecycle.live.json" 2>"$TMP/lifecycle.err"; then
  :
elif grep -q NoSuchLifecycleConfiguration "$TMP/lifecycle.err"; then
  echo '{"Rules": []}' >"$TMP/lifecycle.live.json"
else
  cat "$TMP/lifecycle.err" >&2
  fail "could not read the lifecycle configuration of ${BUCKET}."
fi

set +e
python3 "$TMP/helpers.py" lifecycle "$TMP/lifecycle.live.json" "$TMP/lifecycle.new.json" "$TMP/lifecycle.before.json" "$TMP/lifecycle.minsize" "$WORK_PREFIX" "$OUT_PREFIX"
LC_RC=$?
set -e
case "$LC_RC" in
  3) echo "Both rules are already in place." ;;
  4) fail "refusing to touch the lifecycle configuration (see above). Fix that rule so it cannot reach ${OUT_PREFIX}, then re-run." ;;
  0) echo "$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1])).get("Rules", [])))' "$TMP/lifecycle.live.json") existing rule(s) will be kept; storyvideo-work-expiry (7 days, noncurrent 1 day) and storyvideo-work-delete-markers will be added." ;;
  *) fail "the lifecycle merge failed (exit ${LC_RC})." ;;
esac

# ---------------------------------------------------------------------------
echo
echo "=== 4. Image ==="
if [ -n "${STORY_RENDER_TAG:-}" ]; then
  IMAGE_TAG="$STORY_RENDER_TAG"
else
  IMAGE_TAG=$(git -C "$REPO_ROOT" rev-parse --short=12 HEAD) || fail "could not read the git commit for the image tag. Set STORY_RENDER_TAG."
fi
IMAGE="${REGISTRY}/${REPO}:${IMAGE_TAG}"

IMAGE_PRESENT=false
if aws ecr describe-images --repository-name "$REPO" --image-ids "imageTag=${IMAGE_TAG}" --region "$REGION" >/dev/null 2>&1; then
  IMAGE_PRESENT=true
  echo "Image ${REPO}:${IMAGE_TAG} is already pushed."
else
  echo "Image ${REPO}:${IMAGE_TAG} is not pushed yet; it will be built from backend/ with docker."
  DIRTY=$(git -C "$REPO_ROOT" status --porcelain -- backend 2>/dev/null || true)
  if [ -n "$DIRTY" ] && [ -z "${STORY_RENDER_TAG:-}" ]; then
    if $APPLY && [ -z "${ALLOW_DIRTY:-}" ]; then
      echo "$DIRTY" | head -10 >&2
      fail "backend/ has uncommitted changes, so the image would not be what commit ${IMAGE_TAG} contains. Commit them, or set ALLOW_DIRTY=1 (or STORY_RENDER_TAG) to build anyway."
    fi
    echo "WARNING: backend/ has uncommitted changes; the image would not match commit ${IMAGE_TAG}."
  fi
  if $APPLY; then
    command -v docker >/dev/null 2>&1 || fail "docker is needed to build the image and was not found."
  fi
fi

# ---------------------------------------------------------------------------
echo
echo "=== 5. Task definition ${FAMILY} ==="
IMAGE="$IMAGE" CONTAINER="$CONTAINER" BUCKET="$BUCKET" TABLE="$TABLE" REGION="$REGION" LOG_GROUP="$LOG_GROUP" \
  SSM_PREFIX="$SSM_PREFIX" ACCOUNT_ID="$ACCOUNT_ID" SECRET_NAMES="$SECRET_NAMES" \
  python3 "$TMP/helpers.py" containers "$TMP/containers.json"

TASKDEF_UPTODATE=false
if aws ecs describe-task-definition --task-definition "$FAMILY" --region "$REGION" --output json >"$TMP/taskdef.live.json" 2>/dev/null &&
  python3 "$TMP/helpers.py" taskdef "$TMP/taskdef.live.json" "$TMP/containers.json" "$CPU" "$MEMORY" "$EXEC_ROLE_ARN" "$TASK_ROLE_ARN"; then
  TASKDEF_UPTODATE=true
  echo "The latest revision already is this: ${IMAGE##*/}, ${CPU} CPU / ${MEMORY} MB."
else
  echo "A new revision will be registered: ${IMAGE##*/}, ${CPU} CPU / ${MEMORY} MB, Fargate x86_64, keys from SSM."
fi

# ---------------------------------------------------------------------------
echo
echo "=== 6. State machine ${SM_NAME}: render and validate ==="
(cd "$BACKEND_DIR" && go run -buildvcs=false ./cmd/storypipeline \
  -cluster-arn "$CLUSTER_ARN" -task-definition-arn "$TASKDEF_ARN" -subnets "$SUBNET_IDS" \
  -security-group "$SECURITY_GROUP_ID" -table "$TABLE" -notifier-arn "$NOTIFIER_ARN") >"$TMP/definition.json" ||
  fail "could not render the state machine definition."
STATES=$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))["States"]))' "$TMP/definition.json")
echo "Rendered ${STATES} states with backend/cmd/storypipeline."

VALID=$(aws stepfunctions validate-state-machine-definition --type STANDARD \
  --definition "file://${TMP}/definition.json" --region "$REGION" --query 'result' --output text) || fail "the validator call failed."
if [ "$VALID" != "OK" ]; then
  aws stepfunctions validate-state-machine-definition --type STANDARD \
    --definition "file://${TMP}/definition.json" --region "$REGION" --query 'diagnostics' >&2 || true
  fail "AWS rejected the rendered definition (${VALID})."
fi
echo "AWS validator: OK."

SM_ACTION=create
if aws stepfunctions describe-state-machine --state-machine-arn "$SM_ARN" --region "$REGION" --query 'definition' --output text >"$TMP/definition.live.json" 2>/dev/null; then
  if python3 "$TMP/helpers.py" same "$TMP/definition.live.json" "$TMP/definition.json"; then
    SM_ACTION=same
    echo "The live definition already is this."
  else
    SM_ACTION=update
    echo "The live definition differs; it will be updated (the old one is saved first)."
  fi
else
  echo "The state machine does not exist yet; it will be created."
fi

# ---------------------------------------------------------------------------
echo
echo "=== 7. API Lambda environment: ${API_ENV_KEY} on ${API_FUNCTION} ==="
if python3 "$TMP/helpers.py" apienv status "$TMP/api-config.json" "$API_ENV_KEY" "$API_ARN_VALUE"; then
  API_ENV_ACTION=same
  echo "${API_ENV_KEY} is already set to this state machine."
else
  (umask 077 && python3 "$TMP/helpers.py" apienv merge "$TMP/api-config.json" "$API_ENV_KEY" "$API_ARN_VALUE" "$TMP/api-env.json")
  if python3 "$TMP/helpers.py" apienv present "$TMP/api-config.json" "$API_ENV_KEY"; then
    API_ENV_ACTION=change
    echo "${API_ENV_KEY} is set to something else; it will be changed."
  else
    API_ENV_ACTION=add
    echo "${API_ENV_KEY} is not set; it will be added."
  fi
fi

# ===========================================================================
# PHASE 2: CHANGE. Nothing below runs a command in a dry run: act prints the
# plan instead. Order: what the others depend on first (keys, roles), the state
# machine last.
# ===========================================================================
echo
echo "=========================================================="
if $APPLY; then echo "CHANGES"; else echo "PLAN (nothing below is done in a dry run)"; fi
echo "=========================================================="

# --- stock keys
echo
echo "--- Stock API keys"
if [ "${#SECRETS_TO_CREATE[@]}" -gt 0 ]; then
  for name in "${SECRETS_TO_CREATE[@]}"; do
    act "create SecureString ${SSM_PREFIX}/${name} from ${ENV_FILE}" put_secret "$name"
  done
else
  echo "Nothing to create."
fi

# --- IAM
echo
echo "--- IAM"
IAM_CHANGED=false

ensure_role() { # name description
  if aws iam get-role --role-name "$1" >/dev/null 2>&1; then
    echo "Role $1 exists."
  else
    act "create role $1" aws iam create-role --role-name "$1" \
      --assume-role-policy-document "file://${TMP}/trust.json" --description "$2" --query 'Role.Arn' --output text
    IAM_CHANGED=true
  fi
}

# put_policy ROLE POLICY FILE WHAT: writes the inline policy unless the live one is already identical.
put_policy() {
  if aws iam get-role-policy --role-name "$1" --policy-name "$2" --query PolicyDocument --output json >"$TMP/live-policy.json" 2>/dev/null &&
    python3 "$TMP/helpers.py" same "$TMP/live-policy.json" "$3"; then
    echo "Policy $2 on $1 is up to date."
  else
    act "put inline policy $2 on $1 ($4)" aws iam put-role-policy --role-name "$1" --policy-name "$2" --policy-document "file://$3"
    IAM_CHANGED=true
  fi
}

ensure_role "$EXEC_ROLE_NAME" "Story video task: pull the image, write logs, read the stock keys"
ATTACHED=$(aws iam list-attached-role-policies --role-name "$EXEC_ROLE_NAME" \
  --query "AttachedPolicies[?PolicyName=='AmazonECSTaskExecutionRolePolicy'].PolicyName | [0]" --output text 2>/dev/null || true)
if [ "$ATTACHED" = "AmazonECSTaskExecutionRolePolicy" ]; then
  echo "AmazonECSTaskExecutionRolePolicy is attached."
else
  act "attach AmazonECSTaskExecutionRolePolicy to ${EXEC_ROLE_NAME}" \
    aws iam attach-role-policy --role-name "$EXEC_ROLE_NAME" --policy-arn arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy
  IAM_CHANGED=true
fi
put_policy "$EXEC_ROLE_NAME" "${PROJECT}-story-render-secrets" "$TMP/exec-policy.json" "read only the stock key parameters"

ensure_role "$TASK_ROLE_NAME" "Story video task: its own files, its job record, and Polly"
put_policy "$TASK_ROLE_NAME" "${PROJECT}-story-render-permissions" "$TMP/task-policy.json" "S3 on two prefixes, DynamoDB get/update on one table, Polly"

put_policy "$SFN_ROLE_NAME" "${PROJECT}-story-sfn-permissions" "$TMP/sfn-policy.json" "pass the two roles, DynamoDB get/update on the table"

put_policy "$API_ROLE_NAME" "${PROJECT}-story-api-permissions" "$TMP/api-policy.json" "stop executions of the story state machine"

if $APPLY && $IAM_CHANGED; then
  echo "Waiting for IAM to propagate (10s)..."
  sleep 10
fi

# --- S3 lifecycle
echo
echo "--- S3 lifecycle"
if [ "$LC_RC" -eq 0 ]; then
  LC_BEFORE="/tmp/${BUCKET}-lifecycle-before-${TIMESTAMP}.json"
  MINSIZE=$(cat "$TMP/lifecycle.minsize")
  put_lifecycle() {
    if [ -f "$TMP/lifecycle.before.json" ]; then
      cp "$TMP/lifecycle.before.json" "$LC_BEFORE"
      echo ">> saved the previous rules to ${LC_BEFORE}"
      echo "   undo: aws s3api put-bucket-lifecycle-configuration --bucket ${BUCKET} --lifecycle-configuration file://${LC_BEFORE}"
    else
      echo "   there was no lifecycle configuration before; undo: aws s3api delete-bucket-lifecycle --bucket ${BUCKET}"
    fi
    aws s3api put-bucket-lifecycle-configuration --bucket "$BUCKET" --region "$REGION" \
      --lifecycle-configuration "file://${TMP}/lifecycle.new.json" \
      ${MINSIZE:+--transition-default-minimum-object-size "$MINSIZE"}
  }
  act "merge the two rules into the lifecycle configuration of ${BUCKET}" put_lifecycle
else
  echo "Already in place."
fi

# --- ECR
echo
echo "--- ECR repository and image"
if aws ecr describe-repositories --repository-names "$REPO" --region "$REGION" >/dev/null 2>&1; then
  echo "Repository ${REPO} exists."
else
  act "create ECR repository ${REPO} (immutable tags, scan on push)" \
    aws ecr create-repository --repository-name "$REPO" --image-tag-mutability IMMUTABLE \
    --image-scanning-configuration scanOnPush=true --region "$REGION" --query 'repository.repositoryUri' --output text
fi
cat >"$TMP/ecr-lifecycle.json" <<EOF
{"rules":[{"rulePriority":1,"description":"keep the 10 newest images","selection":{"tagStatus":"any","countType":"imageCountMoreThan","countNumber":10},"action":{"type":"expire"}}]}
EOF
if aws ecr get-lifecycle-policy --repository-name "$REPO" --region "$REGION" --query lifecyclePolicyText --output text >"$TMP/ecr-lifecycle.live.json" 2>/dev/null &&
  python3 "$TMP/helpers.py" same "$TMP/ecr-lifecycle.live.json" "$TMP/ecr-lifecycle.json"; then
  echo "The image retention policy (10 newest) is in place."
else
  act "keep only the 10 newest images in ${REPO}" \
    aws ecr put-lifecycle-policy --repository-name "$REPO" --lifecycle-policy-text "file://${TMP}/ecr-lifecycle.json" \
    --region "$REGION" --query 'repositoryName' --output text
fi

build_and_push() {
  aws ecr get-login-password --region "$REGION" | docker login --username AWS --password-stdin "$REGISTRY"
  # Fargate runs this task on x86_64; --platform keeps an ARM laptop from building the wrong image.
  docker build --platform linux/amd64 -f "${BACKEND_DIR}/cmd/storyrender/Dockerfile" -t "$IMAGE" "$BACKEND_DIR"
  docker push "$IMAGE"
}
if $IMAGE_PRESENT; then
  echo "Image ${REPO}:${IMAGE_TAG} is already pushed."
else
  act "build and push ${IMAGE}" build_and_push
fi

# --- log group
echo
echo "--- Log group ${LOG_GROUP}"
LG=$(aws logs describe-log-groups --log-group-name-prefix "$LOG_GROUP" --region "$REGION" \
  --query "logGroups[?logGroupName=='${LOG_GROUP}'].logGroupName | [0]" --output text 2>/dev/null || true)
if [ "$LG" = "$LOG_GROUP" ]; then
  echo "Log group exists."
else
  act "create log group ${LOG_GROUP}" aws logs create-log-group --log-group-name "$LOG_GROUP" --region "$REGION"
fi
RETENTION=$(aws logs describe-log-groups --log-group-name-prefix "$LOG_GROUP" --region "$REGION" \
  --query "logGroups[?logGroupName=='${LOG_GROUP}'].retentionInDays | [0]" --output text 2>/dev/null || true)
if [ "$RETENTION" = "30" ]; then
  echo "Logs are kept for 30 days."
else
  act "keep the task's logs for 30 days" \
    aws logs put-retention-policy --log-group-name "$LOG_GROUP" --retention-in-days 30 --region "$REGION"
fi

# --- task definition
echo
echo "--- Task definition ${FAMILY}"
register_taskdef() {
  aws ecs register-task-definition --family "$FAMILY" \
    --requires-compatibilities FARGATE --network-mode awsvpc \
    --runtime-platform cpuArchitecture=X86_64,operatingSystemFamily=LINUX \
    --cpu "$CPU" --memory "$MEMORY" \
    --execution-role-arn "$EXEC_ROLE_ARN" --task-role-arn "$TASK_ROLE_ARN" \
    --container-definitions "file://${TMP}/containers.json" \
    --region "$REGION" --query 'taskDefinition.taskDefinitionArn' --output text
}
if $TASKDEF_UPTODATE; then
  echo "Up to date."
else
  act "register a task definition revision (${IMAGE##*/}, ${CPU} CPU / ${MEMORY} MB)" register_taskdef
fi

# --- state machine
echo
echo "--- State machine ${SM_NAME}"
BACKUP="/tmp/${SM_NAME}-before-${TIMESTAMP}.json"
update_machine() {
  cp "$TMP/definition.live.json" "$BACKUP"
  echo ">> saved the previous definition to ${BACKUP}"
  echo "   undo: aws stepfunctions update-state-machine --state-machine-arn ${SM_ARN} --definition file://${BACKUP}"
  aws stepfunctions update-state-machine --state-machine-arn "$SM_ARN" --role-arn "$SFN_ROLE_ARN" \
    --definition "file://${TMP}/definition.json" --region "$REGION" --query 'updateDate' --output text
}
case "$SM_ACTION" in
  same) echo "Up to date." ;;
  update) act "update ${SM_NAME} (running executions finish on the definition they started with)" update_machine ;;
  create)
    act "create state machine ${SM_NAME} (STANDARD, role ${SFN_ROLE_NAME})" \
      aws stepfunctions create-state-machine --name "$SM_NAME" --type STANDARD --role-arn "$SFN_ROLE_ARN" \
      --definition "file://${TMP}/definition.json" --region "$REGION" --query 'stateMachineArn' --output text
    ;;
esac

# --- API Lambda environment (last: the state machine it names exists by now)
echo
echo "--- API Lambda environment ${API_ENV_KEY}"
if [ "$API_ENV_ACTION" = same ]; then
  echo "Up to date."
else
  act "set ${API_ENV_KEY} on ${API_FUNCTION} (every other variable is written back as it was; undo: bash deploy/setup-storyvideo.sh --unwire-api --apply)" write_api_env
fi

# ---------------------------------------------------------------------------
echo
if $APPLY; then
  echo "Done."
else
  echo "Dry run complete. Re-run with --apply to make these changes."
fi
cat <<EOF

State machine: ${SM_ARN}
Task definition family: ${FAMILY}   Image: ${IMAGE}

Still to do by hand: deploy the API code. The routes are in the code, and the
environment variable above only switches the create route on once they are live:
  cd backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bootstrap ./cmd/api
  (cd /tmp && zip -j api.zip bootstrap)
  aws lambda update-function-code --function-name ${API_FUNCTION} --zip-file fileb:///tmp/api.zip --region ${REGION}
(code only; this does not touch the environment variables). To switch the create
route off again: bash deploy/setup-storyvideo.sh --unwire-api --apply

Nothing has run on Fargate yet: the first real job will show the real render time.
The 'storyjob_timing' line in /ecs/${PROJECT}-story-render has the numbers. The worker's own
deadline (15 minutes) and the state machine's (20) were chosen before any measurement.
EOF
