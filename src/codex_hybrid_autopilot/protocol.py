from __future__ import annotations

import json
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any

BEGIN_MARKER = "AUTO_REPORT_JSON_BEGIN"
END_MARKER = "AUTO_REPORT_JSON_END"
VALID_AUTO_MODES = {"continue", "stop"}
VALID_VERIFICATION_STATUSES = {"passed", "failed", "partial", "not_run"}
VALID_TASK_PRIORITIES = {"P0", "P1", "P2"}
VALID_TASK_STATUSES = {"pending", "in_progress", "blocked", "done"}


class ProtocolError(ValueError):
    """Raised when the Codex report is missing or malformed."""


@dataclass(slots=True)
class AutoTask:
    priority: str
    task: str
    status: str

    @classmethod
    def from_obj(cls, raw: Any) -> "AutoTask":
        if not isinstance(raw, dict):
            raise ProtocolError("Each pending task must be an object.")
        priority = str(raw.get("priority", "")).strip()
        task = str(raw.get("task", "")).strip()
        status = str(raw.get("status", "")).strip()
        if priority not in VALID_TASK_PRIORITIES:
            raise ProtocolError(f"Invalid task priority: {priority!r}")
        if status not in VALID_TASK_STATUSES:
            raise ProtocolError(f"Invalid task status: {status!r}")
        if not task:
            raise ProtocolError("Task descriptions must be non-empty.")
        return cls(priority=priority, task=task, status=status)

    def to_obj(self) -> dict[str, str]:
        return {
            "priority": self.priority,
            "task": self.task,
            "status": self.status,
        }


@dataclass(slots=True)
class AutoReport:
    auto_mode_next: str
    summary: str
    recommended_next_prompt: str
    user_engagement_needed: bool
    pending_tasks: list[AutoTask]
    discovered_tasks: list[str]
    reweighting_rationale: str
    verification_status: str
    verification_summary: str

    @classmethod
    def from_obj(cls, raw: Any) -> "AutoReport":
        if not isinstance(raw, dict):
            raise ProtocolError("Auto report payload must be a JSON object.")
        auto_mode_next = str(raw.get("auto_mode_next", "")).strip()
        summary = str(raw.get("summary", "")).strip()
        recommended_next_prompt = str(raw.get("recommended_next_prompt", "")).strip()
        user_engagement_needed = bool(raw.get("user_engagement_needed", False))
        reweighting_rationale = str(raw.get("reweighting_rationale", "")).strip()

        verification = raw.get("verification", {})
        if not isinstance(verification, dict):
            raise ProtocolError("'verification' must be an object.")
        verification_status = str(verification.get("status", "")).strip()
        verification_summary = str(verification.get("summary", "")).strip()

        if auto_mode_next not in VALID_AUTO_MODES:
            raise ProtocolError(
                f"'auto_mode_next' must be one of {sorted(VALID_AUTO_MODES)}."
            )
        if verification_status not in VALID_VERIFICATION_STATUSES:
            raise ProtocolError(
                "'verification.status' must be one of "
                f"{sorted(VALID_VERIFICATION_STATUSES)}."
            )
        if not summary:
            raise ProtocolError("'summary' must be non-empty.")
        if not recommended_next_prompt:
            raise ProtocolError("'recommended_next_prompt' must be non-empty.")
        if not reweighting_rationale:
            raise ProtocolError("'reweighting_rationale' must be non-empty.")
        if not verification_summary:
            raise ProtocolError("'verification.summary' must be non-empty.")

        pending_tasks_raw = raw.get("pending_tasks", [])
        if not isinstance(pending_tasks_raw, list):
            raise ProtocolError("'pending_tasks' must be an array.")
        discovered_tasks_raw = raw.get("discovered_tasks", [])
        if not isinstance(discovered_tasks_raw, list):
            raise ProtocolError("'discovered_tasks' must be an array.")

        pending_tasks = [AutoTask.from_obj(item) for item in pending_tasks_raw]
        discovered_tasks = [str(item).strip() for item in discovered_tasks_raw if str(item).strip()]

        return cls(
            auto_mode_next=auto_mode_next,
            summary=summary,
            recommended_next_prompt=recommended_next_prompt,
            user_engagement_needed=user_engagement_needed,
            pending_tasks=pending_tasks,
            discovered_tasks=discovered_tasks,
            reweighting_rationale=reweighting_rationale,
            verification_status=verification_status,
            verification_summary=verification_summary,
        )

    def to_obj(self) -> dict[str, Any]:
        return {
            "auto_mode_next": self.auto_mode_next,
            "summary": self.summary,
            "recommended_next_prompt": self.recommended_next_prompt,
            "user_engagement_needed": self.user_engagement_needed,
            "pending_tasks": [task.to_obj() for task in self.pending_tasks],
            "discovered_tasks": self.discovered_tasks,
            "reweighting_rationale": self.reweighting_rationale,
            "verification": {
                "status": self.verification_status,
                "summary": self.verification_summary,
            },
        }


def protocol_instructions() -> str:
    example = {
        "auto_mode_next": "continue",
        "summary": "Implemented the highest-priority parser fix and reran the targeted tests.",
        "recommended_next_prompt": "Inspect docs/TODOS.md, then finish the remaining P0 task and rerun the targeted verification before widening scope.",
        "user_engagement_needed": False,
        "pending_tasks": [
            {"priority": "P0", "task": "Fix the remaining parser parity failure.", "status": "pending"},
            {"priority": "P1", "task": "Add a regression test for the parity case.", "status": "pending"},
        ],
        "discovered_tasks": ["Document the new parser edge case in docs/TODOS.md."],
        "reweighting_rationale": "The parser parity failure still blocks broader verification, so it remains above the regression test.",
        "verification": {
            "status": "partial",
            "summary": "Targeted parser tests passed; full parity suite still has one failure.",
        },
    }
    return "\n".join(
        [
            "At the end of your final response, append a machine-readable JSON report.",
            f"Write the exact begin marker on its own line: {BEGIN_MARKER}",
            "Then write valid JSON matching the required shape.",
            f"Write the exact end marker on its own line: {END_MARKER}",
            "Do not omit the report even if you decide to stop.",
            "Required JSON shape:",
            json.dumps(example, indent=2),
        ]
    )


def extract_report_from_text(text: str) -> AutoReport:
    pattern = re.compile(
        rf"{re.escape(BEGIN_MARKER)}\s*(\{{.*?\}})\s*{re.escape(END_MARKER)}",
        re.DOTALL,
    )
    match = pattern.search(text)
    if not match:
        raise ProtocolError(
            "Could not find an AUTO_REPORT_JSON block in the last Codex message."
        )
    try:
        payload = json.loads(match.group(1))
    except json.JSONDecodeError as exc:
        raise ProtocolError(f"Invalid JSON inside auto report: {exc}") from exc
    return AutoReport.from_obj(payload)


def write_report(path: Path, report: AutoReport) -> None:
    path.write_text(json.dumps(report.to_obj(), indent=2) + "\n", encoding="utf-8")

