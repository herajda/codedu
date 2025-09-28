#!/usr/bin/env python3
"""
LLM-driven MCP test: gives an OpenAI model access to the MCP tools and asks it
to solve a simple CLI interaction.
"""

import asyncio
import os
from pathlib import Path

from agents import Agent, RunConfig, Runner
from agents.items import MessageOutputItem, ToolCallItem, ToolCallOutputItem
from agents.mcp import MCPServerStdio, MCPServerStdioParams
from agents import AgentOutputSchema
from openai import OpenAI
from pydantic import BaseModel


class Result(BaseModel):
    transcript: str
    verdict: str

os.environ.setdefault("DOCKER_HOST", "tcp://docker-engine:2375")
os.environ.setdefault("DOCKER_TLS_CERTDIR", "")

WORKSPACE = Path(os.environ.get("MCP_WORKSPACE", "/sandbox/mcp-agent-demo"))
WORKSPACE.mkdir(parents=True, exist_ok=True)

(WORKSPACE / "main.py").write_text(
    "name = input('Name? ')\nprint(f'hello {name}')\n",
    encoding="utf-8",
)


def mcp_server() -> MCPServerStdio:
    params: MCPServerStdioParams = {
        "command": os.environ.get("LLM_EVALUATOR_PYTHON", "/opt/llm-agent-venv/bin/python"),
        "args": [
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
        "env": {
            "DOCKER_HOST": os.environ.get("DOCKER_HOST", "tcp://docker-engine:2375"),
            "DOCKER_TLS_CERTDIR": os.environ.get("DOCKER_TLS_CERTDIR", ""),
            "PYTHONPATH": os.environ.get("PYTHONPATH", "/app"),
        },
        "cwd": str(WORKSPACE),
    }
    return MCPServerStdio(
        params,
        cache_tools_list=True,
        name="sandbox",
        client_session_timeout_seconds=45,
        max_retry_attempts=1,
    )


async def run():
    if not os.getenv("OPENAI_API_KEY"):
        raise SystemExit("OPENAI_API_KEY is not set")

    server = mcp_server()
    try:
        await server.connect()
    except Exception as exc:
        await server.cleanup()
        raise SystemExit(f"Failed to initialise MCP server: {exc}")

    try:
        agent = Agent(
            name="Sandbox Tester",
            instructions=(
                "Use the available tools to run the program, feed it the name 'Alice', "
                "and report the output you observe."
            ),
            model=os.environ.get("OPENAI_LLM_MODEL", "gpt-4.1"),
            output_type=AgentOutputSchema(Result, strict_json_schema=True),
            mcp_servers=[server],
        )

        result = await Runner.run(
            starting_agent=agent,
            input="Please run the student program and tell me what it prints.",
            context={},
            run_config=RunConfig(model=os.environ.get("OPENAI_LLM_MODEL", "gpt-4.1")),
        )
        print("=== TRACE ===")
        for item in result.new_items:
            if isinstance(item, MessageOutputItem):
                print(f"AI message: {item.raw_item.content}")
            elif isinstance(item, ToolCallItem):
                print(f"Tool call -> {item.raw_item.name} args={item.raw_item.arguments}")
            elif isinstance(item, ToolCallOutputItem):
                raw = item.raw_item
                raw_type = getattr(raw, "type", None)
                if raw_type is None and isinstance(raw, dict):
                    raw_type = raw.get("type", type(raw).__name__)
                elif raw_type is None:
                    raw_type = type(raw).__name__
                print(f"Tool output <- {raw_type}: {item.output}")
        print("=== FINAL OUTPUT ===")
        print(result.final_output)
    finally:
        await server.cleanup()


if __name__ == "__main__":
    asyncio.run(run())
