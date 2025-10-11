#!/usr/bin/env python3
"""Generate a manual translation prompt for a specific frontend file.

This helper mirrors the instructions used by translate_app.py but fills in the
placeholders with the concrete file path, existing translations, and current
source contents so the prompt is ready to paste into ChatGPT for manual work.
"""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent
LOCALES_DIR = ROOT / 'frontend' / 'src' / 'lib' / 'i18n' / 'locales'
DEFAULT_LOCALE = 'en'
TARGET_LOCALE = 'cs'


def load_catalogue(locale: str) -> dict[str, str]:
    path = LOCALES_DIR / f'{locale}.json'
    if not path.exists():
        return {}
    try:
        return json.loads(path.read_text(encoding='utf-8'))
    except json.JSONDecodeError as exc:  # noqa: BLE001 - surface precise failure
        raise SystemExit(f'Failed to parse {path}: {exc}') from exc


def collect_existing_entries(rel_path: str, *catalogues: dict[str, str]) -> dict[str, dict[str, str]]:
    prefix = f'{rel_path}::'
    all_keys = {key for catalogue in catalogues for key in catalogue if key.startswith(prefix)}
    entries: dict[str, dict[str, str]] = {}
    for key in sorted(all_keys):
        entries[key] = {
            'en': catalogues[0].get(key, ''),
            'cs': catalogues[1].get(key, ''),
        }
    return entries


def build_prompt(rel_path: str, existing_entries_json: str, file_contents: str) -> str:
    return (
        "You are a meticulous localisation assistant.\n\n"
        "Context:\n"
        "- The project is a SvelteKit app that already exposes helpers at $lib/i18n.\n"
        "- Prefer `import {{ t }} from '$lib/i18n'` for static strings in script blocks.\n"
        "- In Svelte markup that needs reactivity, add `let translate;` and `$: translate = $translator;`, "
        "then call `translate('key')`.\n"
        "- Translate only user-visible copy. Leave logs, thrown errors, URLs, route segments, identifiers, "
        "asset names, data-test hooks, and other non-visual strings unchanged.\n"
        f"- Keys must follow `{rel_path}::slug` for this file.\n"
        "- Reuse existing catalogue entries when the English text already has a translation.\n\n"
        "Existing catalogue entries that match this file (may be empty if none):\n"
        f"{existing_entries_json}\n\n"
        "Task:\n"
        "Rewrite the source file so that all user-facing strings go through the i18n helpers, and produce Czech "
        "translations for every new visual string.\n\n"
        "Input file to update:\n"
        f"Path: {rel_path}\n"
        "Current contents:\n"
        "```\n"
        f"{file_contents}\n"
        "```\n\n"
        "Output requirements (no additional commentary):\n"
        "1. Section header `=== Updated Source ===` followed by the full updated file contents in a code block "
        "(no code fences, no truncation).\n"
        "2. Section header `=== New Translations ===` followed by a JSON object written into a code block with exactly two top-level keys, "
        "`\"en\"` and `\"cs\"`. Each key maps to an object whose properties are the new translation keys you introduced "
        "in this edit (exclude keys that already existed). Example shape:\n"
        "```\n"
        "{\n"
        '  "en": {\n'
        f'    "{rel_path}::new-key": "Original English string"\n'
        "  },\n"
        '  "cs": {\n'
        f'    "{rel_path}::new-key": "Czech translation"\n'
        "  }\n"
        "}\n"
        "```\n\n"
        "Do not include any other text or explanations.\n"
    )


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description='Prepare a manual translation prompt for a single frontend file.'
    )
    parser.add_argument(
        'file',
        help='Path to the source file to translate (relative to repository root).',
    )
    args = parser.parse_args(argv)

    input_path = Path(args.file)
    if not input_path.is_absolute():
        input_path = (ROOT / input_path).resolve()
    else:
        input_path = input_path.resolve()

    try:
        rel_path = input_path.relative_to(ROOT).as_posix()
    except ValueError as exc:
        raise SystemExit(f'File {input_path} must reside within the repository root {ROOT}.') from exc

    if not input_path.exists():
        raise SystemExit(f'File {input_path} does not exist.')

    file_contents = input_path.read_text(encoding='utf-8')

    english_catalogue = load_catalogue(DEFAULT_LOCALE)
    czech_catalogue = load_catalogue(TARGET_LOCALE)

    existing_entries = collect_existing_entries(rel_path, english_catalogue, czech_catalogue)
    existing_entries_json = json.dumps(existing_entries, ensure_ascii=False, indent=2)

    prompt = build_prompt(rel_path, existing_entries_json, file_contents)
    sys.stdout.write(prompt)
    if not prompt.endswith('\n'):
        sys.stdout.write('\n')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
