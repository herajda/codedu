"""Agent-based interactive evaluation using MCP runner."""
from __future__ import annotations

import argparse
import asyncio
import json
import logging
import os
import sys
import textwrap
import time
import shlex
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Any, Dict, List, Literal, Optional

from agents import Agent, AgentOutputSchema, ItemHelpers, RunConfig, Runner
from agents.run import MaxTurnsExceeded
from agents.items import MessageOutputItem, ToolCallItem, ToolCallOutputItem
from agents.mcp import MCPServerStdio, MCPServerStdioParams
from openai import OpenAI

logger = logging.getLogger(__name__)


if not (os.environ.get("OPENAI_API_KEY") or os.environ.get("OPENAI_API_KEY_FILE")):
    print(json.dumps({"error": "OPENAI_API_KEY is not set"}))
    sys.exit(1)


@dataclass
class AssignmentContext:
    title: str
    description: str
    rubric: str = ""
    teacher_baseline: str = ""
    strictness: int = 50
    max_points: float = 0.0


@dataclass
class ReviewContext:
    summary: str


@dataclass
class EvaluationOutput:
    verdict: Literal["PASS", "FAIL", "ERROR"]
    reason: str
    summary: str
    recommendations: List[str] = field(default_factory=list)


def load_json_file(path: Optional[str]) -> Optional[Any]:
    if not path:
        return None
    file_path = Path(path)
    if not file_path.is_file():
        return None
    with file_path.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def truncate_text(text: str, limit: int) -> str:
    text = text.strip()
    if len(text) <= limit:
        return text
    return text[: max(limit - 3, 0)] + "..."


def collect_code_excerpt(workspace: Path, limit: int = 3000) -> str:
    parts: List[str] = []
    remaining = limit
    for path in sorted(workspace.rglob("*.py")):
        try:
            rel = path.relative_to(workspace)
        except ValueError:
            rel = path.name
        try:
            content = path.read_text(encoding="utf-8")
        except Exception:
            continue
        snippet = truncate_text(content, min(600, remaining))
        parts.append(f"# {rel}\n{snippet}")
        remaining -= len(snippet)
        if remaining <= 0:
            break
    return "\n\n".join(parts)


def build_instructions(
    ctx: AssignmentContext, review: Optional[ReviewContext], default_command: str, max_turns: int
) -> str:
    # Map strictness to a message for each 5% increment from 0% to 100%
    strictness_msgs = {
        0:  "Focus only on the most basic happy-path scenario; ignore edge cases.",
        5:  "Focus on the main happy-path scenario; minimal error handling.",
        10: "Test happy-path scenarios and basic error handling.",
        15: "Test happy-path scenarios and a few common error cases.",
        20: "Test happy-path scenarios and some error handling.",
        25: "Test happy-path scenarios and check for basic robustness.",
        30: "Focus on representative happy-path scenarios while checking fundamental error handling.",
        35: "Test typical flows and some important edge cases.",
        40: "Test typical flows and several edge cases.",
        45: "Balance typical flows with a few edge cases and robustness checks.",
        50: "Balance typical flows with important edge cases and robustness checks.",
        55: "Balance typical flows with more edge cases and robustness checks.",
        60: "Balance typical flows with thorough edge cases and robustness checks.",
        65: "Balance typical flows with comprehensive edge cases and robustness checks.",
        70: "Balance typical flows with important edge cases and robustness checks.",
        75: "Be strict and adversarial, probing tricky edge cases and robustness.",
        80: "Be strict and adversarial, probing more tricky edge cases and robustness.",
        85: "Be strict and adversarial, probing all tricky edge cases and robustness.",
        90: "Be strict and adversarial, probing tricky edge cases and robustness.",
        95: "Be maximally adversarial and exhaustive across edge cases.",
        100: "Be maximally adversarial and exhaustive across edge cases.",
    }
    # Find the closest lower or equal strictness bucket
    strictness_level = max([k for k in strictness_msgs if k <= ctx.strictness], default=0)
    strict_msg = strictness_msgs[strictness_level]

    rubric_part = f"Teacher rubric:\n{ctx.rubric}\n\n" if ctx.rubric.strip() else ""
    baseline_part = (
        f"Teacher baseline behavior (canonical solution):\n{ctx.teacher_baseline}\n\n"
        if ctx.teacher_baseline.strip()
        else ""
    )
    review_part = f"Static review findings:\n{review.summary}\n\n" if review else ""

    interactive_guidance = textwrap.dedent(
        """
        Interactive testing protocol:
        - Launch the student's program with start_program (the default command already points to the detected main file).
        - After each read_output, if the program prints a prompt such as "Enter the first number" or "Choose an operation", respond immediately with send_input using the expected value (for example "3", "+", "4").
        - Exercise at least one successful calculation and at least one division-by-zero scenario. If the static review suggests other scenarios, run them as well.
        - The teacher rubric assumes numeric input; encountering a crash on deliberately non-numeric data should be treated as a suggestion rather than a failing behaviour.
        - Prefer reusing the same session until a scenario is finished; call stop_session when the program exits or you have collected the evidence.
        """
    ).strip()

    return textwrap.dedent(
        f"""
        You are CodeEdu's automated examiner. Evaluate the student's CLI program by interacting with
        it inside the provided sandbox tools.

        Available tools (all run inside isolated Docker containers):
        - start_program(command?) -> session_id: launch the program (default command: {default_command}).
        - send_input(session_id, text): send a single line of input (a newline is appended).
        - read_output(session_id, wait_ms?): fetch new stdout/stderr events; wait_ms≈250 captures prompts.
        - stop_session(session_id, kill?): cleanly terminate a session when finished.

        Strategy:
        1. Start a fresh session for each independent scenario.
        2. After sending input, call read_output until no new output arrives or the program exits.
        3. Terminate sessions so containers do not hang.
        4. Compare observed behaviour against assignment requirements and the teacher baseline.
        5. Only return PASS if all requirements are satisfied. Use FAIL when behaviour is incorrect,
           and ERROR when the program cannot be meaningfully exercised (e.g., crashes immediately).

        You have at most {max_turns} turns (model/tool cycles). Finalise promptly once you have a
        confident verdict.

        {strict_msg}

        Assignment title: {ctx.title}
        Description:
        {ctx.description}

        {rubric_part}{baseline_part}{review_part}
        Produce a concise evaluation summary that adheres to the required JSON schema.

        {interactive_guidance}
        """
    ).strip()


def build_user_prompt(code_excerpt: str) -> str:
    if code_excerpt:
        return (
            "You have read-only access to the submission inside the sandbox. "
            "Here are truncated code excerpts for orientation:\n\n" + code_excerpt
        )
    return "Interact with the program to determine whether it meets the requirements."


def parse_assignment(data: Optional[Any]) -> AssignmentContext:
    if not data:
        return AssignmentContext(title="Untitled", description="")
    return AssignmentContext(
        title=str(data.get("title", "Untitled")),
        description=str(data.get("description", "")),
        rubric=str(data.get("rubric", "")),
        teacher_baseline=str(data.get("teacher_baseline", "")),
        strictness=int(data.get("strictness", 50) or 50),
        max_points=float(data.get("max_points", 0) or 0),
    )


def parse_review(data: Optional[Any]) -> Optional[ReviewContext]:
    if not data:
        return None
    if isinstance(data, dict):
        text = json.dumps(data, indent=2)
    else:
        text = str(data)
    return ReviewContext(summary=text)


def ensure_agent_dependencies() -> None:
    OpenAI()


def decode_arguments(raw: Any) -> Dict[str, Any]:
    if raw is None:
        return {}
    if isinstance(raw, str):
        try:
            return json.loads(raw)
        except json.JSONDecodeError:
            return {"raw": raw}
    if isinstance(raw, dict):
        return raw
    return {}


def coerce_structured_output(obj: Any) -> Dict[str, Any]:
    """Attempt to turn a ToolCallOutputItem.output into the structured dict.
    Some agent frameworks return the message content object {type: 'text', text: '{...json...}'}
    instead of the raw structured value. Handle both.
    """
    if obj is None:
        return {}
    # Already looks structured
    if isinstance(obj, dict):
        # Common structured keys we care about
        keys = set(obj.keys())
        if {"events", "alive"} & keys or {"session_id", "command"} <= keys or "ok" in keys:
            return obj
        # Text content wrapper containing JSON
        t = obj.get("type")
        txt = obj.get("text")
        if t == "text" and isinstance(txt, str):
            try:
                parsed = json.loads(txt)
                return parsed if isinstance(parsed, dict) else {}
            except Exception:
                return {}
        return {}
    if isinstance(obj, str):
        # obj might be a JSON string of either the structured payload
        # OR of a text-content wrapper {"type":"text","text":"{...}"}
        try:
            parsed = json.loads(obj)
        except Exception:
            return {}
        if isinstance(parsed, dict):
            # If it's already the structured dict, return it
            if {"events", "alive"} & set(parsed.keys()) or {"session_id", "command"} <= set(parsed.keys()) or "ok" in parsed:
                return parsed
            # If it's a wrapped text content, try to parse inner JSON
            if parsed.get("type") == "text" and isinstance(parsed.get("text"), str):
                try:
                    inner = json.loads(parsed["text"])
                    return inner if isinstance(inner, dict) else {}
                except Exception:
                    return {}
        return {}
    return {}


def format_program_event(event: Dict[str, Any]) -> List[str]:
    etype = event.get("type")
    lines: List[str] = []
    if etype in {"stdout", "stderr"}:
        text = str(event.get("data", ""))
        if not text:
            return []
        prefix = "PROGRAM> " if etype == "stdout" else "PROGRAM! "
        chunks = text.splitlines()
        for chunk in chunks:
            if chunk:
                lines.append(f"{prefix}{chunk}")
        if text.endswith("\n") and not chunks:
            lines.append(prefix.strip())
    elif etype == "exit":
        lines.append(f"PROGRAM> exited with code {event.get('code')}")
    elif etype == "timeout":
        lines.append("PROGRAM! session timed out")
    elif etype == "idle_timeout":
        lines.append("PROGRAM! idle timeout reached")
    elif etype == "limit":
        lines.append("PROGRAM! output limit exceeded")
    return lines


async def run_agent(cfg: argparse.Namespace) -> Dict[str, Any]:
    workspace = Path(cfg.workspace).resolve()
    mcp_params: MCPServerStdioParams = {
        "command": sys.executable,
        "args": [
            "-m",
            "llm_agent.mcp_runner",
            "--workspace",
            str(workspace),
            "--main-file",
            cfg.main_file,
            "--python-image",
            cfg.python_image,
            "--docker-user",
            cfg.docker_user,
            "--docker-cpus",
            cfg.docker_cpus,
            "--docker-memory",
            cfg.docker_memory,
            "--tmpfs-size",
            cfg.tmpfs_size,
            "--output-limit",
            str(cfg.output_limit),
            "--session-timeout",
            str(cfg.session_timeout),
            "--idle-timeout",
            str(cfg.idle_timeout),
        ],
        "cwd": str(workspace),
        "env": os.environ.copy(),
    }

    mcp_server = MCPServerStdio(
        mcp_params,
        cache_tools_list=True,
        name="sandbox",
        client_session_timeout_seconds=45,
        max_retry_attempts=1,
    )
    logger.info("Connecting to MCP server", extra={"mcp_command": mcp_params["command"], "mcp_args": mcp_params["args"]})
    try:
        await mcp_server.connect()
        logger.info("Connected to MCP server successfully")
    except Exception as exc:
        await mcp_server.cleanup()
        logger.exception("Failed to connect to MCP server")
        msg = str(exc)
        return {
            "verdict": "ERROR",
            "reason": msg,
            "summary": "Interactive agent run failed",
            "recommendations": [
                "Ensure the MCP runner can start (check Docker connectivity and Python path).",
            ],
            "transcript": "",
            "interactive": {"sessions": []},
            "model": cfg.model,
            "tool_calls": 0,
            "wall_time_ms": 0,
            "output_size": 0,
            "raw_output": {
                "verdict": "ERROR",
                "reason": msg,
                "summary": "MCP server failed to initialize",
            },
        }

    assignment = parse_assignment(load_json_file(cfg.assignment_json))
    review = parse_review(load_json_file(cfg.review_json))
    instructions = build_instructions(assignment, review, cfg.default_command, cfg.max_turns)
    code_excerpt = collect_code_excerpt(workspace, limit=2800)
    user_prompt = build_user_prompt(code_excerpt)

    print("[mcp-eval] assignment:", json.dumps(asdict(assignment), ensure_ascii=False), flush=True, file=sys.stderr)
    if review:
        print("[mcp-eval] review:", review.summary, flush=True, file=sys.stderr)
    print("[mcp-eval] instructions:\n", instructions, flush=True, file=sys.stderr)
    print("[mcp-eval] user_prompt:\n", user_prompt, flush=True, file=sys.stderr)

    agent = Agent(
        name="CodeEdu Interactive Evaluator",
        instructions=instructions,
        output_type=AgentOutputSchema(EvaluationOutput, strict_json_schema=True),
        mcp_servers=[mcp_server],
        model=cfg.model,
    )

    start = time.time()
    try:
        run_result = await Runner.run(
            starting_agent=agent,
            input=user_prompt,
            context={},
            max_turns=cfg.max_turns,
            run_config=RunConfig(model=cfg.model),
        )
        print("[mcp-eval] === trace ===", flush=True, file=sys.stderr)
        for item in run_result.new_items:
            if isinstance(item, MessageOutputItem):
                text = ItemHelpers.text_message_output(item).strip()
                print("[mcp-eval] ai_message:", text, flush=True, file=sys.stderr)
            elif isinstance(item, ToolCallItem):
                raw = item.raw_item
                print(f"[mcp-eval] tool_call -> {raw.name} args={raw.arguments}", flush=True, file=sys.stderr)
            elif isinstance(item, ToolCallOutputItem):
                raw = item.raw_item
                raw_type = getattr(raw, "type", None)
                if raw_type is None and isinstance(raw, dict):
                    raw_type = raw.get("type", type(raw).__name__)
                elif raw_type is None:
                    raw_type = type(raw).__name__
                print(f"[mcp-eval] tool_output <- {raw_type}: {item.output}", flush=True, file=sys.stderr)
    except MaxTurnsExceeded:
        return {
            "verdict": "ERROR",
            "reason": f"Max turns ({cfg.max_turns}) exceeded before the evaluation completed",
            "summary": "The automated tester hit its interaction limit.",
            "recommendations": [
                "Increase LLM_AGENT_MAX_TURNS or simplify the interactive flow so the agent can finish.",
            ],
            "transcript": "",
            "interactive": {"sessions": []},
            "model": cfg.model,
            "tool_calls": cfg.max_turns,
            "wall_time_ms": int((time.time() - start) * 1000),
            "output_size": 0,
            "raw_output": {
                "verdict": "ERROR",
                "reason": "max_turns_exceeded",
                "summary": "Max turns exceeded",
            },
        }
    finally:
        await mcp_server.cleanup()
    elapsed_ms = int((time.time() - start) * 1000)

    final = run_result.final_output
    if isinstance(final, EvaluationOutput):
        final_payload = asdict(final)
    elif isinstance(final, dict):
        final_payload = final
    else:
        final_payload = {
            "verdict": "ERROR",
            "reason": "Agent did not return structured output",
            "summary": str(final),
            "recommendations": [],
        }

    call_index: Dict[str, Dict[str, Any]] = {}
    session_logs: Dict[str, Dict[str, Any]] = {}
    last_session_id: Optional[str] = None
    transcript_lines: List[str] = []

    for item in run_result.new_items:
        if isinstance(item, MessageOutputItem):
            text = ItemHelpers.text_message_output(item).strip()
            if text:
                transcript_lines.append(f"AI> {text}")
        elif isinstance(item, ToolCallItem):
            raw = item.raw_item
            call_id = getattr(raw, "id", getattr(raw, "call_id", None))
            tool_name = getattr(raw, "name", getattr(raw, "tool_name", "unknown"))
            arguments = decode_arguments(getattr(raw, "arguments", None))
            call_index[str(call_id)] = {"tool": tool_name, "args": arguments}
            if tool_name == "start_program":
                cmd = arguments.get("command") or cfg.default_command
                transcript_lines.append(f"AI> start_program command={cmd}")
            elif tool_name == "send_input":
                transcript_lines.append(f"AI> send_input {arguments.get('text', '')!r}")
            elif tool_name == "stop_session":
                transcript_lines.append("AI> stop_session")
        elif isinstance(item, ToolCallOutputItem):
            raw = item.raw_item
            # Robustly match this output to its originating call
            call_id = (
                getattr(raw, "call_id", None)
                or getattr(raw, "id", None)
                or getattr(raw, "tool_call_id", None)
            )
            ctx = call_index.get(str(call_id)) if call_id is not None else None
            structured = coerce_structured_output(item.output)

            tool_name = ctx.get("tool") if ctx else None
            call_args = ctx.get("args", {}) if ctx else {}

            # Primary path: we know the tool that produced this output
            if tool_name == "start_program":
                session_id = structured.get("session_id")
                command = structured.get("command") or call_args.get("command") or cfg.default_command
                if session_id:
                    session_logs[session_id] = {"session_id": session_id, "command": command, "events": []}
                    last_session_id = session_id
            elif tool_name == "send_input":
                session_id = call_args.get("session_id") or last_session_id
                if session_id:
                    log = session_logs.setdefault(
                        session_id,
                        {"session_id": session_id, "command": call_args.get("command", cfg.default_command), "events": []},
                    )
                    log["events"].append({"type": "input", "text": call_args.get("text", "")})
            elif tool_name == "read_output":
                session_id = call_args.get("session_id") or last_session_id
                events = structured.get("events", [])
                if session_id:
                    log = session_logs.setdefault(
                        session_id,
                        {"session_id": session_id, "command": call_args.get("command", cfg.default_command), "events": []},
                    )
                    for ev in events:
                        log["events"].append(ev)
                        transcript_lines.extend(format_program_event(ev))
            elif tool_name == "stop_session":
                transcript_lines.append("AI> session stopped")
            else:
                # Fallback path: infer tool by output shape when call linkage is missing
                if structured.get("session_id") and structured.get("command"):
                    session_id = structured.get("session_id")
                    command = structured.get("command") or cfg.default_command
                    session_logs[session_id] = {"session_id": session_id, "command": command, "events": []}
                    last_session_id = session_id
                elif "events" in structured:
                    session_id = last_session_id
                    events = structured.get("events", [])
                    if session_id:
                        log = session_logs.setdefault(
                            session_id,
                            {"session_id": session_id, "command": cfg.default_command, "events": []},
                        )
                        for ev in events:
                            log["events"].append(ev)
                            transcript_lines.extend(format_program_event(ev))

    transcript = "\n".join(line for line in transcript_lines if line.strip())
    interactive = {"sessions": list(session_logs.values())}

    if len(interactive["sessions"]) == 0:
        await mcp_server.cleanup()
        return {
            "verdict": "ERROR",
            "reason": "No interactive sessions were executed. The program was not run.",
            "summary": "The evaluator did not observe the CLI; run start_program and interact with it before emitting the review.",
            "recommendations": [
                "Before calling emit_review, launch the student's program via start_program and exercise at least one scenario using send_input/read_output.",
                "Capture the observed prompts and outputs so the transcript reflects the actual run.",
            ],
            "transcript": transcript,
            "interactive": interactive,
            "model": cfg.model,
            "tool_calls": len(run_result.new_items),
            "wall_time_ms": elapsed_ms,
            "output_size": len(transcript),
            "raw_output": {
                "verdict": "ERROR",
                "reason": "no_interactive_session",
                "summary": "Interactive session missing",
            },
        }

    model_name = cfg.model
    if run_result.raw_responses:
        last_resp = run_result.raw_responses[-1]
        model_name = getattr(last_resp, "model", model_name)

    output = {
        "verdict": final_payload.get("verdict", "ERROR"),
        "reason": final_payload.get("reason", ""),
        "summary": final_payload.get("summary", ""),
        "recommendations": final_payload.get("recommendations", []),
        "transcript": transcript,
        "interactive": interactive,
        "model": model_name,
        "tool_calls": sum(1 for item in run_result.new_items if isinstance(item, ToolCallItem)),
        "wall_time_ms": elapsed_ms,
        "output_size": len(transcript),
        "raw_output": final_payload,
    }
    return output


def main() -> None:
    parser = argparse.ArgumentParser(description="Run MCP-backed interactive evaluation")
    parser.add_argument("--workspace", required=True)
    parser.add_argument("--main-file", required=True)
    parser.add_argument("--assignment-json")
    parser.add_argument("--review-json")
    parser.add_argument("--python-image", default=os.environ.get("PYTHON_RUNNER_IMAGE", "python:3.11"))
    parser.add_argument("--docker-user", default=os.environ.get("DOCKER_USER", "65534:65534"))
    parser.add_argument("--docker-cpus", default=os.environ.get("DOCKER_CPUS", "0.5"))
    parser.add_argument("--docker-memory", default=os.environ.get("DOCKER_MEMORY", "256m"))
    parser.add_argument("--tmpfs-size", default=os.environ.get("RUNNER_TMPFS_SIZE", "32m"))
    parser.add_argument("--output-limit", type=int, default=64 * 1024)
    parser.add_argument("--session-timeout", type=float, default=60.0)
    parser.add_argument("--idle-timeout", type=float, default=15.0)
    parser.add_argument("--model", default=os.environ.get("OPENAI_LLM_MODEL", "gpt-4.1"))
    default_turns = int(os.environ.get("LLM_AGENT_MAX_TURNS", "128"))
    parser.add_argument("--max-turns", type=int, default=default_turns)
    parser.add_argument("--default-command", default=None)
    cfg = parser.parse_args()

    level_name = os.environ.get("LLM_LOG_LEVEL") or os.environ.get("LOG_LEVEL") or "INFO"
    level = getattr(logging, level_name.upper(), logging.INFO)
    logging.basicConfig(level=level, format="[%(levelname)s] %(name)s: %(message)s", stream=sys.stderr, force=True)

    ensure_agent_dependencies()

    workspace = Path(cfg.workspace)
    if not workspace.is_dir():
        print(json.dumps({"error": f"workspace not found: {workspace}"}))
        sys.exit(1)

    if not cfg.default_command:
        cfg.default_command = f"python -u {shlex.quote(cfg.main_file)}"

    try:
        result = asyncio.run(run_agent(cfg))
    except Exception as exc:  # pragma: no cover - bubble up to Go worker
        print(json.dumps({
            "verdict": "ERROR",
            "reason": str(exc),
            "summary": "Interactive agent run failed",
            "transcript": "",
            "interactive": {"sessions": []},
        }))
        sys.exit(1)

    print(json.dumps(result))


if __name__ == "__main__":
    main()
