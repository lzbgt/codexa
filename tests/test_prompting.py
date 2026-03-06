import unittest
from pathlib import Path

from codex_hybrid_autopilot.prompting import build_turn_prompt
from codex_hybrid_autopilot.protocol import AutoReport


class PromptingTests(unittest.TestCase):
    def test_prompt_contains_operator_queue(self) -> None:
        prompt = build_turn_prompt(
            workspace=Path("/tmp/repo"),
            initial_objective="Fix the top task.",
            strategy="hybrid",
            turn_index=1,
            last_report=None,
            pending_user_prompts=["Prioritize the flaky test regression."],
            skill_hint=True,
        )
        self.assertIn("Prioritize the flaky test regression.", prompt)
        self.assertIn("AUTO_REPORT_JSON_BEGIN", prompt)

    def test_prompt_contains_previous_report(self) -> None:
        report = AutoReport.from_obj(
            {
                "auto_mode_next": "continue",
                "summary": "Finished the refactor.",
                "recommended_next_prompt": "Run targeted verification.",
                "user_engagement_needed": False,
                "pending_tasks": [
                    {"priority": "P1", "task": "Run targeted verification.", "status": "pending"}
                ],
                "discovered_tasks": ["Write a regression test."],
                "reweighting_rationale": "Verification comes before additional cleanup.",
                "verification": {"status": "not_run", "summary": "Verification has not run yet."},
            }
        )
        prompt = build_turn_prompt(
            workspace=Path("/tmp/repo"),
            initial_objective="Fix the top task.",
            strategy="hybrid",
            turn_index=2,
            last_report=report,
            pending_user_prompts=[],
            skill_hint=False,
        )
        self.assertIn("Finished the refactor.", prompt)
        self.assertIn("Run targeted verification.", prompt)


if __name__ == "__main__":
    unittest.main()

