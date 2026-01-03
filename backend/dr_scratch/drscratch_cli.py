#!/usr/bin/env python3
import argparse
import json
import os
import sys
from zipfile import BadZipFile, ZipFile

from app.hairball3.backdropNaming import BackdropNaming
from app.hairball3.deadCode import DeadCode
from app.hairball3.duplicateScripts import DuplicateScripts
from app.hairball3.mastery import Mastery
from app.hairball3.spriteNaming import SpriteNaming


DEFAULT_SKILL_POINTS = {
    "Abstraction": 4,
    "Parallelization": 4,
    "Logic": 4,
    "Synchronization": 4,
    "FlowControl": 4,
    "UserInteractivity": 4,
    "DataRepresentation": 4,
    "MathOperators": 4,
    "MotionOperators": 4,
}


def load_json_project(sb3_path):
    try:
        with ZipFile(sb3_path, "r") as zip_file:
            with zip_file.open("project.json") as project_file:
                return json.loads(project_file.read().decode("utf-8"))
    except KeyError as exc:
        raise ValueError("project.json not found in sb3 archive") from exc
    except BadZipFile as exc:
        raise ValueError("invalid sb3 file (bad zip)") from exc
    except json.JSONDecodeError as exc:
        raise ValueError("invalid project.json") from exc


def build_bad_habits(duplicate_result, dead_code_result, sprite_naming, backdrop_naming):
    duplicate_payload = duplicate_result.get("result", {})
    dead_payload = dead_code_result.get("result", {})

    dead_scripts = {}
    for sprite_dict in dead_payload.get("list_dead_code_scripts", []):
        for sprite_name, list_blocks in sprite_dict.items():
            dead_scripts[sprite_name] = list_blocks

    return {
        "duplicateScript": {
            "number": duplicate_payload.get("total_duplicate_scripts", 0),
            "scripts": duplicate_payload.get("list_duplicate_scripts", []),
            "csv_format": duplicate_payload.get("list_csv", []),
        },
        "deadCode": {
            "number": dead_payload.get("total_dead_code_scripts", 0),
            "scripts": dead_scripts,
        },
        "spriteNaming": sprite_naming,
        "backdropNaming": backdrop_naming,
    }


def parse_args(argv):
    parser = argparse.ArgumentParser(
        description="Analyze a single Scratch .sb3 file and output scores and bad habits as JSON."
    )
    parser.add_argument("sb3_path", help="Path to a Scratch .sb3 file")
    parser.add_argument(
        "-o",
        "--output",
        help="Output JSON path (default: <input>.analysis.json)",
    )
    return parser.parse_args(argv)


def main(argv):
    args = parse_args(argv)
    sb3_path = os.path.abspath(args.sb3_path)

    if not os.path.isfile(sb3_path):
        print(f"error: file not found: {sb3_path}", file=sys.stderr)
        return 1
    if os.path.splitext(sb3_path)[1].lower() != ".sb3":
        print("error: input must be a .sb3 file", file=sys.stderr)
        return 1

    output_path = args.output
    if not output_path:
        base, _ = os.path.splitext(sb3_path)
        output_path = f"{base}.analysis.json"

    try:
        json_project = load_json_project(sb3_path)
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    mastery_result = Mastery(sb3_path, json_project, DEFAULT_SKILL_POINTS, "Default").finalize()
    duplicate_result = DuplicateScripts(sb3_path, json_project).finalize()
    dead_code_result = DeadCode(sb3_path, json_project).finalize()
    sprite_naming = SpriteNaming(sb3_path, json_project).finalize()
    backdrop_naming = BackdropNaming(sb3_path, json_project).finalize()

    output = {
        "vanilla": mastery_result.get("vanilla", {}),
        "extended": mastery_result.get("extended", {}),
        "bad_habits": build_bad_habits(
            duplicate_result, dead_code_result, sprite_naming, backdrop_naming
        ),
    }

    with open(output_path, "w", encoding="utf-8") as output_file:
        json.dump(output, output_file, indent=2, sort_keys=True)

    print(output_path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
