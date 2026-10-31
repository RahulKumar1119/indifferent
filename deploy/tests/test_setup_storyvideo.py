"""Tests for deploy/setup-storyvideo.sh, run against a stand-in for the aws,
docker and git commands (see stubs/stub.py). Nothing here touches AWS.

    python3 -m unittest discover -s deploy/tests -v

What this proves and what it does not: it runs the real script, end to end, and
checks the calls it makes and the state they leave in a model of the account. It
does not check that AWS accepts those calls. The things that still need a real run
are listed in backend/internal/storyjob/README.md ("What was and was not checked").
"""
import copy
import glob
import json
import os
import re
import shutil
import subprocess
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
STUBS = os.path.join(HERE, "stubs")
DEPLOY = os.path.dirname(HERE)
SCRIPT = os.path.join(DEPLOY, "setup-storyvideo.sh")

SECRET = "pbx-TEST-SECRET-9f3a7c21"
PEXELS_SECRET = "pxl-TEST-SECRET-55d0e8b4"
ACCT = "123456789012"
PREFIX = "/indifferent-fun/story-render"


def shorts_sfn_policy():
    """What setup-shorts.sh and setup-notifications.sh already gave the Step Functions role."""
    return {"Statement": [
        {"Effect": "Allow", "Action": ["ecs:RunTask", "ecs:StopTask", "ecs:DescribeTasks"], "Resource": "*"},
        {"Effect": "Allow", "Action": ["iam:PassRole"], "Resource": [
            "arn:aws:iam::%s:role/indifferent-fun-shorts-task-exec-role" % ACCT,
            "arn:aws:iam::%s:role/indifferent-fun-shorts-task-role" % ACCT]},
        {"Effect": "Allow", "Action": ["events:PutTargets", "events:PutRule", "events:DescribeRule"],
         "Resource": "arn:aws:events:ap-south-1:%s:rule/StepFunctionsGetEventsForECSTaskRule" % ACCT},
        {"Effect": "Allow", "Action": ["lambda:InvokeFunction"], "Resource": [
            "arn:aws:lambda:ap-south-1:*:function:indifferent-fun-shorts-*",
            "arn:aws:lambda:ap-south-1:%s:function:indifferent-fun-notifier" % ACCT]},
    ]}


def base_world():
    """A healthy account with the shorts stack and the notifier, and nothing of this job's."""
    return {
        "account": ACCT, "bucket": True, "table": True,
        "cluster": {"status": "ACTIVE", "providers": ["FARGATE", "FARGATE_SPOT"]},
        "sg": "sg-0abc1234", "subnets": ["subnet-f01f6e8b", "subnet-880ef0e3", "subnet-7bf7d837"],
        "notifier": {"exists": True, "knows_story": True},
        "ssm": {},
        "roles": {"indifferent-fun-sfn-role": {"attached": [], "policies": {}, "base": [shorts_sfn_policy()]}},
        "lifecycle": {"TransitionDefaultMinimumObjectSize": "varies_by_storage_class", "Rules": [
            {"ID": "temp-cleanup", "Status": "Enabled", "Filter": {"Prefix": "temp/"}, "Expiration": {"Days": 1}}]},
        "ecr": {"exists": False, "mutable": None, "scan": None, "images": [], "policy": None},
        "logs": {"exists": False, "retention": None},
        "taskdefs": [], "sm": None, "validator": "OK",
        "git": {"sha": "abc123def456", "dirty": []},
    }


class Result:
    def __init__(self, rc, out, err, calls, world):
        self.rc, self.out, self.err, self.calls, self.world = rc, out, err, calls, world

    @property
    def text(self):
        return self.out + self.err

    @property
    def changes(self):
        return [c for c in self.calls if c["mutating"]]

    def ops(self):
        return [c["op"] for c in self.changes]


class Base(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.mkdtemp(prefix="storyvideo-test-")
        self.addCleanup(shutil.rmtree, self.dir, True)
        self.world_path = os.path.join(self.dir, "world.json")
        self.calls_path = os.path.join(self.dir, "calls.jsonl")
        self.env_path = os.path.join(self.dir, "env")
        self.world = base_world()
        self.write_env("PIXABAY_API_KEY=%s\n" % SECRET)
        self.backups = []

    def tearDown(self):
        for f in self.backups:
            if os.path.exists(f):
                os.unlink(f)

    def write_env(self, text):
        with open(self.env_path, "w") as f:
            f.write(text)

    def run_script(self, *args, extra_env=None, script=SCRIPT, world=None):
        """Runs the script once. The world persists between calls to this method."""
        if world is not None:
            self.world = world
        with open(self.world_path, "w") as f:
            json.dump(self.world, f)
        open(self.calls_path, "w").close()
        env = dict(os.environ, PATH=STUBS + os.pathsep + os.environ["PATH"], STUB_WORLD=self.world_path,
                   STUB_CALLS=self.calls_path, DEPLOY_ENV_FILE=self.env_path)
        env.pop("STORY_RENDER_TAG", None)
        env.pop("ALLOW_DIRTY", None)
        env.update(extra_env or {})
        p = subprocess.run(["bash", script, *args], capture_output=True, text=True, env=env, timeout=300)
        with open(self.calls_path) as f:
            calls = [json.loads(line) for line in f if line.strip()]
        with open(self.world_path) as f:
            self.world = json.load(f)
        for m in re.findall(r"saved the previous \w+ ?\w* to (/tmp/\S+\.json)", p.stdout):
            self.backups.append(m)
        r = Result(p.returncode, p.stdout, p.stderr, calls, self.world)
        self.assertEqual([c["argv"] for c in calls if c.get("unhandled")], [], "the script made a call the stub does not know")
        return r

    def assertNothingChanged(self, r):
        self.assertEqual(r.ops(), [], "something was changed:\n" + r.text)

    def assertNoLeak(self, r, secret=SECRET):
        blob = r.text + json.dumps([c["argv"] for c in r.calls]) + json.dumps([c["files"] for c in r.calls])
        self.assertNotIn(secret, blob, "the secret appears in output, a command line, or a file passed to a command")


def idx(ops, op):
    return ops.index(op)


class DryRun(Base):
    def test_a_dry_run_changes_nothing_and_says_what_it_would_do(self):
        before = copy.deepcopy(self.world)
        r = self.run_script()
        self.assertEqual(r.rc, 0, r.text)
        self.assertNothingChanged(r)
        self.assertEqual(self.world, before, "the dry run changed the account")
        for what in ["create SecureString /indifferent-fun/story-render/PIXABAY_API_KEY", "create role indifferent-fun-story-render-exec-role",
                     "create role indifferent-fun-story-render-task-role", "merge the two rules", "create ECR repository",
                     "build and push", "create log group", "register a task definition revision", "create state machine"]:
            self.assertIn("[dry-run] would: " + what, r.out)
        self.assertNoLeak(r)

    def test_a_dry_run_does_not_call_the_notifier(self):
        r = self.run_script()
        self.assertNotIn("lambda invoke", [c["op"] for c in r.calls])
        self.assertIn("skipped in a dry run", r.out)

    def test_it_asks_the_real_validator_and_the_policy_simulator(self):
        r = self.run_script()
        ops = [c["op"] for c in r.calls]
        self.assertIn("stepfunctions validate-state-machine-definition", ops)
        # one call per check: 9 on the task role and 2 on the execution role (their
        # policies are simulated on their own), 7 on the Step Functions role (its
        # existing policies plus the new one)
        self.assertEqual(ops.count("iam simulate-custom-policy"), 11)
        self.assertEqual(ops.count("iam simulate-principal-policy"), 7)
        self.assertIn("ok   allows ecs:RunTask", r.out)
        self.assertIn("ok   denies s3:DeleteObject", r.out)

    def test_an_unknown_argument_is_refused(self):
        r = self.run_script("--aply")
        self.assertEqual(r.rc, 1)
        self.assertEqual(r.calls, [])


class Apply(Base):
    def test_apply_builds_everything(self):
        r = self.run_script("--apply")
        self.assertEqual(r.rc, 0, r.text)
        w = self.world
        # keys
        self.assertEqual(w["ssm"], {PREFIX + "/PIXABAY_API_KEY": "SecureString"})
        self.assertEqual(w["ssm_values"][PREFIX + "/PIXABAY_API_KEY"], SECRET)
        # roles and policies
        self.assertEqual(set(w["roles"]) - {"indifferent-fun-sfn-role"}, {"indifferent-fun-story-render-exec-role", "indifferent-fun-story-render-task-role"})
        self.assertEqual(w["roles"]["indifferent-fun-story-render-exec-role"]["attached"], ["AmazonECSTaskExecutionRolePolicy"])
        self.assertIn("indifferent-fun-story-sfn-permissions", w["roles"]["indifferent-fun-sfn-role"]["policies"])
        # lifecycle
        ids = [x["ID"] for x in w["lifecycle"]["Rules"]]
        self.assertEqual(ids, ["temp-cleanup", "storyvideo-work-expiry", "storyvideo-work-delete-markers"])
        # image
        self.assertTrue(w["ecr"]["exists"])
        self.assertEqual(w["ecr"]["mutable"], "IMMUTABLE")
        self.assertEqual(w["ecr"]["images"], ["abc123def456"])
        self.assertEqual(w["logs"], {"exists": True, "retention": "30"})
        # task definition
        td = w["taskdefs"][-1]
        self.assertEqual((td["cpu"], td["memory"]), ("2048", "4096"))
        c = td["containerDefinitions"][0]
        self.assertEqual(c["name"], "story-render")
        self.assertEqual(c["image"], "%s.dkr.ecr.ap-south-1.amazonaws.com/indifferent-fun-story-render:abc123def456" % ACCT)
        self.assertEqual({e["name"]: e["value"] for e in c["environment"]}, {"S3_BUCKET": "indifferent-fun-assets", "DYNAMODB_TABLE": "indifferent-fun-projects"})
        self.assertEqual(c["secrets"], [{"name": "PIXABAY_API_KEY", "valueFrom": "arn:aws:ssm:ap-south-1:%s:parameter%s/PIXABAY_API_KEY" % (ACCT, PREFIX)}])
        # state machine
        sm = json.loads(w["sm"])
        self.assertEqual(len(sm["States"]), 14)
        self.assertEqual(w["sm_role"], "arn:aws:iam::%s:role/indifferent-fun-sfn-role" % ACCT)
        self.assertNotIn("${", w["sm"])
        spot = sm["States"]["RunOnSpot"]["Parameters"]
        self.assertEqual(spot["Cluster"], "arn:aws:ecs:ap-south-1:%s:cluster/indifferent-fun-shorts" % ACCT)
        self.assertEqual(spot["TaskDefinition"], "arn:aws:ecs:ap-south-1:%s:task-definition/indifferent-fun-story-render" % ACCT)
        self.assertEqual(spot["NetworkConfiguration"]["AwsvpcConfiguration"]["Subnets"], w["subnets"])
        self.assertEqual(spot["NetworkConfiguration"]["AwsvpcConfiguration"]["SecurityGroups"], ["sg-0abc1234"])
        self.assertEqual(sm["States"]["ReadJob"]["Parameters"]["TableName"], "indifferent-fun-projects")

    def test_things_are_made_in_the_order_they_depend_on_each_other(self):
        r = self.run_script("--apply")
        ops = r.ops()
        order = ["ssm put-parameter", "iam create-role", "iam put-role-policy", "s3api put-bucket-lifecycle-configuration",
                 "ecr create-repository", "docker push", "logs create-log-group", "ecs register-task-definition",
                 "stepfunctions create-state-machine"]
        for a, b in zip(order, order[1:]):
            self.assertLess(idx(ops, a), idx(ops, b), "%s should come before %s: %s" % (a, b, ops))
        # the image is pushed only after the repository exists and was built
        self.assertLess(idx(ops, "docker build"), idx(ops, "docker push"))

    def test_the_key_never_reaches_output_a_command_line_or_the_task_definition(self):
        r = self.run_script("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertNoLeak(r)
        self.assertNotIn(SECRET, json.dumps(self.world["taskdefs"]))
        self.assertNotIn(SECRET, self.world["sm"])
        put = [c for c in r.calls if c["op"] == "ssm put-parameter"][0]["argv"]
        self.assertTrue(any(a.startswith("file://") for a in put), "the key must travel in a file, not an argument")

    def test_a_second_apply_changes_nothing(self):
        first = self.run_script("--apply")
        self.assertEqual(first.rc, 0, first.text)
        snapshot = copy.deepcopy(self.world)
        second = self.run_script("--apply")
        self.assertEqual(second.rc, 0, second.text)
        self.assertNothingChanged(second)
        self.assertEqual(self.world, snapshot)
        self.assertEqual(len(self.world["taskdefs"]), 1, "a revision was made for nothing")
        self.assertNotIn("docker build", [c["op"] for c in second.calls])
        for line in ["Image abc123def456".replace("Image ", "Image indifferent-fun-story-render:") + " is already pushed.", "Both rules are already in place.",
                     "The live definition already is this.", "Up to date."]:
            self.assertIn(line, second.out)

    def test_a_dry_run_after_an_apply_has_nothing_left_to_plan(self):
        self.run_script("--apply")
        r = self.run_script()
        self.assertEqual(r.rc, 0, r.text)
        planned = [l for l in r.out.splitlines() if "[dry-run] would:" in l and "deployed notifier knows" not in l]
        self.assertEqual(planned, [])

    def test_a_new_commit_gets_a_new_image_and_a_new_revision_and_keeps_the_old_ones(self):
        self.run_script("--apply")
        self.world["git"]["sha"] = "fedcba654321"
        r = self.run_script("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertEqual(self.world["ecr"]["images"], ["abc123def456", "fedcba654321"])
        self.assertEqual(len(self.world["taskdefs"]), 2)
        self.assertTrue(self.world["taskdefs"][0]["containerDefinitions"][0]["image"].endswith(":abc123def456"), "the old revision must keep its image")
        self.assertTrue(self.world["taskdefs"][1]["containerDefinitions"][0]["image"].endswith(":fedcba654321"))
        self.assertEqual(r.ops(), ["docker login", "docker build", "docker push", "ecs register-task-definition"])

    def test_a_changed_task_size_registers_a_revision_without_a_rebuild(self):
        self.run_script("--apply")
        r = self.run_script("--apply", extra_env={"STORY_RENDER_CPU": "4096", "STORY_RENDER_MEMORY": "8192"})
        self.assertEqual(r.ops(), ["ecs register-task-definition"])
        self.assertEqual(self.world["taskdefs"][-1]["cpu"], "4096")

    def test_both_keys_are_stored_and_both_reach_the_task_and_the_role_policy(self):
        self.write_env("PEXELS_API_KEY=%s\nPIXABAY_API_KEY=%s\n" % (PEXELS_SECRET, SECRET))
        r = self.run_script("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertEqual(sorted(self.world["ssm"]), [PREFIX + "/PEXELS_API_KEY", PREFIX + "/PIXABAY_API_KEY"])
        names = [s["name"] for s in self.world["taskdefs"][-1]["containerDefinitions"][0]["secrets"]]
        self.assertEqual(sorted(names), ["PEXELS_API_KEY", "PIXABAY_API_KEY"])
        res = self.world["roles"]["indifferent-fun-story-render-exec-role"]["policies"]["indifferent-fun-story-render-secrets"]["Statement"][0]["Resource"]
        self.assertEqual(len(res), 2)
        self.assertNoLeak(r)
        self.assertNoLeak(r, PEXELS_SECRET)

    def test_the_env_file_is_read_not_run(self):
        marker = os.path.join(self.dir, "ran")
        self.write_env('# PIXABAY_API_KEY=commented-out\r\n$(touch %s)\r\nPIXABAY_API_KEY=old\r\nexport PIXABAY_API_KEY="%s"\r\n' % (marker, SECRET))
        r = self.run_script("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertFalse(os.path.exists(marker), "the env file was executed")
        self.assertEqual(self.world["ssm_values"][PREFIX + "/PIXABAY_API_KEY"], SECRET, "the last assignment wins; quotes and CR are removed")


class Lifecycle(Base):
    def test_the_other_rules_and_the_size_setting_are_kept_and_the_old_ones_are_saved(self):
        old = copy.deepcopy(self.world["lifecycle"]["Rules"])
        r = self.run_script("--apply")
        rules = self.world["lifecycle"]["Rules"]
        self.assertEqual(rules[0], old[0])
        self.assertEqual(self.world["lifecycle"]["TransitionDefaultMinimumObjectSize"], "varies_by_storage_class")
        work = {x["ID"]: x for x in rules[1:]}
        e = work["storyvideo-work-expiry"]
        self.assertEqual((e["Filter"], e["Expiration"], e["NoncurrentVersionExpiration"]), ({"Prefix": "storyvideo-work/"}, {"Days": 7}, {"NoncurrentDays": 1}))
        self.assertEqual(work["storyvideo-work-delete-markers"]["Expiration"], {"ExpiredObjectDeleteMarker": True})
        for x in work.values():
            self.assertEqual(x["Status"], "Enabled")
            self.assertNotIn("storyvideo/", x["Filter"]["Prefix"].replace("storyvideo-work/", ""))
        m = re.search(r"saved the previous rules to (/tmp/\S+\.json)", r.out)
        self.assertTrue(m, r.out)
        with open(m.group(1)) as f:
            self.assertEqual(json.load(f), {"Rules": old})
        self.assertIn("undo: aws s3api put-bucket-lifecycle-configuration", r.out)

    def test_with_no_lifecycle_configuration_at_all_it_creates_one_and_says_how_to_undo(self):
        self.world["lifecycle"] = None
        r = self.run_script("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertEqual(len(self.world["lifecycle"]["Rules"]), 2)
        self.assertIn("delete-bucket-lifecycle", r.out)

    def test_a_rule_that_could_reach_the_videos_stops_everything_before_any_change(self):
        risky = {
            "whole bucket, empty filter": {"ID": "all", "Status": "Enabled", "Filter": {}, "Expiration": {"Days": 30}},
            "whole bucket, empty prefix": {"ID": "all", "Status": "Enabled", "Filter": {"Prefix": ""}, "Expiration": {"Days": 30}},
            "legacy form, no filter": {"ID": "all", "Status": "Enabled", "Prefix": "", "Expiration": {"Days": 30}},
            "a prefix that contains it": {"ID": "s", "Status": "Enabled", "Filter": {"Prefix": "story"}, "Expiration": {"Days": 30}},
            "the output prefix itself": {"ID": "o", "Status": "Enabled", "Filter": {"Prefix": "storyvideo/"}, "Expiration": {"Days": 30}},
            "inside the output prefix": {"ID": "o", "Status": "Enabled", "Filter": {"Prefix": "storyvideo/old/"}, "Expiration": {"Days": 30}},
            "a tag only": {"ID": "t", "Status": "Enabled", "Filter": {"Tag": {"Key": "k", "Value": "v"}}, "Expiration": {"Days": 30}},
            "an and filter": {"ID": "a", "Status": "Enabled", "Filter": {"And": {"Prefix": "", "Tags": [{"Key": "k", "Value": "v"}]}}, "Expiration": {"Days": 30}},
            "noncurrent versions only": {"ID": "n", "Status": "Enabled", "Filter": {"Prefix": ""}, "NoncurrentVersionExpiration": {"NoncurrentDays": 5}},
            "a move to cold storage": {"ID": "g", "Status": "Enabled", "Filter": {"Prefix": ""}, "Transitions": [{"Days": 30, "StorageClass": "GLACIER"}]},
        }
        for name, rule in risky.items():
            for mode in ([], ["--apply"]):
                with self.subTest(rule=name, mode=mode):
                    w = base_world()
                    w["lifecycle"]["Rules"].append(rule)
                    before = copy.deepcopy(w)
                    r = self.run_script(*mode, world=w)
                    self.assertNotEqual(r.rc, 0, r.text)
                    self.assertIn("could expire or move objects under storyvideo/", r.err)
                    self.assertNothingChanged(r)
                    self.assertEqual(self.world, before)

    def test_rules_that_cannot_reach_the_videos_are_left_alone(self):
        safe = [
            {"ID": "w", "Status": "Enabled", "Filter": {"Prefix": "storyvideo-work/old/"}, "Expiration": {"Days": 3}},
            {"ID": "off", "Status": "Disabled", "Filter": {"Prefix": ""}, "Expiration": {"Days": 3}},
            {"ID": "abort-only", "Status": "Enabled", "Filter": {"Prefix": ""}, "AbortIncompleteMultipartUpload": {"DaysAfterInitiation": 7}},
            {"ID": "shorts", "Status": "Enabled", "Filter": {"Prefix": "shorts/"}, "Expiration": {"Days": 3}},
        ]
        w = base_world()
        w["lifecycle"]["Rules"] += safe
        r = self.run_script("--apply", world=w)
        self.assertEqual(r.rc, 0, r.text)
        self.assertEqual(self.world["lifecycle"]["Rules"][:5], w["lifecycle"]["Rules"][:5])

    def test_a_drifted_rule_of_ours_is_put_back(self):
        self.run_script("--apply")
        for x in self.world["lifecycle"]["Rules"]:
            if x["ID"] == "storyvideo-work-expiry":
                x["Expiration"] = {"Days": 365}
        r = self.run_script("--apply")
        self.assertEqual(r.ops(), ["s3api put-bucket-lifecycle-configuration"])
        exp = [x for x in self.world["lifecycle"]["Rules"] if x["ID"] == "storyvideo-work-expiry"][0]
        self.assertEqual(exp["Expiration"], {"Days": 7})
        self.assertEqual(len(self.world["lifecycle"]["Rules"]), 3, "a rule was duplicated")


class RefusalsHappenBeforeAnyChange(Base):
    def refuse(self, mutate, *, mode="--apply", extra_env=None, expect=None):
        w = base_world()
        mutate(w)
        before = copy.deepcopy(w)
        r = self.run_script(mode, extra_env=extra_env, world=w)
        self.assertNotEqual(r.rc, 0, r.text)
        self.assertNothingChanged(r)
        self.assertEqual(self.world, before, "the account changed")
        if expect:
            self.assertIn(expect, r.err)
        return r

    def test_each_missing_prerequisite_stops_the_run_and_says_what_to_run_first(self):
        cases = {
            "bucket": (lambda w: w.update(bucket=False), "setup.sh"),
            "table": (lambda w: w.update(table=False), "setup.sh"),
            "cluster": (lambda w: w.update(cluster=None), "setup-shorts.sh"),
            "spot capacity": (lambda w: w["cluster"].update(providers=["FARGATE"]), "FARGATE_SPOT"),
            "security group": (lambda w: w.update(sg=None), "setup-shorts.sh"),
            "a subnet": (lambda w: w.update(subnets=w["subnets"][:2]), "subnets"),
            "step functions role": (lambda w: w["roles"].pop("indifferent-fun-sfn-role"), "setup.sh"),
            "notifier": (lambda w: w["notifier"].update(exists=False), "setup-notifications.sh"),
        }
        for name, (mutate, hint) in cases.items():
            for mode in ("--apply", ""):
                with self.subTest(missing=name, mode=mode):
                    self.refuse(mutate, mode=mode, expect=hint)

    def test_an_old_notifier_stops_an_apply_and_is_only_noticed_by_an_apply(self):
        r = self.refuse(lambda w: w["notifier"].update(knows_story=False), expect="setup-notifications.sh --apply first")
        self.assertIn("lambda invoke", [c["op"] for c in r.calls])
        w = base_world()
        w["notifier"]["knows_story"] = False
        dry = self.run_script(world=w)
        self.assertEqual(dry.rc, 0, "a dry run cannot tell, and says so")

    def test_no_stock_key_anywhere_stops_the_run(self):
        for name, env in {"empty file": "", "placeholder": "PIXABAY_API_KEY=your-pixabay-key\n", "blank value": "PIXABAY_API_KEY=\n", "no file": None}.items():
            with self.subTest(env=name):
                if env is None:
                    os.unlink(self.env_path)
                else:
                    self.write_env(env)
                self.refuse(lambda w: None, expect="no stock key")

    def test_an_existing_parameter_is_used_without_the_env_file_and_is_never_overwritten(self):
        os.unlink(self.env_path)
        w = base_world()
        w["ssm"][PREFIX + "/PIXABAY_API_KEY"] = "SecureString"
        r = self.run_script("--apply", world=w)
        self.assertEqual(r.rc, 0, r.text)
        self.assertNotIn("ssm put-parameter", r.ops())
        self.write_env("PIXABAY_API_KEY=a-different-key\n")
        r = self.run_script("--apply")
        self.assertNotIn("ssm put-parameter", r.ops(), "an existing key was overwritten")

    def test_a_plain_string_parameter_where_a_secret_belongs_is_refused(self):
        self.refuse(lambda w: w["ssm"].update({PREFIX + "/PIXABAY_API_KEY": "String"}), expect="not SecureString")

    def test_uncommitted_changes_stop_an_apply_that_must_build(self):
        r = self.refuse(lambda w: w["git"].update(dirty=[" M backend/internal/storyjob/run.go"]), expect="uncommitted changes")
        self.assertIn("ALLOW_DIRTY", r.err)

    def test_a_dirty_tree_only_warns_in_a_dry_run_and_can_be_overridden(self):
        w = base_world()
        w["git"]["dirty"] = [" M backend/x.go"]
        dry = self.run_script(world=w)
        self.assertEqual(dry.rc, 0, dry.text)
        self.assertIn("WARNING: backend/ has uncommitted changes", dry.out)
        ok = self.run_script("--apply", extra_env={"ALLOW_DIRTY": "1"}, world=w)
        self.assertEqual(ok.rc, 0, ok.text)
        w2 = base_world()
        w2["git"] = {"sha": "x", "dirty": [" M a"]}
        ok2 = self.run_script("--apply", extra_env={"STORY_RENDER_TAG": "my-tag"}, world=w2)
        self.assertEqual(ok2.rc, 0, ok2.text)
        self.assertEqual(self.world["ecr"]["images"], ["my-tag"])

    def test_a_dirty_tree_does_not_matter_when_the_image_is_already_pushed(self):
        self.run_script("--apply")
        self.world["git"]["dirty"] = [" M backend/x.go"]
        r = self.run_script("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertNothingChanged(r)

    def test_a_definition_the_validator_rejects_stops_the_run_before_any_change(self):
        self.refuse(lambda w: w.update(validator="FAIL"), expect="rejected the rendered definition")

    def test_a_missing_docker_stops_an_apply_before_any_change(self):
        # A PATH with the stubs and the real tools the script uses, and no docker.
        with tempfile.TemporaryDirectory() as bin_dir:
            for name in ("aws", "git", "sleep", "stub.py"):
                shutil.copy(os.path.join(STUBS, name), os.path.join(bin_dir, name))
            for tool in ("bash", "sed", "tr", "grep", "tail", "head", "cat", "cp", "rm", "mktemp", "date", "python3", "go", "dirname", "env", "sh", "gcc", "cc", "ld", "as"):
                real = shutil.which(tool)
                if real and not os.path.exists(os.path.join(bin_dir, tool)):
                    os.symlink(real, os.path.join(bin_dir, tool))
            self.assertFalse(os.path.exists(os.path.join(bin_dir, "docker")))
            with open(self.world_path, "w") as f:
                json.dump(base_world(), f)
            open(self.calls_path, "w").close()
            env = dict(os.environ, PATH=bin_dir, STUB_WORLD=self.world_path, STUB_CALLS=self.calls_path, DEPLOY_ENV_FILE=self.env_path)
            p = subprocess.run([shutil.which("bash"), SCRIPT, "--apply"], capture_output=True, text=True, env=env)
            self.assertNotEqual(p.returncode, 0, p.stdout + p.stderr)
            self.assertIn("docker is needed", p.stderr)
            with open(self.calls_path) as f:
                calls = [json.loads(line) for line in f]
            self.assertEqual([c["op"] for c in calls if c["mutating"]], [])


class Permissions(Base):
    def test_a_step_functions_role_that_cannot_run_tasks_is_caught(self):
        w = base_world()
        w["roles"]["indifferent-fun-sfn-role"]["base"] = [{"Statement": [
            {"Effect": "Allow", "Action": ["lambda:InvokeFunction"], "Resource": "*"}]}]
        r = self.run_script("--apply", world=w)
        self.assertNotEqual(r.rc, 0)
        self.assertIn("FAIL allows ecs:RunTask", r.out)
        self.assertNothingChanged(r)

    def test_an_unavailable_simulator_warns_and_goes_on(self):
        w = base_world()
        w["simulator"] = "denied"
        r = self.run_script("--apply", world=w)
        self.assertEqual(r.rc, 0, r.text)
        self.assertIn("NOT checked", r.out)

    def mutant(self, old, new):
        """Runs a copy of the script with one policy line changed, to show the checks can fail."""
        src = open(SCRIPT).read()
        self.assertEqual(src.count(old), 1, "the line to mutate is not unique: %r" % old)
        path = os.path.join(DEPLOY, ".mutant-%d.sh" % os.getpid())
        with open(path, "w") as f:
            f.write(src.replace(old, new))
        self.addCleanup(os.unlink, path)
        return self.run_script("--apply", script=path)

    def test_the_checks_have_teeth_a_task_policy_that_reaches_the_whole_bucket_fails(self):
        r = self.mutant('"arn:aws:s3:::${BUCKET}/${WORK_PREFIX}*",', '"arn:aws:s3:::${BUCKET}/*",')
        self.assertNotEqual(r.rc, 0, r.text)
        self.assertRegex(r.out, r"FAIL denies s3:(Get|Put)Object")
        self.assertNothingChanged(r)

    def test_the_checks_have_teeth_a_task_policy_that_can_delete_videos_fails(self):
        r = self.mutant('"Action": ["s3:GetObject", "s3:PutObject"],\n      "Resource": [', '"Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"],\n      "Resource": [')
        self.assertNotEqual(r.rc, 0, r.text)
        self.assertIn("FAIL denies s3:DeleteObject", r.out)
        self.assertNothingChanged(r)

    def test_the_checks_have_teeth_a_task_policy_missing_the_job_update_fails(self):
        r = self.mutant('"Action": ["dynamodb:GetItem", "dynamodb:UpdateItem"],\n      "Resource": "${TABLE_ARN}"\n    },\n    {\n      "Sid": "Narrate"',
                        '"Action": ["dynamodb:GetItem"],\n      "Resource": "${TABLE_ARN}"\n    },\n    {\n      "Sid": "Narrate"')
        self.assertNotEqual(r.rc, 0, r.text)
        self.assertIn("FAIL allows dynamodb:UpdateItem", r.out)
        self.assertNothingChanged(r)


class StateMachine(Base):
    OLD = json.dumps({"StartAt": "A", "States": {"A": {"Type": "Pass", "End": True}}})

    def test_a_live_definition_that_differs_is_saved_before_it_is_replaced(self):
        w = base_world()
        w["sm"] = self.OLD
        r = self.run_script("--apply", world=w)
        self.assertEqual(r.rc, 0, r.text)
        m = re.search(r"saved the previous definition to (/tmp/\S+\.json)", r.out)
        self.assertTrue(m, r.out)
        with open(m.group(1)) as f:
            self.assertEqual(f.read().strip(), self.OLD)
        self.assertEqual(len(json.loads(self.world["sm"])["States"]), 14)
        self.assertIn("update-state-machine --state-machine-arn", r.out)
        self.assertEqual(r.ops().count("stepfunctions update-state-machine"), 1)
        self.assertNotIn("stepfunctions create-state-machine", r.ops())

    def test_the_definition_is_the_one_the_go_code_renders(self):
        r = self.run_script("--apply")
        self.assertEqual(r.rc, 0, r.text)
        backend = os.path.join(os.path.dirname(DEPLOY), "backend")
        want = subprocess.run(
            ["go", "run", "-buildvcs=false", "./cmd/storypipeline",
             "-cluster-arn", "arn:aws:ecs:ap-south-1:%s:cluster/indifferent-fun-shorts" % ACCT,
             "-task-definition-arn", "arn:aws:ecs:ap-south-1:%s:task-definition/indifferent-fun-story-render" % ACCT,
             "-subnets", ",".join(self.world["subnets"]), "-security-group", "sg-0abc1234",
             "-table", "indifferent-fun-projects", "-notifier-arn", "arn:aws:lambda:ap-south-1:%s:function:indifferent-fun-notifier" % ACCT],
            cwd=backend, capture_output=True, text=True, check=True).stdout
        self.assertEqual(json.loads(self.world["sm"]), json.loads(want))


if __name__ == "__main__":
    unittest.main()
