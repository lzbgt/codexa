from __future__ import annotations

import json
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from .protocol import AutoReport


def utc_now_iso() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat()


@dataclass(slots=True)
class SessionState:
    workspace: str
    initial_objective: str
    strategy: str = "hybrid"
    created_at: str = field(default_factory=utc_now_iso)
    updated_at: str = field(default_factory=utc_now_iso)
    turn_index: int = 0
    session_started: bool = False
    explicit_session_id: str | None = None
    pending_user_prompts: list[str] = field(default_factory=list)
    last_report: dict[str, Any] | None = None
    last_message_path: str | None = None
    last_prompt_path: str | None = None
    operator_notes: list[str] = field(default_factory=list)

    @classmethod
    def from_json(cls, raw: dict[str, Any]) -> "SessionState":
        return cls(
            workspace=str(raw["workspace"]),
            initial_objective=str(raw["initial_objective"]),
            strategy=str(raw.get("strategy", "hybrid")),
            created_at=str(raw.get("created_at", utc_now_iso())),
            updated_at=str(raw.get("updated_at", utc_now_iso())),
            turn_index=int(raw.get("turn_index", 0)),
            session_started=bool(raw.get("session_started", False)),
            explicit_session_id=raw.get("explicit_session_id"),
            pending_user_prompts=list(raw.get("pending_user_prompts", [])),
            last_report=raw.get("last_report"),
            last_message_path=raw.get("last_message_path"),
            last_prompt_path=raw.get("last_prompt_path"),
            operator_notes=list(raw.get("operator_notes", [])),
        )

    def to_json(self) -> dict[str, Any]:
        return {
            "workspace": self.workspace,
            "initial_objective": self.initial_objective,
            "strategy": self.strategy,
            "created_at": self.created_at,
            "updated_at": self.updated_at,
            "turn_index": self.turn_index,
            "session_started": self.session_started,
            "explicit_session_id": self.explicit_session_id,
            "pending_user_prompts": self.pending_user_prompts,
            "last_report": self.last_report,
            "last_message_path": self.last_message_path,
            "last_prompt_path": self.last_prompt_path,
            "operator_notes": self.operator_notes,
        }

    def save(self, path: Path) -> None:
        self.updated_at = utc_now_iso()
        path.write_text(json.dumps(self.to_json(), indent=2) + "\n", encoding="utf-8")

    def absorb_report(self, report: AutoReport) -> None:
        self.last_report = report.to_obj()


def ensure_state_dirs(base_dir: Path) -> dict[str, Path]:
    reports_dir = base_dir / "reports"
    prompts_dir = base_dir / "prompts"
    messages_dir = base_dir / "messages"
    for directory in (base_dir, reports_dir, prompts_dir, messages_dir):
        directory.mkdir(parents=True, exist_ok=True)
    return {
        "base": base_dir,
        "reports": reports_dir,
        "prompts": prompts_dir,
        "messages": messages_dir,
    }


def load_or_create_state(
    state_path: Path,
    workspace: Path,
    initial_objective: str | None,
    strategy: str,
    explicit_session_id: str | None,
) -> SessionState:
    if state_path.exists():
        state = SessionState.from_json(json.loads(state_path.read_text(encoding="utf-8")))
        if initial_objective and not state.initial_objective.strip():
            state.initial_objective = initial_objective.strip()
        if explicit_session_id:
            state.explicit_session_id = explicit_session_id
        if strategy:
            state.strategy = strategy
        return state

    if not initial_objective or not initial_objective.strip():
        raise ValueError("An initial objective is required when creating a new autopilot state.")
    return SessionState(
        workspace=str(workspace),
        initial_objective=initial_objective.strip(),
        strategy=strategy,
        explicit_session_id=explicit_session_id,
    )

