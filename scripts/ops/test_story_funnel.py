"""Tests for story_funnel.py. Nothing here touches AWS.

    python3 -m unittest discover -s scripts/ops -p 'test_story_funnel.py' -v
"""
import contextlib
import io
import json
import os
import re
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import story_funnel as f  # noqa: E402

REAL = {"u%d" % i for i in range(1, 30)}


def plan(user, pid, created="2026-11-11T10:00:00Z", scenes=4, pictured=0, revision=1, picked=0, auto_left=0):
    return {"user": user, "plan": pid, "created": created, "revision": revision, "scenes": scenes,
            "pictured": pictured, "auto_picked": picked, "auto_left": auto_left}


def video(user, pid, status="completed", code="", created="2026-11-11T11:00:00Z"):
    return {"user": user, "plan": pid, "status": status, "code": code, "created": created}


def attempt(user, pid, outcome="refused", code="MISSING_PICTURES"):
    return {"user": user, "plan": pid, "outcome": outcome, "code": code, "time_ms": 1}


class Groups(unittest.TestCase):
    def run_one(self, plans, videos=(), attempts=(), **kw):
        return f.analyse(list(plans), list(videos), list(attempts), REAL, **kw)

    def test_each_user_lands_in_exactly_one_group_in_order(self):
        plans = [
            plan("u1", "p1", pictured=4), plan("u2", "p2", pictured=4), plan("u3", "p3", pictured=4),
            plan("u4", "p4", pictured=4), plan("u5", "p5", pictured=4), plan("u6", "p6", pictured=4),
            plan("u7", "p7", pictured=2), plan("u8", "p8", pictured=0),
        ]
        videos = [
            video("u1", "p1", "completed"),
            video("u2", "p2", "failed", "MEDIA_TEMPORARY"),
            video("u3", "p3", "failed", "CANCELLED"),
            video("u4", "p4", "rendering"),
        ]
        attempts = [attempt("u5", "p5", "refused", "STALE_REVISION")]
        r = self.run_one(plans, videos, attempts, attempts_visible=True)
        self.assertEqual(r["groups"], {
            "made_video": 1, "started_failed": 1, "started_cancelled": 1, "in_progress": 1,
            "clicked_refused": 1, "never_clicked_ready": 1, "blocked_some": 1, "blocked_none": 1,
        })
        self.assertEqual(sum(r["groups"].values()), r["users_with_ready_plan"])
        self.assertEqual(r["users_who_started_a_video"], 4)
        self.assertEqual(r["users_who_never_started"], 4)
        self.assertEqual(r["started_share"], 50.0)

    def test_a_finished_video_beats_a_failed_or_cancelled_one(self):
        plans = [plan("u1", "p1", pictured=4), plan("u1", "p2", pictured=4)]
        videos = [video("u1", "p1", "failed", "INTERNAL"), video("u1", "p2", "completed")]
        r = self.run_one(plans, videos)
        self.assertEqual(r["groups"]["made_video"], 1)
        self.assertEqual(r["groups"]["started_failed"], 0)

    def test_the_users_own_cancel_is_not_a_failure_but_a_failure_next_to_it_is(self):
        plans = [plan("u1", "p1", pictured=4), plan("u2", "p2", pictured=4)]
        videos = [
            video("u1", "p1", "failed", "CANCELLED"),
            video("u2", "p2", "failed", "CANCELLED"), video("u2", "p2", "failed", "TIMED_OUT"),
        ]
        r = self.run_one(plans, videos)
        self.assertEqual((r["groups"]["started_cancelled"], r["groups"]["started_failed"]), (1, 1))

    def test_a_refused_click_counts_only_when_no_job_was_made(self):
        plans = [plan("u1", "p1", pictured=4), plan("u2", "p2", pictured=4)]
        videos = [video("u1", "p1", "completed")]
        attempts = [attempt("u1", "p1"), attempt("u2", "p2", "error", "INTERNAL_ERROR")]
        r = self.run_one(plans, videos, attempts)
        self.assertEqual((r["groups"]["made_video"], r["groups"]["clicked_refused"]), (1, 1))

    def test_an_attempt_on_a_plan_outside_the_window_is_ignored(self):
        plans = [plan("u1", "p1", pictured=4)]
        r = self.run_one(plans, [], [attempt("u1", "other-plan")])
        self.assertEqual((r["groups"]["clicked_refused"], r["groups"]["never_clicked_ready"]), (0, 1))
        self.assertEqual(r["start_attempts"]["refused_or_error"], 0)

    def test_started_attempts_are_not_refusals_and_codes_are_counted(self):
        plans = [plan("u1", "p1", pictured=4)]
        attempts = [attempt("u1", "p1", "started", "-"), attempt("u1", "p1", "refused", "RATE_LIMITED"),
                    attempt("u1", "p1", "refused", "RATE_LIMITED"), attempt("u1", "p1", "error", "INTERNAL_ERROR")]
        r = self.run_one(plans, [], attempts)["start_attempts"]
        self.assertEqual((r["started"], r["refused_or_error"]), (1, 3))
        self.assertEqual(r["by_code"], {"INTERNAL_ERROR": 1, "RATE_LIMITED": 2})

    def test_the_best_plan_decides_ready_or_blocked_and_a_tie_goes_to_the_latest(self):
        plans = [plan("u1", "p1", pictured=0), plan("u1", "p2", pictured=4, created="2026-11-11T12:00:00Z"),
                 plan("u2", "p3", pictured=1), plan("u2", "p4", pictured=3)]
        r = self.run_one(plans)
        self.assertEqual((r["groups"]["never_clicked_ready"], r["groups"]["blocked_some"]), (1, 1))
        self.assertEqual(r["never_started_with_one_plan"], 0)

    def test_a_plan_with_no_scenes_is_never_ready(self):
        r = self.run_one([plan("u1", "p1", scenes=0, pictured=0)])
        self.assertEqual(r["groups"]["blocked_none"], 1)

    def test_test_accounts_and_unknown_users_are_left_out(self):
        plans = [plan("u1", "p1"), plan("test-user", "p2"), plan("ghost", "p3")]
        r = self.run_one(plans)
        self.assertEqual((r["users_with_ready_plan"], r["plans_ready"]), (1, 1))


class Windows(unittest.TestCase):
    def test_since_is_inclusive_until_is_exclusive_and_times_work(self):
        plans = [plan("u1", "a", created="2026-11-10T23:59:59Z"), plan("u2", "b", created="2026-11-11T00:00:00Z"),
                 plan("u3", "c", created="2026-11-11T15:09:59Z"), plan("u4", "d", created="2026-11-11T15:10:00Z"),
                 plan("u5", "e", created="")]
        r = f.analyse(plans, [], [], REAL, since="2026-11-11", until="2026-11-11T15:10")
        self.assertEqual(r["plans_ready"], 2)
        self.assertEqual(f.analyse(plans, [], [], REAL, until="2026-11-11")["plans_ready"], 1)
        self.assertEqual(f.analyse(plans, [], [], REAL)["plans_ready"], 4)

    def test_a_plan_created_exactly_at_until_is_left_out_and_exactly_at_since_is_in(self):
        plans = [plan("u1", "a", created="2026-11-11T15:10"), plan("u2", "b", created="2026-11-11T15:11")]
        self.assertEqual(f.analyse(plans, [], [], REAL, until="2026-11-11T15:10")["plans_ready"], 0)
        self.assertEqual(f.analyse(plans, [], [], REAL, since="2026-11-11T15:10", until="2026-11-11T15:11")["plans_ready"], 1)

    def test_as_of_ignores_videos_started_later(self):
        plans = [plan("u1", "p1", pictured=4)]
        videos = [video("u1", "p1", created="2026-11-12T09:00:00Z")]
        later = f.analyse(plans, videos, [], REAL)
        then = f.analyse(plans, videos, [], REAL, as_of="2026-11-12T00:00")
        self.assertEqual((later["users_who_started_a_video"], then["users_who_started_a_video"]), (1, 0))
        self.assertEqual(then["groups"]["never_clicked_ready"], 1)

    def test_only_videos_of_plans_in_the_window_count(self):
        plans = [plan("u1", "p1", created="2026-11-11T10:00:00Z"), plan("u1", "p2", created="2026-11-20T10:00:00Z")]
        videos = [video("u1", "p2")]
        r = f.analyse(plans, videos, [], REAL, until="2026-11-12")
        self.assertEqual((r["users_who_started_a_video"], r["plans_that_became_a_video"]), (0, 0))

    def test_an_empty_window_divides_by_nothing(self):
        r = f.analyse([], [], [], REAL)
        self.assertIsNone(r["started_share"])
        self.assertIn("Users with a ready plan: 0", f.render(r))


class Numbers(unittest.TestCase):
    def test_auto_pick_counts(self):
        plans = [plan("u1", "p1", scenes=6, pictured=6, picked=6, auto_left=4),
                 plan("u2", "p2", scenes=5, pictured=3, picked=4, auto_left=4),
                 plan("u3", "p3", scenes=5, pictured=0)]
        a = f.analyse(plans, [], [], REAL)["auto_pick"]
        self.assertEqual(a, {"plans": 2, "pictures_picked": 10, "scenes_in_those_plans": 11, "still_auto": 8, "swapped_or_removed": 2})

    def test_swapped_or_removed_is_never_negative(self):
        a = f.analyse([plan("u1", "p1", picked=1, auto_left=3)], [], [], REAL)["auto_pick"]
        self.assertEqual(a["swapped_or_removed"], 0)

    def test_baseline_style_facts(self):
        plans = [plan("u1", "p1", pictured=4, revision=2), plan("u2", "p2", pictured=0), plan("u3", "p3", pictured=0),
                 plan("u3", "p4", pictured=2, revision=3)]
        videos = [video("u1", "p1")]
        r = f.analyse(plans, videos, [], REAL)
        self.assertEqual(r["plans_saved_after_creation"], 2)
        self.assertEqual((r["plans_untouched"], r["plans_untouched_with_no_picture"]), (2, 2))
        self.assertEqual((r["plans_that_became_a_video"], r["plans_that_became_a_video_with_every_scene_pictured"]), (1, 1))
        self.assertEqual((r["users_who_never_started"], r["never_started_with_one_plan"]), (2, 1))


class Output(unittest.TestCase):
    def sample(self, visible):
        plans = [plan("u1", "plan-aaa", pictured=4), plan("u2", "plan-bbb")]
        return f.analyse(plans, [video("u1", "plan-aaa")], [attempt("u2", "plan-bbb")], REAL, attempts_visible=visible)

    def test_no_id_appears_in_the_text_or_the_json(self):
        r = self.sample(True)
        for out in (f.render(r), json.dumps(r)):
            for secret in ("u1", "u2", "plan-aaa", "plan-bbb"):
                self.assertNotIn(secret, out)

    def test_it_says_when_start_attempts_cannot_be_seen(self):
        self.assertIn("NOT visible", f.render(self.sample(False)))
        self.assertNotIn("NOT visible", f.render(self.sample(True)))

    def test_the_text_lists_every_group(self):
        text = f.render(self.sample(True))
        for _, label in f.GROUPS:
            self.assertIn(label, text)

    def test_no_em_dash(self):
        self.assertNotIn("\u2014", f.render(self.sample(True)))


class Parsing(unittest.TestCase):
    def test_a_start_line(self):
        line = "2026/11/13 17:20:22 story_video_start user=u1 plan=01a1d5eb outcome=refused code=MISSING_PICTURES"
        self.assertEqual(f.parse_start_line(line), {"user": "u1", "plan": "01a1d5eb", "outcome": "refused", "code": "MISSING_PICTURES"})

    def test_lines_that_are_not_start_lines(self):
        for line in ("story_video_start_failed user=u1 job=j err=x", "storyplan_ok model=x", ""):
            self.assertIsNone(f.parse_start_line(line))

    def test_dates_and_times(self):
        self.assertEqual(f.to_ms("2026-11-11"), 1794355200000)
        self.assertEqual(f.to_ms("2026-11-11T15:10") - f.to_ms("2026-11-11"), (15 * 60 + 10) * 60 * 1000)
        with self.assertRaises(ValueError):
            f.to_ms("11/11/2026")

    def test_a_bad_date_is_refused_before_anything_is_read(self):
        err = io.StringIO()
        with contextlib.redirect_stderr(err), self.assertRaises(SystemExit) as cm:
            f.main(["--until", "yesterday"])
        self.assertEqual(cm.exception.code, 2)
        self.assertIn("--until must be a UTC date", err.getvalue())


class FakeDB:
    """A DynamoDB client double that pages and records what it was asked."""

    def __init__(self, pages):
        self.pages, self.calls = list(pages), []

    def scan(self, **kw):
        self.calls.append(kw)
        page = self.pages.pop(0)
        out = {"Items": page}
        if self.pages:
            out["LastEvaluatedKey"] = {"PK": {"S": "x"}}
        return out


def S(v):
    return {"S": v}


class Reading(unittest.TestCase):
    def test_a_plan_is_reduced_to_counts_and_its_text_is_dropped(self):
        body = json.dumps({"scenes": [
            {"narration": "SECRET STORY", "keywords": ["secretword"], "media": {"id": "1"}, "mediaSource": "auto"},
            {"narration": "more", "keywords": ["x"], "media": {"id": "2"}},
            {"narration": "none", "keywords": ["y"]},
        ]})
        db = FakeDB([[{"PK": S("USER#u1"), "SK": S("STORYPLAN#p1"), "status": S("ready"), "revision": {"N": "3"},
                       "plan": S(body), "autoPick": S('{"picked":2,"empty":1,"ms":900}'), "createdAt": S("2026-11-13T10:00:00Z")}]])
        got = f.read_plans(db, "t")
        self.assertEqual(got, [{"user": "u1", "plan": "p1", "created": "2026-11-13T10:00:00Z", "revision": 3,
                                "scenes": 3, "pictured": 2, "auto_picked": 2, "auto_left": 1}])
        self.assertNotIn("SECRET", json.dumps(got))
        call = db.calls[0]
        self.assertEqual(call["ExpressionAttributeNames"], {"#s": "status", "#pl": "plan"})
        self.assertNotRegex(call["ProjectionExpression"], r"\b(status|plan)\b")

    def test_a_plan_that_does_not_decode_is_counted_with_no_scenes_not_skipped_or_guessed(self):
        db = FakeDB([[{"PK": S("USER#u1"), "SK": S("STORYPLAN#p1"), "plan": S("{not json"), "autoPick": S("garbage")}]])
        got = f.read_plans(db, "t")
        self.assertEqual((got[0]["scenes"], got[0]["pictured"], got[0]["auto_picked"]), (0, 0, 0))

    def test_videos_are_read_without_their_scenes(self):
        db = FakeDB([[{"PK": S("USER#u1"), "SK": S("STORYVIDEO#j1"), "planId": S("p1"), "status": S("failed"),
                       "errorCode": S("CANCELLED"), "createdAt": S("t")}]])
        self.assertEqual(f.read_videos(db, "t"), [{"user": "u1", "plan": "p1", "status": "failed", "code": "CANCELLED", "created": "t"}])
        self.assertNotIn("scenes", db.calls[0]["ProjectionExpression"])

    def test_scans_follow_every_page(self):
        page = lambda i: [{"PK": S("USER#u%d" % i), "SK": S("STORYVIDEO#j"), "planId": S("p"), "status": S("completed")}]
        db = FakeDB([page(1), page(2), page(3)])
        self.assertEqual(len(f.read_videos(db, "t")), 3)
        self.assertEqual(len(db.calls), 3)

    def test_test_accounts_are_found_by_their_domain_and_never_by_name(self):
        db = FakeDB([[{"PK": S("USER#u1"), "email": S("a@gmail.com")}, {"PK": S("USER#u2"), "email": S("T@Example.COM")}]])
        self.assertEqual(f.read_real_users(db, "t"), {"u1"})


class ReadOnly(unittest.TestCase):
    def test_the_script_calls_no_write_api(self):
        with open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "story_funnel.py")) as fh:
            src = fh.read()
        self.assertIsNone(re.search(r"put_item|update_item|delete_item|batch_write|transact_write|put_log_events|\.(put|delete|create)_", src))
        calls = set(re.findall(r"\b(?:db|logs)\.([a-z_]+)\(", src))
        self.assertEqual(calls, {"scan", "filter_log_events"})


if __name__ == "__main__":
    unittest.main()
