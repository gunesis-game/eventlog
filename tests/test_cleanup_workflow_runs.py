"""Unit tests for repository-wide and per-workflow Actions run retention."""

from runpy import run_path
from unittest.mock import patch
import unittest

cleanup_module = run_path("scripts/cleanup-workflow-runs.py")
runs_to_delete = cleanup_module["runs_to_delete"]
delete_repository_wide_runs = cleanup_module["delete_repository_wide_runs"]


def run(run_id, created_at, conclusion, status="completed"):
    return {"id": run_id, "created_at": created_at, "conclusion": conclusion, "status": status}


class RunRetentionTests(unittest.TestCase):
    def test_only_four_latest_success_or_failure_runs_are_retained(self):
        runs = [
            run(1, "2026-11-24T10:00:00Z", "success"),
            run(2, "2026-11-24T09:00:00Z", "failure"),
            run(3, "2026-11-24T08:00:00Z", "success"),
            run(4, "2026-11-24T07:00:00Z", "failure"),
            run(5, "2026-11-24T06:00:00Z", "success"),
            run(6, "2026-11-24T05:00:00Z", "failure"),
        ]
        self.assertEqual([item["id"] for item in runs_to_delete(runs)], [5, 6])

    def test_skipped_cancelled_and_other_conclusions_are_not_retained(self):
        runs = [
            run(1, "2026-11-24T10:00:00Z", "success"),
            run(2, "2026-11-24T09:00:00Z", "skipped"),
            run(3, "2026-11-24T08:00:00Z", "cancelled"),
            run(4, "2026-11-24T07:00:00Z", "timed_out"),
            run(5, "2026-11-24T06:00:00Z", "failure"),
        ]
        self.assertEqual({item["id"] for item in runs_to_delete(runs)}, {2, 3, 4})

    def test_active_runs_are_never_deleted_or_counted_in_retention(self):
        runs = [
            run(1, "2026-11-24T11:00:00Z", None, "in_progress"),
            run(2, "2026-11-24T10:00:00Z", "success"),
            run(3, "2026-11-24T09:00:00Z", "failure"),
            run(4, "2026-11-24T08:00:00Z", "success"),
            run(5, "2026-11-24T07:00:00Z", "failure"),
            run(6, "2026-11-24T06:00:00Z", "success"),
        ]
        self.assertEqual([item["id"] for item in runs_to_delete(runs)], [5, 6])

    def test_repository_wide_cleanup_deletes_skipped_and_cancelled_across_pages(self):
        pages = [
            ({"workflow_runs": [
                {"id": 10, "name": "Initialize Repository", "conclusion": "cancelled"},
                {"id": 11, "name": "Steering state validation", "conclusion": "success"},
            ]}, '<https://api.github.com/repos/example/project/actions/runs?per_page=100&page=2>; rel="next"'),
            ({"workflow_runs": [
                {"id": 12, "name": "Workflow history cleanup", "conclusion": "skipped"},
                {"id": 13, "name": "Other workflow", "conclusion": "failure"},
            ]}, '<https://api.github.com/repos/example/project/actions/runs?per_page=100&page=3>; rel="next"'),
            ({"workflow_runs": [
                {"id": 14, "name": "Another workflow", "conclusion": "cancelled"},
            ]}, ""),
        ]
        calls = []

        def fake_request(url, token, method="GET"):
            calls.append((url, method))
            if method == "DELETE":
                return None, ""
            return pages.pop(0)

        with patch.dict(delete_repository_wide_runs.__globals__, {"request": fake_request}):
            deleted = delete_repository_wide_runs("example/project", "token")

        self.assertEqual(deleted, [10, 12, 14])
        get_calls = [i for i, (_, method) in enumerate(calls) if method == "GET"]
        delete_calls = [i for i, (_, method) in enumerate(calls) if method == "DELETE"]
        self.assertEqual(len(get_calls), 3)
        self.assertEqual(len(delete_calls), 3)
        self.assertLess(max(get_calls), min(delete_calls), "all pages must be fetched before deleting to avoid pagination shifts")


if __name__ == "__main__":
    unittest.main()
