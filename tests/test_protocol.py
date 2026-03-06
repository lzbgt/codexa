import unittest

from codex_hybrid_autopilot.protocol import (
    AutoReport,
    ProtocolError,
    extract_report_from_text,
)


GOOD_REPORT = """
Some human-readable summary.
AUTO_REPORT_JSON_BEGIN
{
  "auto_mode_next": "continue",
  "summary": "Implemented the parser fix.",
  "recommended_next_prompt": "Run the remaining parser verification and then move to the next P1 task.",
  "user_engagement_needed": false,
  "pending_tasks": [
    {"priority": "P0", "task": "Finish parser parity verification.", "status": "pending"}
  ],
  "discovered_tasks": ["Add a regression test."],
  "reweighting_rationale": "Verification is still the blocker, so it stays first.",
  "verification": {
    "status": "partial",
    "summary": "Targeted tests passed; the full suite is still pending."
  }
}
AUTO_REPORT_JSON_END
"""


class ProtocolTests(unittest.TestCase):
    def test_extract_report(self) -> None:
        report = extract_report_from_text(GOOD_REPORT)
        self.assertIsInstance(report, AutoReport)
        self.assertEqual(report.auto_mode_next, "continue")
        self.assertEqual(report.pending_tasks[0].priority, "P0")

    def test_missing_block_raises(self) -> None:
        with self.assertRaises(ProtocolError):
            extract_report_from_text("no markers here")


if __name__ == "__main__":
    unittest.main()

