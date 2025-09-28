"""MCP stdio server exposing a sandboxed Python runner.

This server is launched by the Agents SDK over stdio. It exposes tools that
allow an LLM to spawn, interact with, and stop Python programs inside a tightly
restricted Docker container. The program code is mounted read-only from the
provided workspace directory.
"""
from __future__ import annotations

import argparse
import json
import logging
import os
import queue
import shlex
import signal
import subprocess
import sys
import threading
import time
import uuid
from dataclasses import dataclass
from typing import Any, Dict, List, Optional

import anyio
from mcp import stdio_server, types
from mcp.server import Server


logger = logging.getLogger(__name__)


@dataclass
class RunnerConfig:
    workspace: str
    default_command: str
    python_image: str
    docker_user: str
    docker_cpus: str
    docker_memory: str
    tmpfs_size: str
    output_limit: int
    session_timeout: float
    idle_timeout: float


class ContainerSession:
    """Represents a running Docker container executing student code."""

    def __init__(self, session_id: str, command: str, config: RunnerConfig) -> None:
        self.id = session_id
        self.command = command
        self.config = config
        self.proc: Optional[subprocess.Popen[bytes]] = None
        self.events: "queue.Queue[Dict[str, Any]]" = queue.Queue()
        self.stdout_bytes = 0
        self.stderr_bytes = 0
        self.last_activity = time.time()
        self.deadline = self.last_activity + config.session_timeout if config.session_timeout > 0 else None
        self.closed = threading.Event()
        self._lock = threading.Lock()
        self._start_process()

    def _start_process(self) -> None:
        workspace = os.path.abspath(self.config.workspace)
        mount_workspace = f"{workspace}:/workspace:ro"
        mount_code = f"{workspace}:/code:ro"

        base_cmd = [
            "docker",
            "run",
            "--rm",
            "-i",
            "--network=none",
            "--user",
            self.config.docker_user,
            "--cpus",
            self.config.docker_cpus,
            "--memory",
            self.config.docker_memory,
            "--memory-swap",
            self.config.docker_memory,
            "--pids-limit",
            "128",
            "--read-only",
            "--cap-drop=ALL",
            "--security-opt",
            "no-new-privileges",
            "--security-opt",
            "label=disable",
            "--mount",
            f"type=tmpfs,destination=/tmp,tmpfs-size={self.config.tmpfs_size}",
            "-v",
            mount_workspace,
            "-v",
            mount_code,
        ]

        env_setup = "HOME=/tmp LANG=C.UTF-8 PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1"
        inner_cmd = f"cd /workspace && {env_setup} {self.command}"
        docker_cmd = base_cmd + [self.config.python_image, "bash", "-lc", inner_cmd]

        print(f"[mcp-runner] launching container {self.id}:", " ".join(docker_cmd), flush=True, file=sys.stderr)
        logger.info(
            "Launching container session",
            extra={"session_id": self.id, "command": self.command, "docker_args": docker_cmd},
        )

        try:
            self.proc = subprocess.Popen(
                docker_cmd,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                bufsize=0,
            )
        except Exception:
            logger.exception("Failed to start Docker process for session %s", self.id)
            raise

        threading.Thread(target=self._reader, args=(self.proc.stdout, "stdout"), daemon=True).start()
        threading.Thread(target=self._reader, args=(self.proc.stderr, "stderr"), daemon=True).start()
        threading.Thread(target=self._watcher, daemon=True).start()

    def _reader(self, pipe: Optional[Any], stream: str) -> None:
        if pipe is None:
            return
        try:
            while True:
                chunk = pipe.read(1024)
                if not chunk:
                    break
                self.last_activity = time.time()
                if stream == "stdout":
                    self.stdout_bytes += len(chunk)
                else:
                    self.stderr_bytes += len(chunk)
                text = chunk.decode("utf-8", errors="replace")
                #print(f"[mcp-runner] {self.id} {stream}: {text}", flush=True, file=sys.stderr)
                self.events.put({"type": stream, "data": text})
                if (self.stdout_bytes + self.stderr_bytes) > self.config.output_limit:
                    logger.warning(
                        "Session output limit exceeded",
                        extra={
                            "session_id": self.id,
                            "limit": self.config.output_limit,
                            "stdout_bytes": self.stdout_bytes,
                            "stderr_bytes": self.stderr_bytes,
                        },
                    )
                    self.events.put(
                        {
                            "type": "limit",
                            "message": "output limit exceeded",
                            "limit": self.config.output_limit,
                        }
                    )
                    self.stop(kill=True)
                    break
        finally:
            self.events.put({"type": f"{stream}_closed"})

    def _watcher(self) -> None:
        if self.proc is None:
            return
        while True:
            if self.deadline is not None and time.time() > self.deadline:
                logger.warning(
                    "Session wall time exceeded",
                    extra={"session_id": self.id, "timeout_seconds": self.config.session_timeout},
                )
                self.events.put(
                    {
                        "type": "timeout",
                        "message": "session wall time exceeded",
                        "seconds": self.config.session_timeout,
                    }
                )
                self.stop(kill=True)
                break
            if self.config.idle_timeout > 0 and (time.time() - self.last_activity) > self.config.idle_timeout:
                logger.warning(
                    "Session idle timeout exceeded",
                    extra={
                        "session_id": self.id,
                        "idle_timeout_seconds": self.config.idle_timeout,
                    },
                )
                self.events.put(
                    {
                        "type": "idle_timeout",
                        "message": "session idle timeout exceeded",
                        "seconds": self.config.idle_timeout,
                    }
                )
                self.stop(kill=True)
                break
            if self.proc.poll() is not None:
                code = self.proc.returncode
                if code not in (0, None):
                    logger.warning(
                        "Session exited with non-zero status",
                        extra={"session_id": self.id, "exit_code": code},
                    )
                self.events.put({"type": "exit", "code": code})
                break
            time.sleep(0.2)
        self.closed.set()

    def send(self, data: str) -> Dict[str, Any]:
        if self.proc is None or self.proc.stdin is None:
            logger.error("Attempted to send input to inactive session", extra={"session_id": self.id})
            return {"ok": False, "error": "process not running"}
        if self.proc.poll() is not None:
            logger.error("Attempted to send input to exited session", extra={"session_id": self.id})
            return {"ok": False, "error": "process already exited"}
        payload = data if data.endswith("\n") else data + "\n"
        try:
            self.proc.stdin.write(payload.encode("utf-8"))
            self.proc.stdin.flush()
            self.last_activity = time.time()
            return {"ok": True}
        except Exception as exc:  # pragma: no cover - pipe errors
            logger.exception("Failed to send input to session %s", self.id)
            return {"ok": False, "error": str(exc)}

    def read(self, wait_ms: int) -> Dict[str, Any]:
        events: List[Dict[str, Any]] = []
        timeout = max(wait_ms, 0) / 1000 if wait_ms else 0
        try:
            if timeout:
                event = self.events.get(timeout=timeout)
                events.append(event)
            while True:
                events.append(self.events.get_nowait())
        except queue.Empty:
            pass
        return {
            "events": events,
            "alive": self.proc.poll() is None if self.proc else False,
            "stdout_bytes": self.stdout_bytes,
            "stderr_bytes": self.stderr_bytes,
        }

    def stop(self, kill: bool = False) -> Dict[str, Any]:
        if self.proc is None:
            return {"ok": True, "message": "already stopped"}
        with self._lock:
            if self.proc.poll() is None:
                try:
                    if kill:
                        self.proc.send_signal(signal.SIGKILL)
                    else:
                        self.proc.terminate()
                except Exception:
                    pass
            try:
                self.proc.wait(timeout=2)
            except Exception:
                if kill:
                    try:
                        self.proc.kill()
                    except Exception:
                        pass
            self.closed.wait(timeout=2)
        return {"ok": True, "message": "stopped"}


class RunnerServer:
    def __init__(self, config: RunnerConfig) -> None:
        self.config = config
        self.sessions: Dict[str, ContainerSession] = {}
        self.server = Server("codedu-runner", instructions=(
            "Provide safe, step-by-step interaction with the student program. "
            "You may start multiple sessions; each session runs inside a sandboxed Docker "
            "container with no network access and a read-only view of the submission files."
        ))
        self._register_handlers()

    def _register_handlers(self) -> None:
        @self.server.list_tools()
        async def list_tools() -> List[types.Tool]:
            return [
                types.Tool(
                    name="start_program",
                    description=(
                        "Start the student's program inside the sandbox. Returns a session_id "
                        "used for subsequent send/read/stop calls."
                    ),
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "command": {
                                "type": "string",
                                "description": "Shell command to execute. Defaults to the provided main script.",
                            },
                            "session_label": {
                                "type": "string",
                                "description": "Optional friendly label for logging purposes.",
                            },
                        },
                    },
                    outputSchema={
                        "type": "object",
                        "properties": {
                            "session_id": {"type": "string"},
                            "command": {"type": "string"},
                        },
                        "required": ["session_id", "command"],
                    },
                ),
                types.Tool(
                    name="send_input",
                    description="Send a single line of input (appends newline automatically).",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "session_id": {"type": "string"},
                            "text": {"type": "string"},
                        },
                        "required": ["session_id", "text"],
                    },
                    outputSchema={
                        "type": "object",
                        "properties": {
                            "ok": {"type": "boolean"},
                            "error": {"type": "string"},
                        },
                        "required": ["ok"],
                    },
                ),
                types.Tool(
                    name="read_output",
                    description="Retrieve new stdout/stderr events from a session.",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "session_id": {"type": "string"},
                            "wait_ms": {
                                "type": "integer",
                                "minimum": 0,
                                "maximum": 10000,
                                "description": "Maximum milliseconds to wait for new events.",
                                "default": 250,
                            },
                        },
                        "required": ["session_id"],
                    },
                    outputSchema={
                        "type": "object",
                        "properties": {
                            "events": {"type": "array"},
                            "alive": {"type": "boolean"},
                            "stdout_bytes": {"type": "integer"},
                            "stderr_bytes": {"type": "integer"},
                        },
                        "required": ["events", "alive"],
                    },
                ),
                types.Tool(
                    name="stop_session",
                    description="Terminate a running session.",
                    inputSchema={
                        "type": "object",
                        "properties": {
                            "session_id": {"type": "string"},
                            "kill": {
                                "type": "boolean",
                                "description": "Force kill immediately (default false).",
                                "default": False,
                            },
                        },
                        "required": ["session_id"],
                    },
                    outputSchema={
                        "type": "object",
                        "properties": {
                            "ok": {"type": "boolean"},
                            "message": {"type": "string"},
                        },
                        "required": ["ok"],
                    },
                ),
            ]

        @self.server.call_tool()
        async def call_tool(name: str, arguments: Dict[str, Any]):
            if name == "start_program":
                return self._tool_start_program(arguments)
            if name == "send_input":
                return self._tool_send(arguments)
            if name == "read_output":
                return self._tool_read(arguments)
            if name == "stop_session":
                return self._tool_stop(arguments)
            return [types.TextContent(type="text", text=f"Unknown tool: {name}")]

    def _tool_start_program(self, args: Dict[str, Any]):
        command = args.get("command")
        if command:
            command = command.strip()
        if not command:
            command = self.config.default_command
        label = args.get("session_label") or "session"
        session_id = f"sess-{uuid.uuid4().hex[:12]}"
        session = ContainerSession(session_id, command, self.config)
        self.sessions[session_id] = session
        print(f"[mcp-runner] tool start_program label={label} command={command}", flush=True, file=sys.stderr)
        summary = {
            "session_id": session_id,
            "command": command,
            "label": label,
        }
        return (
            [types.TextContent(type="text", text=json.dumps(summary, indent=2))],
            summary,
        )

    def _get_session(self, session_id: str) -> Optional[ContainerSession]:
        session = self.sessions.get(session_id)
        if session is None:
            return None
        return session

    def _tool_send(self, args: Dict[str, Any]):
        session_id = args.get("session_id")
        text = args.get("text") or ""
        session = self._get_session(session_id)
        if session is None:
            result = {"ok": False, "error": "unknown session"}
            return (
                [types.TextContent(type="text", text=json.dumps(result))],
                result,
            )
        result = session.send(text)
        print(f"[mcp-runner] tool send_input session={session_id} text={text!r} -> {result}", flush=True, file=sys.stderr)
        return (
            [types.TextContent(type="text", text=json.dumps(result))],
            result,
        )

    def _tool_read(self, args: Dict[str, Any]):
        session_id = args.get("session_id")
        wait_ms = int(args.get("wait_ms", 250))
        session = self._get_session(session_id)
        if session is None:
            result = {"events": [], "alive": False, "error": "unknown session"}
            return (
                [types.TextContent(type="text", text=json.dumps(result))],
                result,
            )
        payload = session.read(wait_ms=wait_ms)
        print(f"[mcp-runner] tool read_output session={session_id} wait_ms={wait_ms} -> {payload}", flush=True, file=sys.stderr)
        return (
            [types.TextContent(type="text", text=json.dumps(payload, ensure_ascii=False))],
            payload,
        )

    def _tool_stop(self, args: Dict[str, Any]):
        session_id = args.get("session_id")
        kill = bool(args.get("kill", False))
        session = self._get_session(session_id)
        if session is None:
            result = {"ok": False, "error": "unknown session"}
            return (
                [types.TextContent(type="text", text=json.dumps(result))],
                result,
            )
        payload = session.stop(kill=kill)
        print(f"[mcp-runner] tool stop_session session={session_id} kill={kill} -> {payload}", flush=True, file=sys.stderr)
        return (
            [types.TextContent(type="text", text=json.dumps(payload))],
            payload,
        )


async def run_server(config: RunnerConfig) -> None:
    print("[mcp-runner] config:", config, flush=True, file=sys.stderr)
    srv = RunnerServer(config)
    init_opts = srv.server.create_initialization_options()
    async with stdio_server() as (read_stream, write_stream):
        await srv.server.run(read_stream, write_stream, init_opts)


def ensure_python_image(image: str) -> None:
    """Best effort ensure the runner image exists locally."""
    try:
        subprocess.run(["docker", "image", "inspect", image], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    except subprocess.CalledProcessError:
        subprocess.run(["docker", "pull", image], check=False)


def main() -> None:
    parser = argparse.ArgumentParser(description="CodeEdu MCP runner server")
    parser.add_argument("--workspace", required=True, help="Path to extracted submission workspace")
    parser.add_argument(
        "--main-file",
        required=True,
        help="Default python entrypoint (relative to workspace)",
    )
    parser.add_argument(
        "--python-image",
        default=os.environ.get("PYTHON_RUNNER_IMAGE", "python:3.11"),
        help="Docker image used to run python",
    )
    parser.add_argument("--docker-user", default=os.environ.get("DOCKER_USER", "65534:65534"))
    parser.add_argument("--docker-cpus", default=os.environ.get("DOCKER_CPUS", "0.5"))
    parser.add_argument("--docker-memory", default=os.environ.get("DOCKER_MEMORY", "256m"))
    parser.add_argument("--tmpfs-size", default=os.environ.get("RUNNER_TMPFS_SIZE", "32m"))
    parser.add_argument("--output-limit", type=int, default=64 * 1024)
    parser.add_argument("--session-timeout", type=float, default=60.0)
    parser.add_argument("--idle-timeout", type=float, default=15.0)
    parser.add_argument(
        "--default-command",
        default=None,
        help="Override default command (otherwise python -u main-file)",
    )
    args = parser.parse_args()

    level_name = os.environ.get("LLM_LOG_LEVEL") or os.environ.get("LOG_LEVEL") or "INFO"
    level = getattr(logging, level_name.upper(), logging.INFO)
    logging.basicConfig(level=level, format="[%(levelname)s] %(name)s: %(message)s", stream=sys.stderr, force=True)

    workspace = os.path.abspath(args.workspace)
    if not os.path.isdir(workspace):
        raise SystemExit(f"workspace not found: {workspace}")
    main_file = args.main_file
    default_command = args.default_command or f"python -u {shlex.quote(main_file)}"

    ensure_python_image(args.python_image)

    config = RunnerConfig(
        workspace=workspace,
        default_command=default_command,
        python_image=args.python_image,
        docker_user=args.docker_user,
        docker_cpus=args.docker_cpus,
        docker_memory=args.docker_memory,
        tmpfs_size=args.tmpfs_size,
        output_limit=int(args.output_limit),
        session_timeout=float(args.session_timeout),
        idle_timeout=float(args.idle_timeout),
    )

    anyio.run(run_server, config)


if __name__ == "__main__":
    main()
