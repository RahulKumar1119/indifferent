#!/bin/bash
set -euo pipefail
# Story2Shorts success-metrics dashboard (issue #194)
# ===================================================
# The metrics themselves need nothing deployed beyond the code: the API and the
# notifier write CloudWatch embedded-metric lines to their logs, and CloudWatch
# turns them into metrics in the namespace Indifferent/Story (backend/internal/
# storymetrics). This script creates the one thing that is not code, the
# dashboard that shows them: one CloudWatch dashboard, indifferent-fun-story.
#
#   1. Render   the dashboard body with backend/cmd/storydashboard, the same code
#               the tests check against the metrics the services emit
#   2. Compare  with the dashboard that exists (a dry run says whether it would change)
#   3. Write    the dashboard, after saving the one that was there. PutDashboard
#               replaces a dashboard whole, so its previous body goes to
#               /tmp/<name>-before-<timestamp>.json first, with the command to restore it
#   4. Alarm    indifferent-fun-story-plans-failing: 4 or more failed plans in 15
#               minutes, and at least as many failed as succeeded. It e-mails the
#               alerts topic the API alarms use (indifferent-fun-email-alerts)
#
# The dashboard is global to the account, not to a region; its widgets read the
# metrics of ${REGION}.
#
# What it shows (the text on the dashboard says the same):
#   1. Story to video      finished videos / stories submitted
#   2. Generation time     average, slowest 10%, slowest
#   3. Stock coverage      searches that found something / searches
#   4. Render failure rate failed / (finished + failed); a cancel is not a failure
#   5. Replacement rate    pictures swapped or removed / pictures chosen
# plus the counts behind them and the failures by reason.
#
# Metrics appear only once the code that writes them is deployed AND something
# happens: the API (for plans, edits, searches, starts, cancels) and the notifier
# (for videos finishing and failing). Deploy order:
#   1. deploy the API code and the notifier code (see the end of this output)
#   2. bash deploy/setup-story-metrics.sh --apply     (the dashboard may come first;
#      its graphs are empty until the first event)
#
# Usage:
#   bash deploy/setup-story-metrics.sh            # dry run: read-only, prints the plan
#   bash deploy/setup-story-metrics.sh --apply    # writes the dashboard and the alarm
#
# Safety:
#   - The dry run only reads (get-dashboard) and renders locally.
#   - The only writes are put-dashboard, for this one dashboard name, and
#     put-metric-alarm, for this one alarm name. Nothing else in the account is
#     touched, and nothing is deleted.
#   - Idempotent: when the dashboard and the alarm already are what would be
#     written, nothing is written.
#   - The alarm needs the alerts topic to exist; without it the script stops
#     before writing the alarm.
#   - Errors in the dashboard stop the script; warnings (elements that may not
#     render) are printed.
#   - Each custom metric costs CloudWatch's per-metric monthly charge (check the
#     current CloudWatch pricing page), and a dashboard has its own charge after
#     the free ones. This creates about a dozen metrics plus one per failure
#     reason that actually occurs (at most ten).
#   - Needs: aws, go, python3.
#
# Undo:
#   aws cloudwatch delete-dashboards --dashboard-names indifferent-fun-story --region ap-south-1
#   (the metrics already recorded stay; they expire on their own, after 15 months)
#   or restore the previous body from the file this script prints.
#   aws cloudwatch delete-alarms --alarm-names indifferent-fun-story-plans-failing --region ap-south-1

APPLY=false
for arg in "$@"; do
  case "$arg" in
    "") ;;
    --apply) APPLY=true ;;
    *)
      echo "Unknown argument: $arg (use --apply, or no argument)" >&2
      exit 1
      ;;
  esac
done

REGION="ap-south-1"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKEND_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)/backend"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

if $APPLY; then
  echo "MODE: apply (the dashboard and the alarm will be written)"
else
  echo "MODE: dry run (read-only; nothing will be changed)"
fi
echo

# --- 1. render. The name and the body come from the Go code, which prints nothing else on
# stdout. That the name is one CloudWatch accepts is checked by a Go test
# (TestTheDashboardNameIsOneCloudWatchAccepts), not here.
DASHBOARD=$(cd "$BACKEND_DIR" && go run -buildvcs=false ./cmd/storydashboard -name) ||
  fail "could not get the dashboard's name from backend/cmd/storydashboard."
(cd "$BACKEND_DIR" && go run -buildvcs=false ./cmd/storydashboard -region "$REGION") >"$TMP/new.json" ||
  fail "could not render the dashboard (backend/cmd/storydashboard)."
WIDGETS=$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))["widgets"]))' "$TMP/new.json") ||
  fail "the rendered dashboard is not valid JSON."
echo "Rendered ${DASHBOARD}: ${WIDGETS} widgets, ${REGION}."

# --- 2. compare with what is there
STATE=create
if aws cloudwatch get-dashboard --dashboard-name "$DASHBOARD" --region "$REGION" --query 'DashboardBody' --output text >"$TMP/live.json" 2>"$TMP/live.err"; then
  if python3 - "$TMP/live.json" "$TMP/new.json" <<'PY'
import json, sys
live = json.load(open(sys.argv[1]))
new = json.load(open(sys.argv[2]))
sys.exit(0 if live == new else 1)
PY
  then
    STATE=same
  else
    STATE=update
  fi
elif grep -q 'ResourceNotFound\|does not exist\|not found' "$TMP/live.err"; then
  STATE=create
else
  fail "could not read the dashboard ${DASHBOARD}: $(tail -n 1 "$TMP/live.err")"
fi

case "$STATE" in
  same) echo "The dashboard is already up to date." ;;
  create) echo "The dashboard does not exist; it will be created." ;;
  update) echo "The dashboard exists and differs; it will be replaced (the current body is saved first)." ;;
esac

# The alarm (step 4) e-mails the alerts topic. Check it is there before writing
# anything, so a missing topic never leaves the dashboard written and the alarm not.
ALARM="indifferent-fun-story-plans-failing"
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text) ||
  fail "could not read the account id (aws sts get-caller-identity)."
TOPIC_ARN="arn:aws:sns:${REGION}:${ACCOUNT_ID}:indifferent-fun-email-alerts"
aws sns get-topic-attributes --topic-arn "$TOPIC_ARN" --region "$REGION" --query 'Attributes.TopicArn' --output text >/dev/null 2>"$TMP/topic.err" ||
  fail "the alerts topic ${TOPIC_ARN} was not found. Run deploy/setup-data-protection.sh first."

# --- 3. write
put_dashboard() {
  if [ "$STATE" = update ]; then
    BACKUP="/tmp/${DASHBOARD}-before-${TIMESTAMP}.json"
    cp "$TMP/live.json" "$BACKUP"
    echo ">> saved the previous dashboard body to ${BACKUP}"
    echo "   undo: aws cloudwatch put-dashboard --dashboard-name ${DASHBOARD} --dashboard-body file://${BACKUP} --region ${REGION}"
  fi
  aws cloudwatch put-dashboard --dashboard-name "$DASHBOARD" --dashboard-body "file://${TMP}/new.json" \
    --region "$REGION" --query 'DashboardValidationMessages' --output json >"$TMP/messages.json"
  python3 - "$TMP/messages.json" <<'PY'
import json, sys
msgs = json.load(open(sys.argv[1])) or []
for m in msgs:
    print("  warning: %s: %s" % (m.get("DataPath", ""), m.get("Message", "")))
print("CloudWatch accepted the dashboard%s." % (" with %d warning(s)" % len(msgs) if msgs else ""))
PY
}

if [ "$STATE" = same ]; then
  :
elif $APPLY; then
  echo ">> write the dashboard ${DASHBOARD}"
  put_dashboard
else
  echo "[dry-run] would: write the dashboard ${DASHBOARD}"
fi

# --- 4. the story-plans-failing alarm
# The API's 5xx alarm fires for any server error, and says nothing about which
# part broke. This one is about the planner: it fires when, in 15 minutes, at
# least 4 plans failed AND at least as many failed as succeeded. Over the week
# to 12 Nov it would have fired only during the planner outage of that night
# (from 01:30 UTC), and not for the few unusable plans of a normal day. It stays
# in ALARM for one quiet period after the last breach, so one outage sends one
# or two emails, not one per burst. It e-mails the same alerts topic as the
# API alarms (deploy/setup-data-protection.sh).

python3 - "$TMP/alarm.json" "$ALARM" "$TOPIC_ARN" <<'PY'
import json, sys
path, name, topic = sys.argv[1:]
# "Dimensions": [] is how describe-alarms returns a metric without dimensions;
# writing it the same way keeps the comparison below exact.
metric = lambda id_, m: {"Id": id_, "ReturnData": False, "MetricStat": {
    "Metric": {"Namespace": "Indifferent/Story", "MetricName": m, "Dimensions": []}, "Period": 900, "Stat": "Sum"}}
json.dump({
    "AlarmName": name,
    "AlarmDescription": "Story planning is failing: in 15 minutes, 4 or more plans failed and at least as many failed as succeeded. "
                        "Check the API log for storyplan_failed lines (model, attempt, outcome).",
    "ActionsEnabled": True,
    "AlarmActions": [topic],
    "Metrics": [
        metric("failed", "PlansFailed"),
        metric("ready", "PlansReady"),
        {"Id": "failing", "Label": "Story plans failing", "ReturnData": True,
         "Expression": "IF(FILL(failed,0) >= 4 AND FILL(failed,0) >= FILL(ready,0), 1, 0)"},
    ],
    "EvaluationPeriods": 2,
    "DatapointsToAlarm": 1,
    "Threshold": 1,
    "ComparisonOperator": "GreaterThanOrEqualToThreshold",
    "TreatMissingData": "notBreaching",
}, open(path, "w"), indent=1)
PY

aws cloudwatch describe-alarms --alarm-names "$ALARM" --region "$REGION" --query 'MetricAlarms[0]' --output json >"$TMP/alarm-live.json" ||
  fail "could not read the alarm ${ALARM}."
ALARM_STATE=$(python3 - "$TMP/alarm-live.json" "$TMP/alarm.json" <<'PY'
import json, sys
live = json.load(open(sys.argv[1]))
want = json.load(open(sys.argv[2]))
if not live:
    print("create")
    sys.exit()
# Compare what this script sets, as describe-alarms returns it.
def norm(a):
    ms = [{k: v for k, v in m.items() if k in ("Id", "ReturnData", "MetricStat", "Expression", "Label")} for m in a.get("Metrics", [])]
    out = {k: a.get(k) for k in want if k != "Metrics"}
    out["Metrics"] = ms
    return out
print("same" if norm(live) == norm(want) else "update")
PY
)
case "$ALARM_STATE" in
  same) echo "The alarm ${ALARM} is already up to date." ;;
  create) echo "The alarm ${ALARM} does not exist; it will be created (e-mails ${TOPIC_ARN##*:})." ;;
  update) echo "The alarm ${ALARM} exists and differs; it will be replaced." ;;
esac
if [ "$ALARM_STATE" = same ]; then
  :
elif $APPLY; then
  echo ">> put the alarm ${ALARM}"
  aws cloudwatch put-metric-alarm --cli-input-json "file://${TMP}/alarm.json" --region "$REGION"
  echo "   undo: aws cloudwatch delete-alarms --alarm-names ${ALARM} --region ${REGION}"
else
  echo "[dry-run] would: put the alarm ${ALARM}"
fi

echo
if $APPLY; then
  echo "Done."
else
  echo "Dry run complete. Re-run with --apply to make this change."
fi
cat <<EOF

The dashboard: https://${REGION}.console.aws.amazon.com/cloudwatch/home?region=${REGION}#dashboards:name=${DASHBOARD}

Still to do by hand: deploy the code that writes the metrics. Neither needs a new
setting or permission (they log to stdout, which both already send to CloudWatch Logs):
  API:       cd backend && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o /tmp/bootstrap ./cmd/api
             (cd /tmp && zip -j api.zip bootstrap)
             aws lambda update-function-code --function-name indifferent-fun-api --zip-file fileb:///tmp/api.zip --region ${REGION}
  Notifier:  bash deploy/setup-notifications.sh --apply     (it rebuilds and redeploys the notifier)

To see that a metric is arriving, look in the API's log group for lines that begin
{"_aws": and in CloudWatch under Metrics > Indifferent/Story after the first event.
If lines are in the log and no metric appears, CloudWatch rejected the format: the
namespace AWS/Logs has metrics for EMF parsing and validation failures.
EOF
