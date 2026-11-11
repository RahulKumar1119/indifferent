#!/usr/bin/env python3
"""Recode Shorts jobs that failed with "no engaging segments" (issue #286).

Before the #286 fix these jobs were stored with errorCode INFRASTRUCTURE and
the raw Step Functions cause as their error, which the progress page showed as
JSON next to a Retry button. This sets, on each named job and nothing else:

    errorCode = VALIDATION_NO_SEGMENTS
    error     = the message the fixed pipeline now stores for that code

Every other attribute is left as it is (updatedAt included).

Safety:
  * Only the job IDs given on the command line are touched.
  * Dry run by default. --apply writes.
  * Before any write, the full original items are saved to --backup (mode
    600). A backup that already exists is kept: originals already in it are
    never overwritten by a later, already-recoded copy.
  * Each write is conditional on the job still being the failure it was
    (failed, at ranking, INFRASTRUCTURE or already recoded, with the
    no-segments cause or the new message), so running it twice is a no-op
    and a job that changed since (retried, deleted) is skipped.
  * After each write the item is read back and every attribute except error
    and errorCode is compared with the original.
  * --restore BACKUP puts error and errorCode back from the backup, only on
    items that still carry the recoded values.

Usage:
  scripts/ops/286_recode_no_segments.py --backup ~/x.json JOB_ID...           # dry run
  scripts/ops/286_recode_no_segments.py --backup ~/x.json --apply JOB_ID...   # write
  scripts/ops/286_recode_no_segments.py --restore ~/x.json [--apply]          # undo
"""
import argparse
import json
import os
import re
import sys

import boto3
from boto3.dynamodb.types import TypeDeserializer

NEW_CODE = "VALIDATION_NO_SEGMENTS"
OLD_CODE = "INFRASTRUCTURE"
# Must equal shorts.NoSegmentsMessage in backend/internal/shorts/constants.go.
NEW_MESSAGE = (
    "We could not find a moment of 30 to 90 seconds to cut into a clip. "
    "Try a longer file with more speech."
)
NEEDLE = "no engaging segments"
JOB_ID = re.compile(r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
CHANGED = {"error", "errorCode"}

_deser = TypeDeserializer()


def plain(item):
    return {k: _deser.deserialize(v) for k, v in item.items()}


def find_pk(db, table, sk):
    """The PK of the one item with this SK. A small table: a filtered scan."""
    pks, kwargs = [], {}
    while True:
        out = db.scan(
            TableName=table,
            FilterExpression="SK = :sk",
            ExpressionAttributeValues={":sk": {"S": sk}},
            ProjectionExpression="PK",
            **kwargs,
        )
        pks += [i["PK"]["S"] for i in out.get("Items", [])]
        if "LastEvaluatedKey" not in out:
            return pks
        kwargs = {"ExclusiveStartKey": out["LastEvaluatedKey"]}


def get(db, table, pk, sk):
    out = db.get_item(TableName=table, Key={"PK": {"S": pk}, "SK": {"S": sk}}, ConsistentRead=True)
    return out.get("Item")


def eligible(item):
    """Why this item may not be recoded, or '' when it may."""
    p = plain(item)
    if p.get("status") != "failed":
        return f"status is {p.get('status')!r}, not 'failed'"
    if p.get("failedStage") != "ranking":
        return f"failedStage is {p.get('failedStage')!r}, not 'ranking'"
    if p.get("errorCode") not in (OLD_CODE, NEW_CODE):
        return f"errorCode is {p.get('errorCode')!r}"
    err = p.get("error", "")
    if NEEDLE not in err and err != NEW_MESSAGE:
        return "error is not the no-segments cause"
    return ""


def load_backup(path):
    if not os.path.exists(path):
        return {}
    with open(path) as f:
        return {(i["PK"]["S"], i["SK"]["S"]): i for i in json.load(f)}


def save_backup(path, items):
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w") as f:
        json.dump(list(items.values()), f, indent=1, sort_keys=True)
    os.chmod(path, 0o600)


def same_except_changed(before, after):
    keys = (set(before) | set(after)) - CHANGED
    return [k for k in sorted(keys) if before.get(k) != after.get(k)]


def recode(db, args):
    bad = [j for j in args.jobs if not JOB_ID.match(j)]
    if bad:
        sys.exit(f"not job ids: {bad}")
    backup = load_backup(args.backup)
    targets = []
    for job in args.jobs:
        sk = f"SHORTS#{job}"
        pks = find_pk(db, args.table, sk)
        if len(pks) != 1:
            print(f"SKIP {job}: {len(pks)} items with that SK")
            continue
        item = get(db, args.table, pks[0], sk)
        if item is None:
            print(f"SKIP {job}: gone")
            continue
        why = eligible(item)
        if why:
            print(f"SKIP {job}: {why}")
            continue
        p = plain(item)
        state = "already recoded" if p["errorCode"] == NEW_CODE and p.get("error") == NEW_MESSAGE else "to recode"
        print(f"OK   {job}: {state} (errorCode={p['errorCode']}, error={p.get('error', '')[:80]!r})")
        targets.append((pks[0], sk, item))
        # Keep the first copy seen: that is the original.
        backup.setdefault((pks[0], sk), item)

    save_backup(args.backup, backup)
    print(f"backup: {len(backup)} item(s) in {args.backup}")
    if not args.apply:
        print("dry run: nothing written. Add --apply to write.")
        return 0

    failed = 0
    for pk, sk, before in targets:
        try:
            db.update_item(
                TableName=args.table,
                Key={"PK": {"S": pk}, "SK": {"S": sk}},
                UpdateExpression="SET #code = :new, #err = :msg",
                ConditionExpression=(
                    "#status = :failed AND failedStage = :ranking AND "
                    "(#code = :old OR #code = :new) AND (contains(#err, :needle) OR #err = :msg)"
                ),
                ExpressionAttributeNames={"#code": "errorCode", "#err": "error", "#status": "status"},
                ExpressionAttributeValues={
                    ":new": {"S": NEW_CODE}, ":old": {"S": OLD_CODE}, ":msg": {"S": NEW_MESSAGE},
                    ":failed": {"S": "failed"}, ":ranking": {"S": "ranking"}, ":needle": {"S": NEEDLE},
                },
            )
        except db.exceptions.ConditionalCheckFailedException:
            print(f"SKIP {sk}: changed since it was read")
            failed += 1
            continue
        after = get(db, args.table, pk, sk)
        drift = same_except_changed(plain(before), plain(after))
        pa = plain(after)
        if drift or pa.get("errorCode") != NEW_CODE or pa.get("error") != NEW_MESSAGE:
            print(f"FAIL {sk}: other attributes changed: {drift}")
            failed += 1
        else:
            print(f"DONE {sk}: recoded, other attributes unchanged")
    return 1 if failed else 0


def restore(db, args):
    backup = load_backup(args.restore)
    if not backup:
        sys.exit(f"no backup at {args.restore}")
    failed = 0
    for (pk, sk), orig in backup.items():
        o = plain(orig)
        print(f"{'RESTORE' if args.apply else 'WOULD RESTORE'} {sk}: errorCode={o.get('errorCode')}")
        if not args.apply:
            continue
        try:
            db.update_item(
                TableName=args.table,
                Key={"PK": {"S": pk}, "SK": {"S": sk}},
                UpdateExpression="SET #code = :code, #err = :err",
                ConditionExpression="#code = :new AND #err = :msg",
                ExpressionAttributeNames={"#code": "errorCode", "#err": "error"},
                ExpressionAttributeValues={
                    ":code": orig["errorCode"], ":err": orig["error"],
                    ":new": {"S": NEW_CODE}, ":msg": {"S": NEW_MESSAGE},
                },
            )
        except db.exceptions.ConditionalCheckFailedException:
            print(f"SKIP {sk}: no longer carries the recoded values")
            failed += 1
    return 1 if failed else 0


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--table", default="indifferent-fun-projects")
    ap.add_argument("--region", default="ap-south-1")
    ap.add_argument("--apply", action="store_true", help="write; without it nothing is changed")
    ap.add_argument("--backup", help="where to keep the original items (required to recode)")
    ap.add_argument("--restore", metavar="BACKUP", help="put error and errorCode back from this backup")
    ap.add_argument("jobs", nargs="*")
    args = ap.parse_args()
    db = boto3.client("dynamodb", region_name=args.region)
    if args.restore:
        return restore(db, args)
    if not args.backup or not args.jobs:
        ap.error("--backup and at least one job id are required")
    return recode(db, args)


if __name__ == "__main__":
    sys.exit(main())
