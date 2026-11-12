#!/usr/bin/env python3
"""Count signed-up users in the users table. Read-only.

The users table holds two kinds of item, both with SK "PROFILE":

  USER#<userId>   the account: email, name, avatarUrl or passwordHash, createdAt
  EMAIL#<email>   the email index: only userId. Written at sign-up (Google and
                  password) to claim the address. It is not an account and has
                  no email attribute, so it must not be counted as a user.

Accounts with an @example.com address are test accounts and are reported
apart. The index is also checked: every EMAIL# row should point to an account
with that address, and every account should have one (accounts created before
the index existed get theirs at their next Google sign-in).

It also shows the sign-up counter COUNTER#accounts (#297), which numbers the
owner's sign-up alerts, and --seed-counter sets it to the number of real
accounts. Seeding writes only when the counter is missing or lower than that
number, so it can never lower a counter that new sign-ups have moved on.

Prints counts only, never an email or a name.

Usage:
  scripts/ops/user_counts.py [--days 7] [--table indifferent-fun-users] [--region ap-south-1]
  scripts/ops/user_counts.py --seed-counter            # dry run: says what it would set
  scripts/ops/user_counts.py --seed-counter --apply    # sets it
"""
import argparse
import collections
import datetime
import sys

import boto3

TEST_DOMAIN = "@example.com"
# The sign-up counter (backend/internal/auth/signupalert.go).
COUNTER_PK, COUNTER_SK = "COUNTER#accounts", "COUNTER"


def scan(db, table):
    items, kwargs = [], {}
    while True:
        out = db.scan(TableName=table, **kwargs)
        items += out.get("Items", [])
        if "LastEvaluatedKey" not in out:
            return items
        kwargs = {"ExclusiveStartKey": out["LastEvaluatedKey"]}


def s(item, name):
    return item.get(name, {}).get("S", "")


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--table", default="indifferent-fun-users")
    ap.add_argument("--region", default="ap-south-1")
    ap.add_argument("--days", type=int, default=7, help="show sign-ups for this many recent days")
    ap.add_argument("--seed-counter", action="store_true", help="set the sign-up counter to the number of real accounts")
    ap.add_argument("--apply", action="store_true", help="with --seed-counter: write; without it nothing is changed")
    args = ap.parse_args()

    db = boto3.client("dynamodb", region_name=args.region)
    items = scan(db, args.table)
    counter = None
    accounts, index, other = {}, {}, collections.Counter()
    for it in items:
        pk = s(it, "PK")
        if pk.startswith("USER#") and s(it, "SK") == "PROFILE":
            accounts[pk[len("USER#"):]] = it
        elif pk.startswith("EMAIL#") and s(it, "SK") == "PROFILE":
            index[pk[len("EMAIL#"):]] = s(it, "userId")
        elif pk == COUNTER_PK and s(it, "SK") == COUNTER_SK:
            counter = int(it.get("n", {}).get("N", "0"))
        else:
            other[pk.split("#", 1)[0] + "#"] += 1

    real = {u: a for u, a in accounts.items() if not s(a, "email").lower().endswith(TEST_DOMAIN)}
    test = len(accounts) - len(real)
    password = sum(1 for a in real.values() if "passwordHash" in a)
    no_email = sum(1 for a in real.values() if not s(a, "email"))

    print(f"Real users:      {len(real)}")
    print(f"  Google:        {len(real) - password}")
    print(f"  Email/password:{password:>4}")
    print(f"Test accounts:   {test} ({TEST_DOMAIN})")
    if no_email:
        print(f"  of the real users, {no_email} have no email (unexpected)")

    today = datetime.datetime.now(datetime.timezone.utc).date()
    by_day = collections.Counter(s(a, "createdAt")[:10] for a in real.values())
    print(f"Sign-ups, last {args.days} days (UTC):")
    for n in range(args.days - 1, -1, -1):
        d = (today - datetime.timedelta(days=n)).isoformat()
        print(f"  {d}  {by_day.get(d, 0)}")

    # Email index integrity.
    emails = {u: s(a, "email").lower() for u, a in accounts.items()}
    pointing = collections.Counter(index.values())
    orphans = sum(1 for e, u in index.items() if emails.get(u) != e)
    missing = sum(1 for u in accounts if u not in pointing)
    print(f"Email index rows: {len(index)} (not users)")
    print(f"  pointing to a missing account or another address: {orphans}")
    print(f"  accounts without an index row: {missing} (older accounts get one at their next Google sign-in)")
    print(f"Sign-up counter:  {counter if counter is not None else 'not set'}")
    if other:
        print(f"Other items: {dict(other)}")

    if args.seed_counter:
        want = len(real)
        if counter is not None and counter >= want:
            print(f"Counter already at {counter}, not below {want}: nothing to do.")
        elif not args.apply:
            print(f"[dry-run] would set the sign-up counter to {want}. Add --apply to write.")
        else:
            try:
                db.put_item(
                    TableName=args.table,
                    Item={"PK": {"S": COUNTER_PK}, "SK": {"S": COUNTER_SK}, "n": {"N": str(want)}},
                    ConditionExpression="attribute_not_exists(PK) OR n < :want",
                    ExpressionAttributeValues={":want": {"N": str(want)}},
                )
                print(f"Set the sign-up counter to {want}.")
            except db.exceptions.ConditionalCheckFailedException:
                print("The counter moved past that number meanwhile: left as it is.")
    return 1 if orphans or no_email else 0


if __name__ == "__main__":
    sys.exit(main())
