from __future__ import annotations

import argparse
import select
import shlex
import sys
from pathlib import Path

from .prompting import build_turn_prompt
from .protocol import AutoReport, ProtocolError, extract_report_from_text, write_report
from .runner import run_codex_turn
from .state import ensure_state_dirs, load_or_create_state


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        prog="codex-hybrid-autopilot",
        description="Visible hybrid autopilot wrapper for the official Codex CLI.",
    )
    parser.add_argument(
        "--workspace",
        default=".",
        help="Target workspace where Codex should run. Defaults to the current directory.",
    )
    parser.add_argument(
        "--objective",
        help="Initial objective for a new session. Required when no prior state exists.",
    )
    parser.add_argument(
        "--strategy",
        choices=["hybrid", "stateless"],
        default="hybrid",
        help="Use `hybrid` to resume the previous Codex session, or `stateless` to start a fresh exec turn each time.",
    )
    parser.add_argument(
        "--codex-bin",
        default="codex",
        help="Path to the Codex CLI binary. Defaults to `codex` from PATH.",
    )
    parser.add_argument(
        "--profile",
        default="yolo",
        help="Codex profile to use. Defaults to `yolo` to match the requested startup behavior.",
    )
    parser.add_argument(
        "--pause-window",
        type=int,
        default=10,
        help="Seconds to wait after each turn for operator input before auto-continuing.",
    )
    parser.add_argument(
        "--max-turns",
        type=int,
        default=20,
        help="Maximum number of turns to run before stopping.",
    )
    parser.add_argument(
        "--skill-hint",
        action=argparse.BooleanOptionalAction,
        default=True,
        help="Mention the optional $codex-session-autopilot skill in generated prompts.",
    )
    parser.add_argument(
        "--session-id",
        help="Optional explicit Codex session id to resume instead of relying on `--last`.",
    )
    parser.add_argument(
        "--state-dir",
        help="Directory for wrapper state. Defaults to <workspace>/.codex-autopilot.",
    )
    parser.add_argument(
        "--skip-git-repo-check",
        action="store_true",
        help="Forward `--skip-git-repo-check` to Codex exec commands.",
    )
    parser.add_argument(
        "--codex-root-arg",
        action="append",
        default=[],
        help="Extra root-level arguments forwarded before `exec`, for example `--search`.",
    )
    return parser.parse_args(argv)


def print_banner(text: str) -> None:
    print(f"\n=== {text} ===", flush=True)


def summarize_report(report: AutoReport) -> str:
    first_task = report.pending_tasks[0].task if report.pending_tasks else "No pending task reported."
    return (
        f"auto_mode_next={report.auto_mode_next} | "
        f"verification={report.verification_status} | "
        f"top_task={first_task}"
    )


def _stdin_ready(timeout_seconds: int) -> bool:
    if timeout_seconds <= 0:
        return False
    if not sys.stdin.isatty():
        return False
    readable, _, _ = select.select([sys.stdin], [], [], timeout_seconds)
    return bool(readable)


def _command_loop(report: AutoReport, pending_user_prompts: list[str]) -> str:
    print(
        "\nOperator input mode. Press Enter to continue, type free text to queue a prompt, "
        "or use /show, /clear, /stop, /help.",
        flush=True,
    )
    while True:
        try:
            line = input("autopilot> ").strip()
        except EOFError:
            return "continue"
        if not line:
            return "stop" if report.auto_mode_next == "stop" and not pending_user_prompts else "continue"
        if line == "/help":
            print(
                "/show   Show queued prompts and pending tasks\n"
                "/clear  Clear queued prompts\n"
                "/stop   Stop after this turn\n"
                "/help   Show this help\n"
                "Any other text queues a user prompt for the next turn.",
                flush=True,
            )
            continue
        if line == "/show":
            print("Queued prompts:", flush=True)
            if pending_user_prompts:
                for index, prompt in enumerate(pending_user_prompts, start=1):
                    print(f"  {index}. {prompt}", flush=True)
            else:
                print("  (none)", flush=True)
            print("Pending tasks:", flush=True)
            if report.pending_tasks:
                for task in report.pending_tasks:
                    print(f"  {task.priority} [{task.status}] {task.task}", flush=True)
            else:
                print("  (none reported)", flush=True)
            continue
        if line == "/clear":
            pending_user_prompts.clear()
            print("Queued prompts cleared.", flush=True)
            continue
        if line == "/stop":
            return "stop"
        pending_user_prompts.append(line)
        print("Queued for next turn.", flush=True)


def _post_turn_decision(
    *,
    report: AutoReport,
    pending_user_prompts: list[str],
    pause_window: int,
) -> str:
    print_banner("Turn Summary")
    print(report.summary, flush=True)
    print(summarize_report(report), flush=True)
    if report.user_engagement_needed:
        return _command_loop(report, pending_user_prompts)
    if not sys.stdin.isatty():
        return "stop" if report.auto_mode_next == "stop" else "continue"

    print(
        f"Next turn decision in {pause_window}s. "
        "Type anything to open operator input mode; otherwise the wrapper follows the agent decision.",
        flush=True,
    )
    if _stdin_ready(pause_window):
        return _command_loop(report, pending_user_prompts)
    return "stop" if report.auto_mode_next == "stop" else "continue"


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    workspace = Path(args.workspace).expanduser().resolve()
    state_dir = (
        Path(args.state_dir).expanduser().resolve()
        if args.state_dir
        else workspace / ".codex-autopilot"
    )
    dirs = ensure_state_dirs(state_dir)
    state_path = dirs["base"] / "session_state.json"

    try:
        state = load_or_create_state(
            state_path=state_path,
            workspace=workspace,
            initial_objective=args.objective,
            strategy=args.strategy,
            explicit_session_id=args.session_id,
        )
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    print_banner("Wrapper State")
    print(f"Workspace: {workspace}", flush=True)
    print(f"State dir: {dirs['base']}", flush=True)
    print(f"Strategy: {state.strategy}", flush=True)
    print(f"Profile: {args.profile}", flush=True)
    if args.codex_root_arg:
        print(f"Extra Codex root args: {' '.join(args.codex_root_arg)}", flush=True)
    state.save(state_path)

    while state.turn_index < args.max_turns:
        state.turn_index += 1
        resume = state.strategy == "hybrid" and state.session_started
        report_obj = AutoReport.from_obj(state.last_report) if state.last_report else None
        prompt = build_turn_prompt(
            workspace=workspace,
            initial_objective=state.initial_objective,
            strategy=state.strategy,
            turn_index=state.turn_index,
            last_report=report_obj,
            pending_user_prompts=state.pending_user_prompts,
            skill_hint=args.skill_hint,
        )

        prompt_path = dirs["prompts"] / f"turn-{state.turn_index:04d}.md"
        last_message_path = dirs["messages"] / f"turn-{state.turn_index:04d}.md"
        preview_command_parts = [
            args.codex_bin,
            *([] if not args.profile else ["-p", args.profile]),
            *args.codex_root_arg,
            "exec",
            *(
                ["resume", state.explicit_session_id]
                if resume and state.explicit_session_id
                else []
            ),
            *(["resume", "--last"] if resume and not state.explicit_session_id else []),
            *(["--skip-git-repo-check"] if args.skip_git_repo_check else []),
            "-o",
            str(last_message_path),
            "-",
        ]
        state.last_prompt_path = str(prompt_path)
        state.last_message_path = None
        state.save(state_path)
        print_banner(
            f"Starting turn {state.turn_index} ({'resume' if resume else 'initial'})"
        )
        print(" ".join(shlex.quote(part) for part in preview_command_parts), flush=True)

        result = run_codex_turn(
            codex_bin=args.codex_bin,
            profile=args.profile,
            workspace=workspace,
            prompt=prompt,
            prompt_path=prompt_path,
            last_message_path=last_message_path,
            resume=resume,
            explicit_session_id=state.explicit_session_id,
            extra_root_args=args.codex_root_arg,
            skip_git_repo_check=args.skip_git_repo_check,
        )

        state.last_prompt_path = str(prompt_path)
        state.last_message_path = str(last_message_path)
        state.session_started = True
        state.pending_user_prompts.clear()
        state.save(state_path)

        if result.returncode != 0:
            print(
                f"Codex exited with status {result.returncode}. "
                f"Inspect {last_message_path} and the console output above.",
                file=sys.stderr,
            )
            return result.returncode

        try:
            last_message_text = last_message_path.read_text(encoding="utf-8")
        except FileNotFoundError:
            print(
                f"error: Codex did not produce {last_message_path}.",
                file=sys.stderr,
            )
            return 1

        try:
            report = extract_report_from_text(last_message_text)
        except ProtocolError as exc:
            print(f"error: {exc}", file=sys.stderr)
            print(
                "The wrapper stops on protocol failure so the operator can inspect the last message.",
                file=sys.stderr,
            )
            return 1

        report_path = dirs["reports"] / f"turn-{state.turn_index:04d}.json"
        write_report(report_path, report)
        state.absorb_report(report)
        state.save(state_path)

        decision = _post_turn_decision(
            report=report,
            pending_user_prompts=state.pending_user_prompts,
            pause_window=args.pause_window,
        )
        state.save(state_path)
        if decision == "stop":
            print_banner("Wrapper Stop")
            print(
                "Stopping after this turn. "
                f"Last report: {report_path}",
                flush=True,
            )
            return 0

    print_banner("Max Turns Reached")
    print(
        f"Stopped after {args.max_turns} turns. "
        f"Review {dirs['base'] / 'reports'} before continuing.",
        flush=True,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
