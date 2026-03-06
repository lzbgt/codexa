from __future__ import annotations

from pathlib import Path

from .protocol import AutoReport, protocol_instructions


def _format_pending_tasks(report: AutoReport | None) -> str:
    if not report or not report.pending_tasks:
        return "- None reported yet."
    lines = []
    for task in report.pending_tasks[:10]:
        lines.append(f"- {task.priority} [{task.status}] {task.task}")
    return "\n".join(lines)


def _format_discovered_tasks(report: AutoReport | None) -> str:
    if not report or not report.discovered_tasks:
        return "- None."
    return "\n".join(f"- {task}" for task in report.discovered_tasks[:10])


def _format_user_prompts(pending_user_prompts: list[str]) -> str:
    if not pending_user_prompts:
        return "- None queued by the operator."
    return "\n".join(
        f"- Operator prompt {index}: {prompt}"
        for index, prompt in enumerate(pending_user_prompts, start=1)
    )


def _mode_block(strategy: str, workspace: Path) -> str:
    return "\n".join(
        [
            "Wrapper mode:",
            f"- Strategy: {strategy}",
            f"- Workspace: {workspace}",
            "- An external wrapper is orchestrating turn boundaries.",
            "- Keep all work grounded in repo state, explicit user prompts, and concrete verification results.",
            "- Do not invent follow-up work unless it is directly implied by current evidence.",
        ]
    )


def build_turn_prompt(
    *,
    workspace: Path,
    initial_objective: str,
    strategy: str,
    turn_index: int,
    last_report: AutoReport | None,
    pending_user_prompts: list[str],
    skill_hint: bool,
) -> str:
    lines: list[str] = []
    if skill_hint:
        lines.append(
            "Use $codex-session-autopilot if it is installed; otherwise follow the autopilot protocol described below exactly."
        )
    lines.extend(
        [
            _mode_block(strategy, workspace),
            "",
            f"Session objective: {initial_objective}",
            f"Turn index: {turn_index}",
            "",
        ]
    )

    if turn_index == 1:
        lines.extend(
            [
                "Initial turn requirements:",
                "- Build or update a concise task tracker if the workspace needs one.",
                "- Reweight work based on the highest-leverage concrete task.",
                "- Verify the changed area before finalizing.",
                "",
            ]
        )
    else:
        lines.extend(
            [
                "Previous turn summary:",
                f"- Summary: {last_report.summary if last_report else 'Unavailable.'}",
                f"- Verification: {last_report.verification_summary if last_report else 'Unavailable.'}",
                f"- Reweighting rationale: {last_report.reweighting_rationale if last_report else 'Unavailable.'}",
                "",
                "Pending tasks from the previous turn:",
                _format_pending_tasks(last_report),
                "",
                "Newly discovered tasks from the previous turn:",
                _format_discovered_tasks(last_report),
                "",
            ]
        )
    lines.extend(
        [
            "Queued operator prompts for this new turn:",
            _format_user_prompts(pending_user_prompts),
            "",
            "Execution requirements:",
            "- Reweight tasks using the operator prompts first, then the remaining technical blockers.",
            "- Execute the highest-leverage concrete task; do not only restate a plan.",
            "- Update or create a concise task tracker when it helps continuity.",
            "- Verify the changed area before finalizing.",
            "- If you stop, it must be because no concrete task remains or operator input is required.",
            "",
            protocol_instructions(),
        ]
    )
    return "\n".join(lines).strip() + "\n"

