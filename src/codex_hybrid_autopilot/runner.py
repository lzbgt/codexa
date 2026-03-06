from __future__ import annotations

import subprocess
from dataclasses import dataclass
from pathlib import Path


@dataclass(slots=True)
class RunResult:
    command: list[str]
    returncode: int
    prompt_path: Path
    last_message_path: Path


def build_codex_command(
    *,
    codex_bin: str,
    profile: str | None,
    workspace: Path,
    last_message_path: Path,
    resume: bool,
    explicit_session_id: str | None,
    extra_root_args: list[str],
    skip_git_repo_check: bool,
) -> list[str]:
    command = [codex_bin]
    if profile:
        command.extend(["-p", profile])
    command.extend(extra_root_args)
    command.append("exec")
    if resume:
        command.append("resume")
        if explicit_session_id:
            command.append(explicit_session_id)
        else:
            command.append("--last")
    if skip_git_repo_check:
        command.append("--skip-git-repo-check")
    command.extend(["-o", str(last_message_path), "-"])
    return command


def run_codex_turn(
    *,
    codex_bin: str,
    profile: str | None,
    workspace: Path,
    prompt: str,
    prompt_path: Path,
    last_message_path: Path,
    resume: bool,
    explicit_session_id: str | None,
    extra_root_args: list[str],
    skip_git_repo_check: bool,
) -> RunResult:
    prompt_path.write_text(prompt, encoding="utf-8")
    command = build_codex_command(
        codex_bin=codex_bin,
        profile=profile,
        workspace=workspace,
        last_message_path=last_message_path,
        resume=resume,
        explicit_session_id=explicit_session_id,
        extra_root_args=extra_root_args,
        skip_git_repo_check=skip_git_repo_check,
    )
    result = subprocess.run(
        command,
        cwd=str(workspace),
        input=prompt,
        text=True,
        check=False,
    )
    return RunResult(
        command=command,
        returncode=result.returncode,
        prompt_path=prompt_path,
        last_message_path=last_message_path,
    )

