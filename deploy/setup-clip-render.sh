#!/bin/bash
set -euo pipefail
# Rendering one finished Shorts clip again, and hook suggestions (issue #329)
# ===========================================================================
#
# Creates or updates, in this order:
#
#   1. Step Functions  indifferent-fun-shorts-clip-render (backend/internal/shorts/pipeline/clip_render.json):
#                      runs the existing Shorts render task in single-clip mode, Spot first then
#                      on-demand, and marks the clip's render failed if every try fails
#   2. IAM             one inline policy on the Step Functions role (run the Shorts render task,
#                      pass its roles, update the clip render record) and one on the API Lambda role
#                      (start and stop executions of this state machine only)
#   3. API Lambda      CLIP_RENDER_STATE_MACHINE_ARN, and with --hooks-on HOOK_SUGGESTIONS_ENABLED=true
#
# It does NOT build or deploy code. Deploy in this order first:
#   a. the Shorts render image with single-clip mode, and a new task definition revision
#      (the state machine names the task family, so it uses the latest revision);
#   b. the API Lambda code.
# Setting CLIP_RENDER_STATE_MACHINE_ARN before (a) would start renders on an image that does not know
# single-clip mode: they would stop and be marked failed, leaving the clip as it was, but would
# still use one of the person's daily tries.
#
# Usage:
#   bash deploy/setup-clip-render.sh                       # dry run: read-only calls, prints the plan
#   bash deploy/setup-clip-render.sh --apply               # makes the changes
#   bash deploy/setup-clip-render.sh --apply --hooks-on    # also switches hook suggestions on
#
# Undo:
#   remove CLIP_RENDER_STATE_MACHINE_ARN (and HOOK_SUGGESTIONS_ENABLED) from the API Lambda environment;
#   the routes then answer 503. Then, if wanted:
#   aws stepfunctions delete-state-machine --state-machine-arn <arn>
#   aws iam delete-role-policy --role-name indifferent-fun-sfn-role --policy-name indifferent-fun-clip-render-sfn-permissions
#   aws iam delete-role-policy --role-name indifferent-fun-lambda-role --policy-name indifferent-fun-clip-render-lambda-permissions
REGION="ap-south-1"
PROJECT="indifferent-fun"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
TABLE="${PROJECT}-projects"
CLUSTER="${PROJECT}-shorts"
TASK_FAMILY="${PROJECT}-shorts-render"
SM_NAME="${PROJECT}-shorts-clip-render"
SM_ARN="arn:aws:states:${REGION}:${ACCOUNT_ID}:stateMachine:${SM_NAME}"
SFN_ROLE="${PROJECT}-sfn-role"
LAMBDA_ROLE="${PROJECT}-lambda-role"
API_FUNCTION="${PROJECT}-api"
SG_NAME="${PROJECT}-shorts-task-sg"
SUBNET_IDS="${SHORTS_SUBNET_IDS:-subnet-f01f6e8b,subnet-880ef0e3,subnet-7bf7d837}"
BACKEND_DIR="$(cd "$(dirname "$0")/../backend" && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
APPLY=false
HOOKS=false
for a in "$@"; do
  case "$a" in
    --apply) APPLY=true ;;
    --hooks-on) HOOKS=true ;;
    *) echo "Unknown argument: $a (use --apply and/or --hooks-on)" >&2; exit 1 ;;
  esac
done
if $APPLY; then echo "MODE: apply (account ${ACCOUNT_ID}, ${REGION})"; else echo "MODE: dry run (read-only)"; fi
echo
act() {
  local what="$1"; shift
  if $APPLY; then echo ">> ${what}"; "$@"; else echo "[dry-run] would: ${what}"; fi
}

echo "=== 0. Preflight ==="
aws ecs describe-clusters --clusters "$CLUSTER" --region "$REGION" --query 'clusters[0].status' --output text | grep -qx ACTIVE \
  || { echo "ERROR: ECS cluster ${CLUSTER} is not ACTIVE." >&2; exit 1; }
SECURITY_GROUP_ID=$(aws ec2 describe-security-groups --filters "Name=group-name,Values=${SG_NAME}" --region "$REGION" --query 'SecurityGroups[0].GroupId' --output text)
[ -n "$SECURITY_GROUP_ID" ] && [ "$SECURITY_GROUP_ID" != "None" ] || { echo "ERROR: security group ${SG_NAME} not found." >&2; exit 1; }
TD=$(aws ecs describe-task-definition --task-definition "$TASK_FAMILY" --region "$REGION" --query 'taskDefinition' --output json)
TASK_ROLE=$(echo "$TD" | python3 -c 'import json,sys; print(json.load(sys.stdin)["taskRoleArn"])')
EXEC_ROLE=$(echo "$TD" | python3 -c 'import json,sys; print(json.load(sys.stdin)["executionRoleArn"])')
IMAGE=$(echo "$TD" | python3 -c 'import json,sys; print(json.load(sys.stdin)["containerDefinitions"][0]["image"])')
echo "cluster ${CLUSTER}, security group ${SECURITY_GROUP_ID}, task ${TASK_FAMILY} image ${IMAGE}"
echo "  check that this image has single-clip mode (#329) before switching the route on"
echo

echo "=== 1. Step Functions ==="
SUBNET_JSON="[$(echo "$SUBNET_IDS" | awk -F, '{for(i=1;i<=NF;i++){printf (i>1?",":"") "\"" $i "\""}}')]"
sed \
  -e "s|\${EcsClusterArn}|arn:aws:ecs:${REGION}:${ACCOUNT_ID}:cluster/${CLUSTER}|g" \
  -e "s|\${ShortsRenderTaskDefArn}|arn:aws:ecs:${REGION}:${ACCOUNT_ID}:task-definition/${TASK_FAMILY}|g" \
  -e "s|\[\"\${SubnetIds}\"\]|${SUBNET_JSON}|g" \
  -e "s|\${SecurityGroupId}|${SECURITY_GROUP_ID}|g" \
  -e "s|\${DynamoTable}|${TABLE}|g" \
  "${BACKEND_DIR}/internal/shorts/pipeline/clip_render.json" > "$TMP/sm.json"
if grep -q '\${' "$TMP/sm.json"; then echo "ERROR: unreplaced placeholders." >&2; exit 1; fi
RESULT=$(aws stepfunctions validate-state-machine-definition --type STANDARD --definition "file://${TMP}/sm.json" --region "$REGION" --query 'result' --output text)
[ "$RESULT" = "OK" ] || { echo "ERROR: definition did not validate (${RESULT})." >&2; exit 1; }
echo "state machine definition validates"
SFN_ROLE_ARN="arn:aws:iam::${ACCOUNT_ID}:role/${SFN_ROLE}"

echo
echo "=== 2. IAM ==="
cat > "$TMP/sfn-policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {"Effect": "Allow", "Action": ["ecs:RunTask"], "Resource": "arn:aws:ecs:${REGION}:${ACCOUNT_ID}:task-definition/${TASK_FAMILY}:*"},
    {"Effect": "Allow", "Action": ["ecs:StopTask", "ecs:DescribeTasks"], "Resource": "*"},
    {"Effect": "Allow", "Action": ["iam:PassRole"], "Resource": ["${TASK_ROLE}", "${EXEC_ROLE}"]},
    {"Effect": "Allow", "Action": ["events:PutTargets", "events:PutRule", "events:DescribeRule"],
     "Resource": "arn:aws:events:${REGION}:${ACCOUNT_ID}:rule/StepFunctionsGetEventsForECSTaskRule"},
    {"Effect": "Allow", "Action": ["dynamodb:UpdateItem"], "Resource": "arn:aws:dynamodb:${REGION}:${ACCOUNT_ID}:table/${TABLE}"}
  ]
}
EOF
act "add permissions to ${SFN_ROLE}" aws iam put-role-policy --role-name "$SFN_ROLE" \
  --policy-name "${PROJECT}-clip-render-sfn-permissions" --policy-document "file://${TMP}/sfn-policy.json"
cat > "$TMP/lambda-policy.json" <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {"Effect": "Allow", "Action": ["states:StartExecution"], "Resource": "${SM_ARN}"},
    {"Effect": "Allow", "Action": ["states:StopExecution"], "Resource": "arn:aws:states:${REGION}:${ACCOUNT_ID}:execution:${SM_NAME}:*"}
  ]
}
EOF
act "allow ${LAMBDA_ROLE} to start and stop ${SM_NAME}" aws iam put-role-policy --role-name "$LAMBDA_ROLE" \
  --policy-name "${PROJECT}-clip-render-lambda-permissions" --policy-document "file://${TMP}/lambda-policy.json"
if $APPLY; then sleep 10; fi
if aws stepfunctions describe-state-machine --state-machine-arn "$SM_ARN" --region "$REGION" >/dev/null 2>&1; then
  act "update state machine ${SM_NAME}" aws stepfunctions update-state-machine --state-machine-arn "$SM_ARN" \
    --definition "file://${TMP}/sm.json" --role-arn "$SFN_ROLE_ARN" --region "$REGION" --query updateDate --output text
else
  act "create state machine ${SM_NAME}" aws stepfunctions create-state-machine --name "$SM_NAME" \
    --definition "file://${TMP}/sm.json" --role-arn "$SFN_ROLE_ARN" --type STANDARD --region "$REGION" --query stateMachineArn --output text
fi

echo
echo "=== 3. API Lambda settings ==="
if $APPLY; then
  CURRENT_ENV=$(aws lambda get-function-configuration --function-name "$API_FUNCTION" --region "$REGION" --query 'Environment.Variables' --output json)
  echo "$CURRENT_ENV" | python3 -c "
import json, sys
raw = sys.stdin.read().strip()
env = json.loads(raw) if raw and raw != 'null' else {}
env['CLIP_RENDER_STATE_MACHINE_ARN'] = sys.argv[1]
if sys.argv[2] == 'true':
    env['HOOK_SUGGESTIONS_ENABLED'] = 'true'
json.dump({'Variables': env}, open(sys.argv[3], 'w'))
" "$SM_ARN" "$HOOKS" "$TMP/api-env.json"
  aws lambda update-function-configuration --function-name "$API_FUNCTION" --region "$REGION" --environment "file://${TMP}/api-env.json" >/dev/null
  aws lambda wait function-updated --function-name "$API_FUNCTION" --region "$REGION"
  echo ">> set CLIP_RENDER_STATE_MACHINE_ARN${HOOKS:+ (hooks: $HOOKS)}"
else
  echo "[dry-run] would: set CLIP_RENDER_STATE_MACHINE_ARN=${SM_ARN} on ${API_FUNCTION}"
  $HOOKS && echo "[dry-run] would: set HOOK_SUGGESTIONS_ENABLED=true on ${API_FUNCTION}"
fi
echo
if $APPLY; then echo "Done."; else echo "Dry run complete. Re-run with --apply to make these changes."; fi
