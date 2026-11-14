#!/usr/bin/env python3
"""Story plan to video funnel, for plans created in a date range. Read-only.

Prints counts only: no id, email, name, story or search word (#305, part of
#288). It reads three things and changes nothing:
  - users table:    which accounts are real (@example.com accounts are tests)
  - projects table: STORYPLAN# items (status, revision, how many scenes have a
                    picture, how many the plan picked itself) and STORYVIDEO#
                    items (which plan, status, error code)
  - API log group:  the story_video_start lines (user, plan, outcome, code),
                    which show a "Make the video" click that made no job.
The plan JSON is read in memory to count scenes and pictures, and dropped.

Who is counted: every real user with at least one READY plan created in
[--since, --until). "Started a video" means a job was made for one of those
plans, which is what the 11 Nov baseline called "made a video". Each user lands
in exactly one group, in this order:
  a video finished         a video from one of those plans finished
  failed                   a job failed (not cancelled), none finished
  cancelled                every job was cancelled by the user
  still being made         a job is queued or running
  clicked and refused      a logged start was refused or errored, no job made
  never clicked, ready     every scene has a picture, nothing was started
  never clicked, blocked   some scene has no picture (split: none / some)
A user's "best" plan is the one with the most pictured scenes (the latest on a
tie); the ready/blocked split uses that plan as it is stored now.

The baseline (80 users, 24 started, of 56 who did not: 41 none, 5 some, 10 all)
is reproduced by --until 2026-11-11T15:10 (UTC) on the live tables.

Limits worth knowing:
  - Start attempts exist only from when #301 was deployed. Before that, "never
    clicked" and "clicked and refused" cannot be told apart, and the script
    says so. Lines older than the log group's retention are gone.
  - A plan is read as it is stored NOW. Pictures chosen after --until still
    show. --as-of ignores videos started at or after that time, so a past
    window can be compared as it stood then (default: now).
  - "Swapped or removed" is auto-picked pictures minus those still marked auto,
    so it cannot tell a swap from a removal.

Usage:
  scripts/ops/story_funnel.py --until 2026-11-11T15:10             # the 11 Nov baseline
  scripts/ops/story_funnel.py --since 2026-11-20 --until 2026-11-27  # a week after release
  scripts/ops/story_funnel.py --since 2026-11-20 --json
"""
import argparse
import collections
import datetime
import json
import re
import sys

TEST_DOMAIN = "@example.com"

# The funnel groups, in the order a user is placed in them. A user who STARTED a
# video (a job was made) is what the 11 Nov baseline called "made a video".
STARTED = [
    ("made_video", "a video finished"),
    ("started_failed", "failed (not cancelled)"),
    ("started_cancelled", "cancelled by the user"),
    ("in_progress", "still being made"),
]
NOT_STARTED = [
    ("clicked_refused", "clicked and refused (logged, no job)"),
    ("never_clicked_ready", "never clicked, every scene pictured"),
    ("blocked_some", "never clicked, some scenes pictured"),
    ("blocked_none", "never clicked, no scene pictured"),
]
GROUPS = STARTED + NOT_STARTED

START_LINE = re.compile(r"story_video_start user=(\S+) plan=(\S+) outcome=(\S+) code=(\S+)")


# ---------------------------------------------------------------------------
# Reading (the only part that touches AWS)

def scan(db, table, **kwargs):
    items = []
    while True:
        out = db.scan(TableName=table, **kwargs)
        items += out.get("Items", [])
        if "LastEvaluatedKey" not in out:
            return items
        kwargs["ExclusiveStartKey"] = out["LastEvaluatedKey"]


def s(item, name):
    return item.get(name, {}).get("S", "")


def n(item, name):
    try:
        return int(item.get(name, {}).get("N", "0"))
    except ValueError:
        return 0


def read_real_users(db, table):
    """Ids of accounts that are not test accounts."""
    real = set()
    for it in scan(
        db, table,
        FilterExpression="begins_with(PK, :u) AND SK = :p AND attribute_exists(email)",
        ExpressionAttributeValues={":u": {"S": "USER#"}, ":p": {"S": "PROFILE"}},
        ProjectionExpression="PK, email",
    ):
        if not s(it, "email").lower().endswith(TEST_DOMAIN):
            real.add(s(it, "PK")[len("USER#"):])
    return real


def read_plans(db, table):
    """One summary per ready plan: counts only, the plan text is dropped here."""
    plans = []
    for it in scan(
        db, table,
        FilterExpression="begins_with(SK, :p) AND #s = :ready",
        # status and plan are reserved words in DynamoDB.
        ExpressionAttributeNames={"#s": "status", "#pl": "plan"},
        ExpressionAttributeValues={":p": {"S": "STORYPLAN#"}, ":ready": {"S": "ready"}},
        ProjectionExpression="PK, SK, #s, revision, #pl, autoPick, createdAt",
    ):
        scenes = pictured = auto_left = 0
        try:
            raw = json.loads(s(it, "plan") or "{}")
        except ValueError:
            raw = {}
        for sc in raw.get("scenes") or []:
            scenes += 1
            if sc.get("media"):
                pictured += 1
                if sc.get("mediaSource") == "auto":
                    auto_left += 1
        picked = 0
        try:
            picked = int(json.loads(s(it, "autoPick") or "{}").get("picked", 0))
        except (ValueError, TypeError, AttributeError):
            pass
        plans.append({
            "user": s(it, "PK")[len("USER#"):],
            "plan": s(it, "SK")[len("STORYPLAN#"):],
            "created": s(it, "createdAt"),
            "revision": n(it, "revision"),
            "scenes": scenes,
            "pictured": pictured,
            "auto_picked": picked,
            "auto_left": auto_left,
        })
    return plans


def read_videos(db, table):
    videos = []
    for it in scan(
        db, table,
        FilterExpression="begins_with(SK, :p)",
        ExpressionAttributeNames={"#s": "status"},
        ExpressionAttributeValues={":p": {"S": "STORYVIDEO#"}},
        ProjectionExpression="PK, planId, #s, errorCode, createdAt",
    ):
        videos.append({
            "user": s(it, "PK")[len("USER#"):],
            "plan": s(it, "planId"),
            "status": s(it, "status"),
            "code": s(it, "errorCode"),
            "created": s(it, "createdAt"),
        })
    return videos


def parse_start_line(message):
    m = START_LINE.search(message)
    if not m:
        return None
    return {"user": m.group(1), "plan": m.group(2), "outcome": m.group(3), "code": m.group(4)}


def read_attempts(logs, group, since_ms, until_ms):
    """Logged "Make the video" requests in the range, and the earliest line seen (ms)."""
    attempts, earliest, token = [], None, None
    while True:
        kw = dict(logGroupName=group, filterPattern='"story_video_start user="', startTime=since_ms, endTime=until_ms)
        if token:
            kw["nextToken"] = token
        out = logs.filter_log_events(**kw)
        for ev in out.get("events", []):
            a = parse_start_line(ev.get("message", ""))
            if a:
                a["time_ms"] = ev["timestamp"]
                attempts.append(a)
                earliest = ev["timestamp"] if earliest is None else min(earliest, ev["timestamp"])
        token = out.get("nextToken")
        if not token:
            return attempts, earliest


# ---------------------------------------------------------------------------
# Counting (pure: the tests call this)

def in_range(created, since, until):
    return bool(created) and (since is None or created >= since) and (until is None or created < until)


def analyse(plans, videos, attempts, real_users, since=None, until=None, as_of=None, attempts_visible=True):
    """Counts for plans created in [since, until). Dates are ISO strings.

    attempts_visible says whether start attempts were logged for the whole
    window; when False, "never clicked" is an upper bound and is reported so.
    """
    window = [p for p in plans if p["user"] in real_users and in_range(p["created"], since, until)]
    by_user = collections.defaultdict(list)
    for p in window:
        by_user[p["user"]].append(p)

    plan_ids = {p["plan"] for p in window}
    vids = [v for v in videos if v["plan"] in plan_ids and (as_of is None or v["created"] < as_of)]
    vids_by_user = collections.defaultdict(list)
    for v in vids:
        vids_by_user[v["user"]].append(v)
    tries_by_user = collections.defaultdict(list)
    for a in attempts:
        if a["plan"] in plan_ids:
            tries_by_user[a["user"]].append(a)

    groups = collections.Counter()
    for user, ps in by_user.items():
        best = max(ps, key=lambda p: (p["pictured"], p["created"]))
        vs = vids_by_user.get(user, [])
        statuses = {v["status"] for v in vs}
        # The user's own cancel is not a failure of ours.
        failed = [v for v in vs if v["status"] == "failed" and v["code"] != "CANCELLED"]
        if "completed" in statuses:
            groups["made_video"] += 1
        elif failed:
            groups["started_failed"] += 1
        elif vs and all(v["status"] == "failed" and v["code"] == "CANCELLED" for v in vs):
            groups["started_cancelled"] += 1
        elif vs:
            groups["in_progress"] += 1
        elif any(a["outcome"] in ("refused", "error") for a in tries_by_user.get(user, [])):
            groups["clicked_refused"] += 1
        elif best["scenes"] > 0 and best["pictured"] == best["scenes"]:
            groups["never_clicked_ready"] += 1
        elif best["pictured"] > 0:
            groups["blocked_some"] += 1
        else:
            groups["blocked_none"] += 1

    users = len(by_user)
    started_any = sum(1 for u in by_user if vids_by_user.get(u))
    not_started = [u for u in by_user if not vids_by_user.get(u)]
    plan_by_id = {p["plan"]: p for p in window}
    video_plans = {v["plan"] for v in vids}
    full_video_plans = sum(1 for pid in video_plans if plan_by_id[pid]["scenes"] > 0 and plan_by_id[pid]["pictured"] == plan_by_id[pid]["scenes"])
    untouched = [p for p in window if p["revision"] <= 1]

    with_auto = [p for p in window if p["auto_picked"] > 0 or p["auto_left"] > 0]
    picked = sum(p["auto_picked"] for p in window)
    scenes_in_auto_plans = sum(p["scenes"] for p in with_auto)
    left = sum(p["auto_left"] for p in window)

    in_window = [a for a in attempts if a["plan"] in plan_ids]
    refused = collections.Counter(a["code"] for a in in_window if a["outcome"] != "started")
    return {
        "window": {"since": since, "until": until, "as_of": as_of},
        "users_with_ready_plan": users,
        "plans_ready": len(window),
        "users_who_started_a_video": started_any,
        "started_share": round(100.0 * started_any / users, 1) if users else None,
        "users_who_never_started": len(not_started),
        "never_started_with_one_plan": sum(1 for u in not_started if len(by_user[u]) == 1),
        "plans_that_became_a_video": len(video_plans),
        "plans_that_became_a_video_with_every_scene_pictured": full_video_plans,
        "groups": {k: groups.get(k, 0) for k, _ in GROUPS},
        "plans_saved_after_creation": sum(1 for p in window if p["revision"] > 1),
        "plans_untouched": len(untouched),
        "plans_untouched_with_no_picture": sum(1 for p in untouched if p["pictured"] == 0),
        "auto_pick": {
            "plans": len(with_auto),
            "pictures_picked": picked,
            "scenes_in_those_plans": scenes_in_auto_plans,
            "still_auto": left,
            "swapped_or_removed": max(0, picked - left),
        },
        "start_attempts": {
            "visible_for_whole_window": attempts_visible,
            "started": sum(1 for a in in_window if a["outcome"] == "started"),
            "refused_or_error": sum(refused.values()),
            "by_code": dict(sorted(refused.items())),
        },
    }


# ---------------------------------------------------------------------------
# Printing

def render(r):
    w = r["window"]
    lines = []
    lines.append("Story funnel, plans created %s to %s%s" % (
        w["since"] or "the start", w["until"] or "now", (", videos before " + w["as_of"]) if w["as_of"] else ""))
    lines.append("")
    users = r["users_with_ready_plan"]
    g = r["groups"]
    lines.append("Users with a ready plan: %d (%d plans)" % (users, r["plans_ready"]))
    share = "n/a" if r["started_share"] is None else "%s%%" % r["started_share"]
    lines.append("Started a video: %d (%s)" % (r["users_who_started_a_video"], share))
    for key, label in STARTED:
        lines.append("    %-40s %4d" % (label, g[key]))
    lines.append("Never started a video: %d (%d of them made only one plan)" % (r["users_who_never_started"], r["never_started_with_one_plan"]))
    for key, label in NOT_STARTED:
        lines.append("    %-40s %4d" % (label, g[key]))
    lines.append("")
    lines.append("Plans that became a video: %d, with every scene pictured: %d" % (
        r["plans_that_became_a_video"], r["plans_that_became_a_video_with_every_scene_pictured"]))
    lines.append("")
    lines.append("Plans saved after creation (revision > 1): %d" % r["plans_saved_after_creation"])
    lines.append("Untouched plans (never saved): %d, of which with no picture: %d" % (r["plans_untouched"], r["plans_untouched_with_no_picture"]))
    a = r["auto_pick"]
    lines.append("")
    if a["plans"]:
        lines.append("Auto-pick: %d plans, %d of %d scenes picked; %d still the plan's pick, %d swapped or removed" % (
            a["plans"], a["pictures_picked"], a["scenes_in_those_plans"], a["still_auto"], a["swapped_or_removed"]))
    else:
        lines.append("Auto-pick: no plan in this window had it")
    t = r["start_attempts"]
    if t["visible_for_whole_window"]:
        codes = ", ".join("%s %d" % kv for kv in t["by_code"].items()) or "none"
        lines.append("Start attempts logged: %d started, %d refused or errored (%s)" % (t["started"], t["refused_or_error"], codes))
    else:
        lines.append("Start attempts: NOT visible for the whole window (logging began later), so 'never clicked'")
        lines.append("  also holds users who clicked and were refused. Seen in the lines that exist: %d started, %d refused or errored." % (
            t["started"], t["refused_or_error"]))
    return "\n".join(lines)


def to_ms(when):
    """UTC milliseconds for YYYY-MM-DD or YYYY-MM-DDTHH:MM."""
    fmt = "%Y-%m-%dT%H:%M" if "T" in when else "%Y-%m-%d"
    d = datetime.datetime.strptime(when, fmt).replace(tzinfo=datetime.timezone.utc)
    return int(d.timestamp() * 1000)


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--since", help="plans created at or after this UTC date or time (YYYY-MM-DD or YYYY-MM-DDTHH:MM)")
    ap.add_argument("--until", help="plans created before this UTC date or time")
    ap.add_argument("--as-of", help="ignore videos started at or after this UTC date or time (default: now)")
    ap.add_argument("--region", default="ap-south-1")
    ap.add_argument("--users-table", default="indifferent-fun-users")
    ap.add_argument("--projects-table", default="indifferent-fun-projects")
    ap.add_argument("--log-group", default="/aws/lambda/indifferent-fun-api")
    ap.add_argument("--no-logs", action="store_true", help="do not read the API log group")
    ap.add_argument("--json", action="store_true", help="print the counts as JSON")
    args = ap.parse_args(argv)
    for name in ("since", "until", "as_of"):
        if getattr(args, name):
            try:
                to_ms(getattr(args, name))
            except ValueError:
                ap.error("--%s must be a UTC date like 2026-11-12 or a time like 2026-11-11T15:10" % name.replace("_", "-"))

    import boto3  # imported here so the counting code can be tested without it

    db = boto3.client("dynamodb", region_name=args.region)
    real = read_real_users(db, args.users_table)
    plans = read_plans(db, args.projects_table)
    videos = read_videos(db, args.projects_table)

    attempts, visible = [], False
    if not args.no_logs:
        logs = boto3.client("logs", region_name=args.region)
        now_ms = int(datetime.datetime.now(datetime.timezone.utc).timestamp() * 1000)
        attempts, first_ms = read_attempts(logs, args.log_group, 0, now_ms)
        # Attempts are matched to plans by plan id, so no time window is needed.
        # They are visible for the whole window only if logging began before it:
        # a plan made before the first line cannot show a click.
        visible = first_ms is not None and args.since is not None and to_ms(args.since) >= first_ms

    r = analyse(plans, videos, attempts, real, args.since, args.until, args.as_of, attempts_visible=visible)
    print(json.dumps(r, indent=2, sort_keys=True) if args.json else render(r))
    return 0


if __name__ == "__main__":
    sys.exit(main())
