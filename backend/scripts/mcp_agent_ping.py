#!/usr/bin/env python3
"""
Basic MCP smoke test: connect to backend.llm_agent.mcp_runner over stdio and
execute a simple “run python -c 'print(123)'” scenario without an LLM.
"""

import asyncio
import json
import os
import sys
from pathlib import Path

from anyio import create_task_group
from anyio.streams.memory import MemoryObjectReceiveStream, MemoryObjectSendStream

from mcp.client.stdio import stdio_client, StdioServerParameters
from mcp.client.session import ClientSession
from mcp.shared.message import SessionMessage
from mcp.shared.session import RequestResponder
from mcp import types

os.environ.setdefault("DOCKER_HOST", "tcp://docker-engine:2375")
os.environ.setdefault("DOCKER_TLS_CERTDIR", "")

WORKSPACE = Path(os.environ.get("MCP_WORKSPACE", "/sandbox/mcp-agent-demo"))
WORKSPACE.mkdir(parents=True, exist_ok=True)

# Make sure there is a simple Python program to run.
(WORKSPACE / "main.py").write_text(
    "name = input('Name? ')\nprint(f'hello {name}')\n",
    encoding="utf-8",
)


def server_params() -> StdioServerParameters:
    return StdioServerParameters(
        command=sys.executable,
        args=[
            "-m",
            "llm_agent.mcp_runner",
            "--workspace",
            str(WORKSPACE),
            "--main-file",
            "main.py",
            "--python-image",
            os.environ.get("PYTHON_RUNNER_IMAGE", "python:3.11"),
            "--docker-user",
            os.environ.get("DOCKER_USER", "65534:65534"),
            "--docker-cpus",
            os.environ.get("DOCKER_CPUS", "0.5"),
            "--docker-memory",
            os.environ.get("DOCKER_MEMORY", "256m"),
            "--tmpfs-size",
            os.environ.get("RUNNER_TMPFS_SIZE", "32m"),
            "--output-limit",
            "65536",
            "--session-timeout",
            "60",
            "--idle-timeout",
            "15",
        ],
        env={
            "DOCKER_HOST": os.environ.get("DOCKER_HOST", "tcp://docker-engine:2375"),
            "DOCKER_TLS_CERTDIR": os.environ.get("DOCKER_TLS_CERTDIR", ""),
            "PYTHONPATH": os.environ.get("PYTHONPATH", "/app"),
        },
        cwd=str(WORKSPACE),
    )


async def call_tool(
    session: ClientSession,
    name: str,
    arguments: dict,
) -> types.CallToolResult:
    req = types.CallToolRequest(
        params=types.CallToolRequestParams(name=name, arguments=arguments)
    )
    response = await session.call_tool(req.params.name, req.params.arguments or {})
    return response


async def run():
    async with stdio_client(server_params()) as streams:
        async with ClientSession(
            streams[0], streams[1], read_timeout_seconds=None
        ) as session:
            print("Connected, initializing…", flush=True)
            await session.initialize()
            print("Listing tools…")
            tools = await session.list_tools()
            for tool in tools.tools:
                print(f"  - {tool.name}")
            print("Starting program…")
            start = await call_tool(
                session,
                "start_program",
                {"session_label": "demo"},
            )
            payload = json.loads(start.content[0].text)
            session_id = payload["session_id"]
            print(f"Spawned session: {session_id}")

            print("Sending input…")
            await call_tool(
                session,
                "send_input",
                {"session_id": session_id, "text": "Alice"},
            )

            print("Reading output…")
            output = await call_tool(
                session,
                "read_output",
                {"session_id": session_id, "wait_ms": 500},
            )
            print(output.content[0].text)

            print("Stopping session…")
            await call_tool(
                session,
                "stop_session",
                {"session_id": session_id},
            )
            print("Done.")


if __name__ == "__main__":
    asyncio.run(run())
