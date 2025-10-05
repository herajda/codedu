#!/usr/bin/env python3
"""Automated localization helper for the codedu frontend.

This script coordinates three responsibilities:

1. Discover user-facing source files in the frontend.
2. Ask OpenAI's ``gpt-5-mini`` model to rewrite each file so it relies on the
   shared ``$lib/i18n`` helpers and to provide Czech translations for all
   messages.
3. Persist the resulting source changes and locale dictionaries incrementally
   so interrupted runs can resume without re-translating work that already
   completed.

The script is intentionally asynchronous – up to 50 files are processed in
parallel. Progress is tracked in ``translations/state.json`` and the
locale catalogues are updated after every processed file.
"""
from __future__ import annotations

import argparse
import asyncio
import json
import logging
import os
from dataclasses import dataclass, field
from datetime import datetime, timezone
from hashlib import sha256
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional

# Third-party dependency provided by the OpenAI Python SDK.
from openai import AsyncOpenAI

ROOT = Path(__file__).resolve().parent
FRONTEND_SRC = ROOT / 'frontend' / 'src'
I18N_INTERNAL_DIR = FRONTEND_SRC / 'lib' / 'i18n'
LOCALES_DIR = I18N_INTERNAL_DIR / 'locales'
STATE_DIR = ROOT / 'translations'
STATE_FILE = STATE_DIR / 'state.json'
PROGRESS_LOG = STATE_DIR / 'progress.log'

DEFAULT_LOCALE = 'en'
TARGET_LOCALE = 'cs'
MODEL_NAME = 'gpt-5-mini'
CONCURRENCY = 50

SUPPORTED_EXTENSIONS = {'.svelte', '.ts', '.tsx'}
IGNORED_DIRECTORIES = {'node_modules', '.svelte-kit', '.git', '__pycache__'}


@dataclass
class ProcessedFile:
    """Metadata recorded for each processed source file."""

    checksum: str
    updated_at: str

    @classmethod
    def fresh(cls, checksum: str) -> 'ProcessedFile':
        return cls(checksum=checksum, updated_at=datetime.now(timezone.utc).isoformat())


@dataclass
class TranslationState:
    """Serializable progress information stored on disk."""

    processed_files: Dict[str, ProcessedFile] = field(default_factory=dict)

    @classmethod
    def load(cls, path: Path) -> 'TranslationState':
        if not path.exists():
            return cls()
        raw = json.loads(path.read_text(encoding='utf-8'))
        processed = {
            rel_path: ProcessedFile(**entry)
            for rel_path, entry in raw.get('processed_files', {}).items()
        }
        return cls(processed_files=processed)

    def to_json(self) -> Dict[str, Any]:
        return {
            'processed_files': {
                rel_path: {
                    'checksum': meta.checksum,
                    'updated_at': meta.updated_at,
                }
                for rel_path, meta in sorted(self.processed_files.items())
            }
        }

    def is_up_to_date(self, rel_path: str, checksum: str) -> bool:
        entry = self.processed_files.get(rel_path)
        return entry is not None and entry.checksum == checksum

    def mark_processed(self, rel_path: str, checksum: str) -> None:
        self.processed_files[rel_path] = ProcessedFile.fresh(checksum)


def load_env(dotenv_path: Path) -> None:
    if not dotenv_path.exists():
        return
    for line in dotenv_path.read_text(encoding='utf-8').splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith('#'):
            continue
        if '=' not in stripped:
            continue
        key, value = stripped.split('=', 1)
        key = key.strip()
        value = value.strip().strip('\"').strip("'")
        os.environ.setdefault(key, value)


def discover_source_files(root: Path) -> List[Path]:
    files: List[Path] = []
    for path in root.rglob('*'):
        if not path.is_file():
            continue
        if any(part in IGNORED_DIRECTORIES for part in path.parts):
            continue
        if I18N_INTERNAL_DIR in path.parents:
            continue
        if path.suffix.lower() in SUPPORTED_EXTENSIONS:
            files.append(path)
    files.sort()
    return files


def compute_checksum(content: str) -> str:
    return sha256(content.encode('utf-8')).hexdigest()


def read_json_file(path: Path) -> Dict[str, str]:
    if not path.exists():
        return {}
    try:
        return json.loads(path.read_text(encoding='utf-8'))
    except json.JSONDecodeError as exc:
        raise RuntimeError(f'Failed to read JSON from {path}: {exc}') from exc


def dump_json_atomic(path: Path, data: Dict[str, str]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp_path = path.with_suffix(path.suffix + '.tmp')
    tmp_path.write_text(json.dumps(data, ensure_ascii=False, indent=2, sort_keys=True) + '\n', encoding='utf-8')
    tmp_path.replace(path)


def write_text_atomic(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp_path = path.with_suffix(path.suffix + '.tmp')
    tmp_path.write_text(content, encoding='utf-8')
    tmp_path.replace(path)


async def async_write_json(path: Path, data: Dict[str, str]) -> None:
    await asyncio.to_thread(dump_json_atomic, path, data)


async def async_write_text(path: Path, content: str) -> None:
    await asyncio.to_thread(write_text_atomic, path, content)


async def async_read_text(path: Path) -> str:
    return await asyncio.to_thread(path.read_text, 'utf-8')


def build_prompt(rel_path: str, source: str, existing_catalogue: Dict[str, Dict[str, str]]) -> List[Dict[str, str]]:
    catalogue_preview = json.dumps(existing_catalogue, ensure_ascii=False, indent=2)
    instructions = f"""
You are helping localise a SvelteKit application. The project already exposes an
internationalisation helper at ``$lib/i18n`` with the following notable APIs:

- ``import {{ t, translator }} from '$lib/i18n'``
- ``$translator`` is a Svelte store that yields a translation function.
- ``t('key')`` immediately returns the translated string and may be used inside
  TypeScript/JavaScript modules.
- Keys map to entries in ``frontend/src/lib/i18n/locales/{{locale}}.json``.

Update the provided source file so every user-facing string resolves through one
of these helpers. Prefer adding ``import {{ t }} from '$lib/i18n'`` for static
invocations. When a Svelte component requires reactivity, declare
``let translate;`` followed by ``$: translate = $translator;`` and use
``translate('key')`` inside the markup.

Return a JSON object with two fields:
- ``updated_source`` containing the full updated file contents.
- ``translations`` mapping each translation key to an object with ``en`` and
  ``cs`` properties. ``en`` should keep the original English source. ``cs`` must
  contain the Czech translation. Use deterministic keys of the form
  ``"{rel_path}::slug"``.

Existing catalogue entries to reuse when possible:
{catalogue_preview}

Ensure the response is **valid JSON** and nothing else. Do not escape the JSON
string in additional quotes.
"""
    return [
        {'role': 'system', 'content': 'You are a meticulous localisation assistant.'},
        {
            'role': 'user',
            'content': f"File: {rel_path}\n\n{instructions}\n\nCurrent source:\n```\n{source}\n```"
        },
    ]


async def request_translation(client: AsyncOpenAI, rel_path: str, source: str, existing_catalogue: Dict[str, Dict[str, str]]) -> Dict[str, Any]:
    messages = build_prompt(rel_path, source, existing_catalogue)
    response = await client.responses.create(
        model=MODEL_NAME,
        input=[
            {'role': message['role'], 'content': [{'type': 'text', 'text': message['content']}]}  # type: ignore[arg-type]
            for message in messages
        ],
        temperature=0.2,
        max_output_tokens=4096,
    )
    output_text = response.output_text
    try:
        return json.loads(output_text)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f'Model returned invalid JSON for {rel_path}: {output_text}') from exc


def merge_translations(
    english: Dict[str, str],
    czech: Dict[str, str],
    payload: Dict[str, Dict[str, str]],
) -> None:
    for key, value in payload.items():
        if not isinstance(value, dict):
            logging.warning('Skipping malformed translation for %s (expected dict, got %r)', key, value)
            continue
        en_text = value.get('en')
        cs_text = value.get('cs')
        if en_text:
            english[key] = en_text
        if cs_text:
            czech[key] = cs_text


def ensure_logging(verbose: bool = False) -> None:
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    handlers: List[logging.Handler] = [logging.StreamHandler()]
    file_handler = logging.FileHandler(PROGRESS_LOG, encoding='utf-8')
    handlers.append(file_handler)

    logging.basicConfig(
        level=logging.DEBUG if verbose else logging.INFO,
        format='%(asctime)s [%(levelname)s] %(message)s',
        handlers=handlers,
    )


async def process_file(
    client: AsyncOpenAI,
    state: TranslationState,
    english_catalogue: Dict[str, str],
    czech_catalogue: Dict[str, str],
    path: Path,
    semaphore: asyncio.Semaphore,
    lock: asyncio.Lock,
    dry_run: bool,
) -> None:
    rel_path = path.relative_to(ROOT).as_posix()

    async with semaphore:
        source = await async_read_text(path)
        checksum = compute_checksum(source)
        if state.is_up_to_date(rel_path, checksum):
            logging.info('Skipping %s (already processed)', rel_path)
            return

        logging.info('Translating %s', rel_path)

        existing_entries = {
            key: {'en': english_catalogue.get(key, ''), 'cs': czech_catalogue.get(key, '')}
            for key in english_catalogue
            if key.startswith(f'{rel_path}::')
        }

        try:
            payload = await request_translation(client, rel_path, source, existing_entries)
        except Exception as exc:  # noqa: BLE001 - preserve stack for debugging
            logging.exception('Failed to translate %s: %s', rel_path, exc)
            return

        updated_source = payload.get('updated_source')
        translations = payload.get('translations', {})

        if not isinstance(updated_source, str):
            raise RuntimeError(f'Response for {rel_path} is missing the updated source code.')
        if not isinstance(translations, dict):
            raise RuntimeError(f'Response for {rel_path} returned invalid translations payload.')

        if dry_run:
            logging.info('Dry run – generated localisation for %s but did not modify files.', rel_path)
            return

        await async_write_text(path, updated_source)

        async with lock:
            merge_translations(english_catalogue, czech_catalogue, translations)
            final_checksum = compute_checksum(updated_source)
            state.mark_processed(rel_path, final_checksum)
            english_snapshot = dict(english_catalogue)
            czech_snapshot = dict(czech_catalogue)
            state_payload = state.to_json()

            await asyncio.gather(
                async_write_json(LOCALES_DIR / f'{DEFAULT_LOCALE}.json', english_snapshot),
                async_write_json(LOCALES_DIR / f'{TARGET_LOCALE}.json', czech_snapshot),
                asyncio.to_thread(STATE_FILE.write_text, json.dumps(state_payload, ensure_ascii=False, indent=2) + '\n', 'utf-8'),
            )

        logging.info('Finished %s', rel_path)


async def main_async(args: argparse.Namespace) -> None:
    load_env(ROOT / '.env')

    api_key = os.environ.get('OPENAI_API_KEY')
    if not api_key:
        raise EnvironmentError('OPENAI_API_KEY is not set. Add it to your environment or .env file.')

    ensure_logging(verbose=args.verbose)

    client = AsyncOpenAI(api_key=api_key)

    english_catalogue = read_json_file(LOCALES_DIR / f'{DEFAULT_LOCALE}.json')
    czech_catalogue = read_json_file(LOCALES_DIR / f'{TARGET_LOCALE}.json')
    state = TranslationState.load(STATE_FILE)

    files = discover_source_files(FRONTEND_SRC)
    if not files:
        logging.warning('No source files matched the selection. Nothing to do.')
        return

    concurrency = args.concurrency or CONCURRENCY
    if concurrency <= 0:
        raise ValueError('Concurrency must be a positive integer.')

    semaphore = asyncio.Semaphore(concurrency)
    lock = asyncio.Lock()

    tasks = [
        asyncio.create_task(
            process_file(client, state, english_catalogue, czech_catalogue, path, semaphore, lock, args.dry_run)
        )
        for path in files
    ]

    await asyncio.gather(*tasks)

    if args.dry_run:
        logging.info('Dry run finished. No files were modified.')
    else:
        STATE_FILE.write_text(json.dumps(state.to_json(), ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
        logging.info('Translation completed successfully.')


def parse_args(argv: Optional[Iterable[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description='Translate the frontend into Czech using OpenAI.')
    parser.add_argument('--dry-run', action='store_true', help='Run all requests without modifying files.')
    parser.add_argument('--verbose', action='store_true', help='Enable verbose logging output.')
    parser.add_argument('--concurrency', type=int, default=CONCURRENCY, help='Maximum number of concurrent translation requests.')
    return parser.parse_args(list(argv) if argv is not None else None)


def main(argv: Optional[Iterable[str]] = None) -> None:
    args = parse_args(argv)
    asyncio.run(main_async(args))


if __name__ == '__main__':
    main()
