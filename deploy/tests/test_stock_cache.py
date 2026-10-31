"""Tests for deploy/setup-stock-cache.sh and deploy/flush-stockmedia-cache.sh, run
against the same stand-in for the aws command as test_setup_storyvideo.py. Nothing
here touches AWS.
    python3 -m unittest discover -s deploy/tests -v

What this proves and what it does not: it runs the real scripts, end to end, and
checks the calls they make and the state they leave in a model of the account. It
does not check that AWS accepts those calls (flags, argument shapes, the CLI's own
output), and the IAM check is against the stand-in's small matcher, not against the
real policy simulator. The first real run is what shows those; the scripts' dry run
is read-only so that it can be that first run.
"""
import json
import os
import unittest

from test_setup_storyvideo import (
    API_PLAIN,
    API_SECRETS,
    DEPLOY,
    Base,
    api_leaks,
    base_world,
)

SETUP = os.path.join(DEPLOY, "setup-stock-cache.sh")
FLUSH = os.path.join(DEPLOY, "flush-stockmedia-cache.sh")
TABLE = "indifferent-fun-stock-cache"


def cache_table(**over):
    t = {"status": "ACTIVE", "keys": [["PK", "HASH"]], "billing": "PAY_PER_REQUEST",
         "ttl": {"status": "ENABLED", "attr": "expiresAt"}, "items": {}}
    t.update(over)
    return t


class Setup(Base):
    def run_setup(self, *args, **kw):
        return self.run_script(*args, script=SETUP, **kw)

    def assertRefused(self, r, message):
        """A refusal is the script's own, with its message and exit status 1. A script
        that stopped because of a bug (an unbound variable, a failed command) also exits
        non-zero, and must not be mistaken for one."""
        self.assertEqual(r.rc, 1, r.text)
        self.assertIn("ERROR: " + message, r.err)
        self.assertNotIn("unbound variable", r.err)
        self.assertNothingChanged(r)

    def api_env(self):
        return self.world["api"]["env"]

    def test_a_dry_run_reads_everything_and_changes_nothing(self):
        r = self.run_setup()
        self.assertEqual(r.rc, 0, r.text)
        self.assertNothingChanged(r)
        self.assertIn("MODE: dry run", r.text)
        self.assertIn("[dry-run] would: create table %s" % TABLE, r.text)
        self.assertIn("[dry-run] would: turn on TTL for expiresAt", r.text)
        self.assertIn("[dry-run] would: set STOCK_CACHE_TABLE=%s" % TABLE, r.text)
        self.assertIsNone(self.world.get("stock_cache"))
        self.assertNotIn("STOCK_CACHE_TABLE", self.api_env())
        api_leaks(self, r)

    def test_apply_from_nothing_makes_the_table_its_ttl_and_the_variable_in_order(self):
        r = self.run_setup("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertEqual(r.ops(), ["dynamodb create-table", "dynamodb update-time-to-live", "lambda update-function-configuration"])
        t = self.world["stock_cache"]
        self.assertEqual(t["keys"], [["PK", "HASH"]])
        self.assertEqual(t["billing"], "PAY_PER_REQUEST")
        self.assertEqual(t["ttl"], {"status": "ENABLED", "attr": "expiresAt"})
        self.assertEqual(self.api_env()["STOCK_CACHE_TABLE"], TABLE)
        # The table is waited for before its TTL is set: TTL cannot be set on a table still being made.
        ops = [c["op"] for c in r.calls]
        self.assertLess(ops.index("dynamodb wait"), ops.index("dynamodb update-time-to-live"))
        self.assertLess(ops.index("dynamodb update-time-to-live"), ops.index("lambda update-function-configuration"))

    def test_the_api_environment_is_carried_through_and_nothing_secret_is_shown(self):
        before = dict(self.world["api"]["env"])
        r = self.run_setup("--apply")
        self.assertEqual(r.rc, 0, r.text)
        after = self.api_env()
        self.assertEqual({k: v for k, v in after.items() if k != "STOCK_CACHE_TABLE"}, before, "another variable changed")
        api_leaks(self, r)
        self.assertIn("kept %d other variable(s) as they were" % len(before), r.text)
        update = [c for c in r.calls if c["op"] == "lambda update-function-configuration"][0]
        info = list(update["fileinfo"].values())[0]
        self.assertEqual(info["mode"], 0o600, "the file holding the environment is not private")
        self.assertEqual(info["dirmode"], 0o700, "the directory holding it is not private")
        self.assertIn("--revision-id", update["argv"])
        self.assertIn("--query", update["argv"], "the command's own output echoes the environment unless narrowed")

    def test_running_it_again_changes_nothing(self):
        self.assertEqual(self.run_setup("--apply").rc, 0)
        r = self.run_setup("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertNothingChanged(r)
        self.assertEqual(r.text.count("Up to date."), 3, r.text)

    def test_an_existing_correct_table_is_left_alone_and_only_the_missing_steps_run(self):
        self.world["stock_cache"] = cache_table(ttl={"status": "DISABLED", "attr": None})
        r = self.run_setup("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertEqual(r.ops(), ["dynamodb update-time-to-live", "lambda update-function-configuration"])

    def test_a_table_whose_ttl_is_still_being_switched_on_counts_as_in_place(self):
        self.world["stock_cache"] = cache_table(ttl={"status": "ENABLING", "attr": "expiresAt"})
        r = self.run_setup("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertNotIn("dynamodb update-time-to-live", r.ops())

    # --- refusals: found before anything is changed

    def test_a_table_with_another_key_is_refused_and_not_touched(self):
        for keys in ([["PK", "HASH"], ["SK", "RANGE"]], [["id", "HASH"]]):
            self.world["stock_cache"] = cache_table(keys=keys)
            self.assertRefused(self.run_setup("--apply"), "table %s exists but its key is not a single string hash key named PK" % TABLE)

    def test_a_table_that_expires_items_on_another_attribute_is_refused(self):
        self.world["stock_cache"] = cache_table(ttl={"status": "ENABLED", "attr": "ttl"})
        self.assertRefused(self.run_setup("--apply"), "table %s already expires items on 'ttl'" % TABLE)

    def test_a_table_being_changed_is_refused(self):
        for status in ("CREATING", "UPDATING"):
            self.world["stock_cache"] = cache_table(status=status)
            self.assertRefused(self.run_setup("--apply"), "table %s is still being changed (%s)" % (TABLE, status))

    def test_a_table_in_an_unexpected_state_is_not_touched(self):
        self.world["stock_cache"] = cache_table(status="DELETING")
        self.assertRefused(self.run_setup("--apply"), "table %s is in state DELETING" % TABLE)

    def test_a_ttl_being_switched_off_is_refused(self):
        self.world["stock_cache"] = cache_table(ttl={"status": "DISABLING", "attr": "expiresAt"})
        self.assertRefused(self.run_setup("--apply"), "TTL on %s is being switched off" % TABLE)

    def test_a_variable_already_set_to_something_else_is_not_overwritten(self):
        self.world["api"]["env"]["STOCK_CACHE_TABLE"] = "someone-elses-table"
        self.assertRefused(self.run_setup("--apply"), "STOCK_CACHE_TABLE on indifferent-fun-api is already set, to 'someone-elses-table'")
        self.assertEqual(self.api_env()["STOCK_CACHE_TABLE"], "someone-elses-table")

    def test_a_role_that_cannot_use_the_table_is_refused_before_anything_is_made(self):
        self.world["roles"]["indifferent-fun-lambda-role"]["base"] = [{"Statement": [
            {"Effect": "Allow", "Action": ["dynamodb:GetItem"], "Resource": "arn:aws:dynamodb:ap-south-1:*:table/indifferent-fun-*"}]}]
        self.assertRefused(self.run_setup("--apply"), "the role indifferent-fun-lambda-role may not read and write")

    def test_a_role_outside_the_table_pattern_is_refused(self):
        self.world["roles"]["indifferent-fun-lambda-role"]["base"] = [{"Statement": [
            {"Effect": "Allow", "Action": ["dynamodb:GetItem", "dynamodb:PutItem"], "Resource": "arn:aws:dynamodb:ap-south-1:*:table/indifferent-fun-projects"}]}]
        self.assertRefused(self.run_setup("--apply"), "the role indifferent-fun-lambda-role may not read and write")

    def test_an_unusable_simulator_warns_and_goes_on(self):
        self.world["simulator"] = "denied"
        r = self.run_setup("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertIn("NOT checked", r.text)
        self.assertEqual(self.api_env()["STOCK_CACHE_TABLE"], TABLE)

    def test_a_missing_api_function_or_role_is_refused(self):
        self.world["api"]["exists"] = False
        r = self.run_setup("--apply")
        self.assertNotEqual(r.rc, 0)
        self.assertIn("could not read the configuration of indifferent-fun-api", r.text)
        self.assertNothingChanged(r)
        w = base_world()
        del w["roles"]["indifferent-fun-lambda-role"]
        r = self.run_setup("--apply", world=w)
        self.assertNotEqual(r.rc, 0)
        self.assertIn("role indifferent-fun-lambda-role (the API Lambda's) does not exist", r.text)
        self.assertNothingChanged(r)

    def test_a_function_that_is_being_updated_is_refused(self):
        self.world["api"]["status"] = "InProgress"
        self.assertRefused(self.run_setup("--apply"), "indifferent-fun-api cannot be changed right now")

    def test_a_function_changed_by_someone_else_after_it_was_read_is_not_overwritten(self):
        self.world["api"]["race"] = True
        before = dict(self.api_env())
        r = self.run_setup("--apply")
        self.assertNotEqual(r.rc, 0, "the write went through although the revision had changed")
        self.assertEqual(self.api_env(), before)

    def test_an_unknown_argument_is_refused_before_aws_is_asked_anything(self):
        r = self.run_setup("--aply")
        self.assertEqual(r.rc, 1)
        self.assertEqual(r.calls, [])

    # --- --unwire-api

    def test_unwiring_removes_only_the_variable_and_leaves_the_table(self):
        self.assertEqual(self.run_setup("--apply").rc, 0)
        before = {k: v for k, v in self.api_env().items() if k != "STOCK_CACHE_TABLE"}
        r = self.run_setup("--unwire-api", "--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertEqual(r.ops(), ["lambda update-function-configuration"])
        self.assertEqual(self.api_env(), before)
        self.assertIsNotNone(self.world["stock_cache"], "unwiring must not remove the table")
        api_leaks(self, r)

    def test_unwiring_when_the_variable_is_not_set_does_nothing(self):
        r = self.run_setup("--unwire-api", "--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertNothingChanged(r)
        self.assertIn("nothing to do", r.text)

    def test_unwiring_in_a_dry_run_changes_nothing(self):
        self.assertEqual(self.run_setup("--apply").rc, 0)
        r = self.run_setup("--unwire-api")
        self.assertEqual(r.rc, 0, r.text)
        self.assertNothingChanged(r)
        self.assertIn("STOCK_CACHE_TABLE", self.api_env())

    def test_nothing_is_ever_deleted(self):
        for args in ([], ["--apply"], ["--unwire-api", "--apply"]):
            r = self.run_setup(*args)
            for c in r.calls:
                self.assertNotRegex(c["op"], r"delete|remove|deregister", c["argv"])


class Flush(Base):
    def run_flush(self, *args, **kw):
        return self.run_script(*args, script=FLUSH, **kw)

    def epoch(self):
        item = self.world["stock_cache"]["items"].get("EPOCH")
        return item["epoch"]["S"] if item else None

    def test_a_dry_run_shows_the_epoch_and_writes_nothing(self):
        self.world["stock_cache"] = cache_table()
        r = self.run_flush()
        self.assertEqual(r.rc, 0, r.text)
        self.assertNothingChanged(r)
        self.assertIn("never been flushed", r.text)
        self.assertIn("[dry-run] would: write the new epoch", r.text)
        self.assertIsNone(self.epoch())

    def test_apply_writes_exactly_one_item_a_fresh_timestamp(self):
        self.world["stock_cache"] = cache_table()
        r = self.run_flush("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertEqual(r.ops(), ["dynamodb put-item"])
        self.assertRegex(self.epoch(), r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$")
        self.assertEqual(list(self.world["stock_cache"]["items"]), ["EPOCH"])
        self.assertEqual(self.world["stock_cache"]["items"]["EPOCH"]["PK"], {"S": "EPOCH"})
        self.assertIn("within about a minute", r.text)

    def test_the_current_epoch_is_read_consistently_and_printed_so_it_can_be_put_back(self):
        self.world["stock_cache"] = cache_table(items={"EPOCH": {"PK": {"S": "EPOCH"}, "epoch": {"S": "2020-01-01T00:00:00Z"}}})
        r = self.run_flush("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertIn("Current epoch: 2020-01-01T00:00:00Z", r.text)
        self.assertIn("put epoch '2020-01-01T00:00:00Z' back", r.text)
        self.assertNotEqual(self.epoch(), "2020-01-01T00:00:00Z")
        self.assertTrue(all(read["consistent"] for read in self.world["stock_cache_reads"]), "the epoch must be read consistently")

    def test_the_first_flush_says_how_to_undo_it_too(self):
        self.world["stock_cache"] = cache_table()
        r = self.run_flush("--apply")
        self.assertIn("delete-item", r.text)

    def test_other_items_in_the_table_are_not_touched(self):
        entry = {"PK": {"S": "E#" + "ab" * 32}, "data": {"B": "x"}}
        self.world["stock_cache"] = cache_table(items={entry["PK"]["S"]: entry})
        self.assertEqual(self.run_flush("--apply").rc, 0)
        self.assertEqual(self.world["stock_cache"]["items"][entry["PK"]["S"]], entry)

    def test_a_missing_table_is_refused_and_not_created(self):
        r = self.run_flush("--apply")
        self.assertNotEqual(r.rc, 0)
        self.assertIn("setup-stock-cache.sh", r.text)
        self.assertNothingChanged(r)
        self.assertIsNone(self.world.get("stock_cache"))

    def test_a_table_with_the_wrong_key_is_refused(self):
        self.world["stock_cache"] = cache_table(keys=[["PK", "HASH"], ["SK", "RANGE"]])
        r = self.run_flush("--apply")
        self.assertNotEqual(r.rc, 0)
        self.assertIn("not the stock cache", r.text)
        self.assertNothingChanged(r)

    def test_a_table_not_yet_active_is_refused(self):
        self.world["stock_cache"] = cache_table(status="CREATING")
        r = self.run_flush("--apply")
        self.assertNotEqual(r.rc, 0)
        self.assertNothingChanged(r)

    def test_an_unknown_argument_is_refused_before_aws_is_asked_anything(self):
        r = self.run_flush("--now")
        self.assertEqual(r.rc, 1)
        self.assertEqual(r.calls, [])

    def test_the_scripts_work_together(self):
        self.assertEqual(self.run_script("--apply", script=SETUP).rc, 0)
        r = self.run_flush("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertIsNotNone(self.epoch())


if __name__ == "__main__":
    unittest.main()
