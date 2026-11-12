"""Tests for deploy/setup-story-metrics.sh, run against the same stand-in for the
aws command as the other deploy tests. Nothing here touches AWS.
    python3 -m unittest discover -s deploy/tests -v

The script renders the dashboard with the real Go code (backend/cmd/storydashboard),
so what is checked here is the body that would be uploaded. What this does not
prove: that CloudWatch accepts that body (the stand-in checks it is JSON with a
widgets list, not CloudWatch's schema), or that the metrics it reads exist in a real
account. The script's dry run is read-only so that it can be the first real check.
"""
import json
import os
import re
import unittest

from test_setup_storyvideo import DEPLOY, Base, base_world

SCRIPT = os.path.join(DEPLOY, "setup-story-metrics.sh")
NAME = "indifferent-fun-story"
ALARM = "indifferent-fun-story-plans-failing"
TOPIC = "arn:aws:sns:ap-south-1:%s:indifferent-fun-email-alerts"


def render_expected():
    """The body the script should upload: the Go renderer's own output."""
    import subprocess

    backend = os.path.join(os.path.dirname(DEPLOY), "backend")
    out = subprocess.run(["go", "run", "-buildvcs=false", "./cmd/storydashboard", "-region", "ap-south-1"],
                         cwd=backend, capture_output=True, text=True, check=True).stdout
    return json.loads(out)


EXPECTED = None


class Metrics(Base):
    @classmethod
    def setUpClass(cls):
        global EXPECTED
        EXPECTED = render_expected()

    def run_metrics(self, *args, **kw):
        # The script runs `go`, which the stand-in does not replace: the real renderer is the point.
        return self.run_script(*args, script=SCRIPT, **kw)

    def board(self):
        raw = self.world.get("dashboards", {}).get(NAME)
        return json.loads(raw) if raw else None

    def test_a_dry_run_renders_reads_and_writes_nothing(self):
        r = self.run_metrics()
        self.assertEqual(r.rc, 0, r.text)
        self.assertNothingChanged(r)
        self.assertIn("MODE: dry run", r.text)
        self.assertIn("Rendered %s: 9 widgets, ap-south-1." % NAME, r.text)
        self.assertIn("does not exist; it will be created", r.text)
        self.assertIn("[dry-run] would: write the dashboard %s" % NAME, r.text)
        self.assertIsNone(self.board())
        self.assertIn("[dry-run] would: put the alarm %s" % ALARM, r.text)
        self.assertEqual([c["op"] for c in r.calls],
                         ["cloudwatch get-dashboard", "sts get-caller-identity", "sns get-topic-attributes", "cloudwatch describe-alarms"])
        self.assertEqual(self.world.get("alarms", {}), {})

    def test_apply_from_nothing_uploads_exactly_the_rendered_dashboard(self):
        r = self.run_metrics("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertEqual(r.ops(), ["cloudwatch put-dashboard", "cloudwatch put-metric-alarm"])
        self.assertEqual(self.board(), EXPECTED)
        self.assertIn("CloudWatch accepted the dashboard.", r.text)
        put = [c for c in r.calls if c["op"] == "cloudwatch put-dashboard"][0]
        self.assertEqual(put["argv"][put["argv"].index("--region") + 1], "ap-south-1")
        self.assertEqual(put["argv"][put["argv"].index("--dashboard-name") + 1], NAME)

    def test_running_it_again_writes_nothing(self):
        self.assertEqual(self.run_metrics("--apply").rc, 0)
        r = self.run_metrics("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertNothingChanged(r)
        self.assertIn("already up to date", r.text)

    def test_a_different_dashboard_is_saved_before_it_is_replaced(self):
        old = {"widgets": [{"type": "text", "x": 0, "y": 0, "width": 6, "height": 3, "properties": {"markdown": "someone's edit"}}]}
        self.world.setdefault("dashboards", {})[NAME] = json.dumps(old)
        r = self.run_metrics("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertEqual(self.board(), EXPECTED)
        m = re.search(r"saved the previous dashboard body to (/tmp/\S+\.json)", r.text)
        self.assertIsNotNone(m, r.text)
        self.backups.append(m.group(1))
        with open(m.group(1)) as f:
            self.assertEqual(json.load(f), old, "the backup is not what was there")
        self.assertIn("undo: aws cloudwatch put-dashboard", r.text)
        self.assertIn("file://" + m.group(1), r.text)

    def test_a_dry_run_says_what_it_would_replace_and_saves_nothing(self):
        self.world.setdefault("dashboards", {})[NAME] = json.dumps({"widgets": []})
        r = self.run_metrics()
        self.assertEqual(r.rc, 0, r.text)
        self.assertNothingChanged(r)
        self.assertIn("exists and differs", r.text)
        self.assertNotIn("saved the previous", r.text)
        self.assertEqual(json.loads(self.world["dashboards"][NAME]), {"widgets": []})

    def test_warnings_are_shown_and_do_not_fail_the_script(self):
        self.world["dashboard_warnings"] = [{"DataPath": "/widgets/2/properties/metrics", "Message": "Metric may not render"}]
        r = self.run_metrics("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertIn("warning: /widgets/2/properties/metrics: Metric may not render", r.text)
        self.assertIn("with 1 warning(s)", r.text)

    def test_errors_stop_the_script_with_a_non_zero_exit(self):
        self.world["dashboard_errors"] = ["/widgets/0: unknown widget type"]
        r = self.run_metrics("--apply")
        self.assertNotEqual(r.rc, 0, r.text)
        self.assertIn("The dashboard body is invalid", r.err)
        self.assertNotIn("Done.", r.out)
        self.assertIsNone(self.board())

    def test_a_read_failure_other_than_not_found_is_refused_and_nothing_is_written(self):
        # Treating an unreadable dashboard as missing would overwrite it unseen.
        self.world["dashboard_read_error"] = True
        r = self.run_metrics("--apply")
        self.assertNotEqual(r.rc, 0, r.text)
        self.assertIn("could not read the dashboard", r.err)
        self.assertNothingChanged(r)

    def test_the_only_writes_are_the_dashboard_and_the_alarm_and_nothing_is_ever_deleted(self):
        for args in ([], ["--apply"], ["--apply"]):
            r = self.run_metrics(*args)
            for c in r.calls:
                self.assertNotRegex(c["op"], r"delete|remove|deregister", c["argv"])
            self.assertLessEqual(len(r.changes), 2)
            for c in r.changes:
                self.assertIn(c["op"], ("cloudwatch put-dashboard", "cloudwatch put-metric-alarm"))
            for c in r.changes:
                if c["op"] == "cloudwatch put-metric-alarm":
                    self.assertEqual(json.loads(list(c["files"].values())[0])["AlarmName"], ALARM)

    def test_an_unknown_argument_is_refused_before_aws_is_asked_anything(self):
        r = self.run_metrics("--aply")
        self.assertEqual(r.rc, 1)
        self.assertEqual(r.calls, [])

    def test_the_dashboard_it_uploads_reads_only_metrics_the_code_emits(self):
        # The Go tests check the renderer; this checks what the script put in the account.
        self.assertEqual(self.run_metrics("--apply").rc, 0)
        names = set()
        for w in self.board()["widgets"]:
            if w["type"] != "metric":
                continue
            for row in w["properties"]["metrics"]:
                if isinstance(row[0], str):
                    self.assertEqual(row[0], "Indifferent/Story")
                    names.add(row[1])
        self.assertGreaterEqual(len(names), 12)
        self.assertLessEqual(names, {
            "PlansReady", "PlansFailed", "VideosStarted", "VideosCompleted", "VideosFailed", "VideosCancelled", "VideoSeconds",
            "StockSearches", "StockSearchesEmpty", "PicturesChosen", "PicturesReplaced", "PicturesRemoved", "NarrationsEdited"})

    # --- the story-plans-failing alarm

    def alarm(self):
        return self.world.get("alarms", {}).get(ALARM)

    def test_the_alarm_fires_on_four_failures_that_outnumber_successes_and_mails_the_alerts_topic(self):
        r = self.run_metrics("--apply")
        self.assertEqual(r.rc, 0, r.text)
        a = self.alarm()
        self.assertIsNotNone(a, r.text)
        self.assertEqual(a["AlarmActions"], [TOPIC % self.world["account"]])
        ids = {m["Id"]: m for m in a["Metrics"]}
        self.assertEqual(set(ids), {"failed", "ready", "failing"})
        for id_, metric in (("failed", "PlansFailed"), ("ready", "PlansReady")):
            stat = ids[id_]["MetricStat"]
            self.assertEqual(stat["Metric"], {"Namespace": "Indifferent/Story", "MetricName": metric, "Dimensions": []})
            self.assertEqual((stat["Period"], stat["Stat"]), (900, "Sum"))
            self.assertFalse(ids[id_]["ReturnData"])
        self.assertEqual(ids["failing"]["Expression"], "IF(FILL(failed,0) >= 4 AND FILL(failed,0) >= FILL(ready,0), 1, 0)")
        self.assertTrue(ids["failing"]["ReturnData"])
        self.assertEqual((a["Threshold"], a["ComparisonOperator"]), (1, "GreaterThanOrEqualToThreshold"))
        self.assertEqual((a["EvaluationPeriods"], a["DatapointsToAlarm"]), (2, 1))
        self.assertEqual(a["TreatMissingData"], "notBreaching")
        self.assertIn("undo: aws cloudwatch delete-alarms --alarm-names %s" % ALARM, r.text)

    def test_the_alarm_is_not_written_again_when_it_is_already_right(self):
        self.assertEqual(self.run_metrics("--apply").rc, 0)
        r = self.run_metrics("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertNothingChanged(r)
        self.assertIn("The alarm %s is already up to date." % ALARM, r.text)

    def test_an_alarm_that_differs_is_replaced(self):
        self.assertEqual(self.run_metrics("--apply").rc, 0)
        self.world = json.load(open(self.world_path))
        self.world["alarms"][ALARM]["Threshold"] = 5
        r = self.run_metrics("--apply")
        self.assertEqual(r.rc, 0, r.text)
        self.assertEqual(r.ops(), ["cloudwatch put-metric-alarm"])
        self.assertIn("exists and differs; it will be replaced", r.text)
        self.assertEqual(self.alarm()["Threshold"], 1)

    def test_without_the_alerts_topic_nothing_is_written(self):
        self.world["topics"] = []
        r = self.run_metrics("--apply")
        self.assertNotEqual(r.rc, 0, r.text)
        self.assertIn("the alerts topic", r.err)
        self.assertNothingChanged(r)
        self.assertIsNone(self.board())


if __name__ == "__main__":
    unittest.main()
