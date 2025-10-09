#!/usr/bin/env python3
"""Automated localization helper for the codedu frontend (tool-calling version).

This script coordinates three responsibilities:

1. Discover user-facing source files in the frontend.
2. Ask Google's Gemini ``gemini-2.5-flash`` model to rewrite each file so it relies on the
   shared ``$lib/i18n`` helpers and to provide Czech translations for all
   messages.
3. Persist the resulting source changes and locale dictionaries incrementally
   so interrupted runs can resume without re-translating work that already
   completed.

Key differences vs. the JSON-text approach:
- Uses Gemini's function calling with a STRICT tool schema and forces a call to
  function `emit_translation` that returns validated JSON.
"""
from __future__ import annotations

import argparse
import ast
import asyncio
import json
import logging
import os
import re
from dataclasses import dataclass, field
from datetime import datetime, timezone
from hashlib import sha256
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Union

import httpx
from pydantic import BaseModel, ConfigDict, Field, ValidationError

ROOT = Path(__file__).resolve().parent
FRONTEND_SRC = ROOT / 'frontend' / 'src'
I18N_INTERNAL_DIR = FRONTEND_SRC / 'lib' / 'i18n'
LOCALES_DIR = I18N_INTERNAL_DIR / 'locales'
STATE_DIR = ROOT / 'translations'
STATE_FILE = STATE_DIR / 'state.json'
PROGRESS_LOG = STATE_DIR / 'progress.log'

DEFAULT_LOCALE = 'en'
TARGET_LOCALE = 'cs'
MODEL_NAME = 'gemini-2.5-flash'
GEMINI_API_BASE = 'https://generativelanguage.googleapis.com/v1beta'
CONCURRENCY = 10
MAX_TRANSLATION_RETRIES = 3
RETRY_BACKOFF_SECONDS = 2.0

# ---- Pydantic models (still useful for type safety locally) ------------------

class TranslationEntry(BaseModel):
    model_config = ConfigDict(extra='forbid')
    key: str
    en: str
    cs: str


class TranslationPayload(BaseModel):
    """Our internal payload shape after we reassemble chunks."""
    model_config = ConfigDict(extra='forbid')
    updated_source: str
    translations: List[TranslationEntry] = Field(default_factory=list)

# ---- Tool schema (STRICT) ---------------------------------------------------

# Drop-in tool matching TranslationPayload exactly
# ---- Tool schema (STRICT) ---------------------------------------------------
# Explicit JSON Schema to satisfy the Responses API validation rules.

TRANSLATION_TOOL: Dict[str, Any] = {
    "type": "function",
    "name": "emit_translation",
    "description": "Return updated source and visual translation entries for one file.",
    "parameters": {
        "type": "object",
        "properties": {
            "updated_source": {"type": "string"},
            "translations": {
                "type": "array",
                "items": {
                    "type": "object",
                    "properties": {
                        "key": {"type": "string"},
                        "en": {"type": "string"},
                        "cs": {"type": "string"},
                    },
                    "required": ["key", "en", "cs"],
                    "additionalProperties": False,
                },
            },
        },
        "required": ["updated_source", "translations"],
        "additionalProperties": False,
    },
    "strict": True,
}
GEMINI_FUNCTION_DECLARATIONS = [{
    "name": "emit_translation",
    "description": "Return updated source and visual translation entries for one file.",
    "parameters": {
        "type": "OBJECT",
        "properties": {
            "updated_source": {"type": "STRING"},
            "translations": {
                "type": "ARRAY",
                "items": {
                    "type": "OBJECT",
                    "properties": {
                        "key": {"type": "STRING"},
                        "en": {"type": "STRING"},
                        "cs": {"type": "STRING"},
                    },
                    "required": ["key", "en", "cs"],
                },
            },
        },
        "required": ["updated_source", "translations"],
    },
}]

# ---- File discovery / state --------------------------------------------------

SUPPORTED_EXTENSIONS = {'.svelte', '.ts', '.tsx'}
IGNORED_DIRECTORIES = {'node_modules', '.svelte-kit', '.git', '__pycache__'}

ROUTE_LIKE_RE = re.compile(r'^/?[A-Za-z0-9_\-./\[\]]+$')
FILE_SUFFIX_RE = re.compile(r'\.[A-Za-z0-9]{2,6}$')
NON_VISUAL_KEY_TERMS = (
    '::error', '_error', 'exception', 'stack', 'trace', 'logger', 'log_',
    'route', 'path', 'url', 'href', 'logo'
)


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

# ---- Utilities ---------------------------------------------------------------

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
    tmp_path.write_text(
        json.dumps(data, ensure_ascii=False, indent=2, sort_keys=True) + '\n',
        encoding='utf-8',
    )
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

# ---- Prompt builder ----------------------------------------------------------

def build_prompt(rel_path: str, source: str, existing_catalogue: Dict[str, Dict[str, str]]) -> List[Dict[str, Any]]:
    """Build a prompt that instructs the model AND points it at our tool."""
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

Translate **only** text that the user can see in the rendered interface.
- Do not translate log statements, thrown errors, developer diagnostics, or
  console output.
- Do not translate route fragments, URLs, file paths, asset names, brand or
  logo identifiers, or any other non-visual identifiers.
- Leave internal constants, data-test attributes, ARIA labels used solely for
  structure, and programmatic sentinel values untouched unless they are also
  shown to users.

Emit your result by CALLING the function tool ``emit_translation`` exactly once
with arguments matching the schema:
- ``updated_source``: the FULL updated file contents as a single string (no code fences).
- ``translations``: array of objects with ``key``, ``en``, ``cs``.
  Keys MUST follow: ``"{rel_path}::slug"``. ``en`` keeps original English, ``cs`` is Czech.
Do not wrap this tool call in ``print(...)``, Markdown fences, or any other text.
Return nothing except the structured function call.

Existing catalogue entries to reuse when possible:
{catalogue_preview}
"""
    return [
        {"role": "system", "content": "You are a meticulous localisation assistant."},
        {
            "role": "user",
            "content": [
                {"type": "input_text", "text": f"File: {rel_path}\n\n{instructions}\n\nCurrent source:\n```\n{source}\n```"}  # noqa: E501
            ],
        },
    ]

# ---- Heuristics for filtering non-visual strings ----------------------------

def _is_route_like(text: str) -> bool:
    stripped = text.strip()
    if '/' not in stripped or ' ' in stripped:
        return False
    return bool(ROUTE_LIKE_RE.fullmatch(stripped))


def _looks_like_file_reference(text: str) -> bool:
    stripped = text.strip()
    if FILE_SUFFIX_RE.search(stripped) and '/' in stripped:
        return True
    return False


def _contains_letters(text: str) -> bool:
    return any(ch.isalpha() for ch in text)


def should_translate_entry(key: str, en_text: str) -> bool:
    if not en_text:
        return False
    stripped = en_text.strip()
    if not stripped or not _contains_letters(stripped):
        return False

    lower_key = key.lower()
    lower_text = stripped.lower()

    if any(term in lower_key for term in NON_VISUAL_KEY_TERMS):
        return False
    if any(term in lower_text for term in ('console', 'traceback', 'stack trace', 'stacktrace', 'logger')):
        return False
    if 'logo' in lower_text:
        return False
    if _is_route_like(stripped):
        return False
    if _looks_like_file_reference(stripped):
        return False
    return True


def _appears_in_non_visual_context(updated_source: str, key: str) -> bool:
    escaped_key = re.escape(key)
    patterns = [
        rf'throw\s+new\s+Error\s*\(\s*t\(\s*["\']{escaped_key}["\']',
        rf'console\.[a-zA-Z]+\s*\([^)]*t\(\s*["\']{escaped_key}["\']',
        rf'logger\.[a-zA-Z]+\s*\([^)]*t\(\s*["\']{escaped_key}["\']',
    ]
    return any(re.search(pattern, updated_source) for pattern in patterns)


def filter_visual_translations(updated_source: str, entries: Iterable[Dict[str, str]]) -> List[Dict[str, str]]:
    filtered: List[Dict[str, str]] = []
    for entry in entries:
        key = entry.get('key') if isinstance(entry, dict) else None
        en_text = entry.get('en') if isinstance(entry, dict) else None
        if not key or en_text is None:
            continue
        if not should_translate_entry(key, en_text):
            logging.debug('Skipping %s – non-visual content detected (text=%r)', key, en_text)
            continue
        if _appears_in_non_visual_context(updated_source, key):
            logging.debug('Skipping %s – used in non-visual context', key)
            continue
        filtered.append(entry)
    return filtered

# ---- Responses API call (tool-calling) --------------------------------------

def _normalise_content_parts(content: Any) -> List[Dict[str, str]]:
    parts: List[Dict[str, str]] = []
    if isinstance(content, list):
        for item in content:
            if isinstance(item, dict):
                text = item.get('text')
                if text is None and item.get('type') == 'input_text':
                    text = item.get('text')
                if text is not None:
                    parts.append({'text': str(text)})
            elif item is not None:
                parts.append({'text': str(item)})
    elif content is not None:
        parts.append({'text': str(content)})
    return parts


def _build_gemini_payload(messages: List[Dict[str, Any]]) -> Dict[str, Any]:
    system_texts: List[str] = []
    contents: List[Dict[str, Any]] = []

    for message in messages:
        role = message.get('role', 'user')
        parts = _normalise_content_parts(message.get('content'))
        if not parts:
            continue
        if role == 'system':
            system_texts.extend(part['text'] for part in parts if 'text' in part)
            continue
        contents.append({'role': role, 'parts': parts})

    payload: Dict[str, Any] = {
        'contents': contents,
        'tools': [{'functionDeclarations': GEMINI_FUNCTION_DECLARATIONS}],
        'toolConfig': {
            'functionCallingConfig': {
                'mode': 'ANY',
                'allowedFunctionNames': [TRANSLATION_TOOL['name']],
            }
        },
        'generationConfig': {'maxOutputTokens': 8192},
    }

    if system_texts:
        payload['systemInstruction'] = {
            'role': 'system',
            'parts': [{'text': '\n\n'.join(system_texts)}],
        }

    return payload


def _extract_translation_payload(data: Dict[str, Any]) -> Dict[str, Any]:
    candidates = data.get('candidates') or []
    for candidate in candidates:
        content = candidate.get('content') or {}
        parts = content.get('parts') or []
        for part in parts:
            function_call = part.get('functionCall')
            if function_call and function_call.get('name') == TRANSLATION_TOOL['name']:
                args = function_call.get('args') or {}
                if isinstance(args, str):
                    args_json = args
                else:
                    try:
                        args_json = json.dumps(args)
                    except (TypeError, ValueError) as exc:
                        raise RuntimeError(f'Invalid function arguments returned: {exc}') from exc
                model_obj = TranslationPayload.model_validate_json(args_json)
                return model_obj.model_dump()
            text = part.get('text')
            if isinstance(text, str):
                cleaned = _coerce_json_snippet(text)
                if cleaned is None:
                    continue
                try:
                    model_obj = TranslationPayload.model_validate_json(cleaned)
                    return model_obj.model_dump()
                except ValidationError:
                    continue
        fallback = _coerce_from_finish_message(candidate)
        if fallback is not None:
            return fallback
    logging.debug(
        "Gemini response lacked emit_translation call. Candidates: %s",
        json.dumps(candidates, ensure_ascii=False)[:8000],
    )
    raise RuntimeError(
        "Model did not return a function call to emit_translation or valid JSON text."
    )


def _coerce_json_snippet(text: str) -> Optional[str]:
    stripped = text.strip()
    if not stripped:
        return None
    if stripped.startswith('```') and stripped.endswith('```'):
        stripped = stripped.strip('`').strip()
        if stripped.startswith('json'):
            stripped = stripped[4:].strip()
    if stripped.startswith('```') and '```' in stripped[3:]:
        inner = stripped.split('```', 2)[1]
        return inner.strip()
    if stripped.startswith('{') and stripped.endswith('}'):
        return stripped
    start = stripped.find('{')
    end = stripped.rfind('}')
    if start != -1 and end != -1 and end > start:
        candidate = stripped[start:end + 1].strip()
        if candidate:
            return candidate
    return None


def _coerce_from_finish_message(candidate: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    finish_message = candidate.get('finishMessage')
    if not isinstance(finish_message, str):
        return None
    for payload in _iter_emit_translation_payloads(finish_message):
        try:
            return TranslationPayload.model_validate(payload).model_dump()
        except ValidationError:
            continue
    return None


def _iter_emit_translation_payloads(text: str) -> Iterable[Dict[str, Any]]:
    for match in re.finditer(r'emit_translation\s*\((.*?)\)', text, re.DOTALL):
        args_src = match.group(1)
        expr_src = f"emit_translation({args_src})"
        try:
            node = ast.parse(expr_src, mode='eval').body  # type: ignore[attr-defined]
        except SyntaxError:
            continue
        if not isinstance(node, ast.Call):
            continue
        # Prefer keyword arguments
        payload: Dict[str, Any] = {}
        keyword_failed = False
        for kw in node.keywords:
            if kw.arg is None:
                keyword_failed = True
                break
            try:
                payload[kw.arg] = ast.literal_eval(kw.value)
            except Exception:
                keyword_failed = True
                break
        if not keyword_failed and payload:
            yield payload
            continue
        # Fallback: single dict positional argument
        if node.args and len(node.args) == 1:
            try:
                candidate = ast.literal_eval(node.args[0])
            except Exception:
                continue
            if isinstance(candidate, dict):
                yield candidate

async def request_translation(
    client: httpx.AsyncClient,
    rel_path: str,
    source: str,
    existing_catalogue: Dict[str, Dict[str, str]],
) -> Dict[str, Any]:
    messages = build_prompt(rel_path, source, existing_catalogue)

    for attempt in range(1, MAX_TRANSLATION_RETRIES + 1):
        try:
            payload = _build_gemini_payload(messages)
            response = await client.post(
                f"/models/{MODEL_NAME}:generateContent",
                json=payload,
            )
            if response.status_code >= 400:
                logging.warning(
                    "Gemini returned %s for %s: %s",
                    response.status_code,
                    rel_path,
                    response.text.strip()[:500],
                )
            response.raise_for_status()
            data = response.json()
            prompt_feedback = data.get('promptFeedback') or {}
            block_reason = prompt_feedback.get('blockReason')
            if block_reason:
                raise RuntimeError(f"Gemini blocked the request ({block_reason}).")

            translation_payload = _extract_translation_payload(data)
            return translation_payload

        except (httpx.HTTPStatusError, httpx.RequestError) as exc:
            logging.warning("Attempt %s failed for %s due to HTTP error: %s", attempt, rel_path, exc)
            if attempt == MAX_TRANSLATION_RETRIES:
                raise
            await asyncio.sleep(RETRY_BACKOFF_SECONDS * attempt)
        except Exception as exc:
            logging.warning("Attempt %s failed for %s: %s", attempt, rel_path, exc)
            if attempt == MAX_TRANSLATION_RETRIES:
                raise
            await asyncio.sleep(RETRY_BACKOFF_SECONDS * attempt)
            # Nudge the model towards valid structured output on retry
            messages.append(
                {
                    "role": "system",
                    "content": (
                        "You must call the function emit_translation exactly once with arguments "
                        'matching the schema {"updated_source": string, "translations": array of {"key","en","cs"}} '
                        "and avoid any extra commentary."
                    ),
                }
            )

    raise RuntimeError(f"Exhausted translation attempts for {rel_path}.")

# ---- Merge & log -------------------------------------------------------------

def merge_translations(
    english: Dict[str, str],
    czech: Dict[str, str],
    payload: Iterable[Dict[str, str]],
) -> None:
    for entry in payload:
        if not isinstance(entry, dict):
            logging.warning('Skipping malformed translation entry (expected dict, got %r)', entry)
            continue

        key = entry.get('key')
        en_text = entry.get('en')
        cs_text = entry.get('cs')

        if not key:
            logging.warning('Skipping translation entry missing "key": %r', entry)
            continue

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

# ---- Main per-file worker ----------------------------------------------------

async def process_file(
    client: httpx.AsyncClient,
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
            if key.startswith(f'{rel_path}::') and should_translate_entry(key, english_catalogue.get(key, ''))
        }

        try:
            payload = await request_translation(client, rel_path, source, existing_entries)
        except Exception as exc:  # noqa: BLE001 - preserve stack for debugging
            logging.exception('Failed to translate %s: %s', rel_path, exc)
            return

        updated_source = payload.get('updated_source')
        translations = payload.get('translations', [])

        if not isinstance(updated_source, str):
            raise RuntimeError(f'Response for {rel_path} is missing the updated source code.')
        if not isinstance(translations, list):
            raise RuntimeError(f'Response for {rel_path} returned invalid translations payload.')

        translations = filter_visual_translations(updated_source, translations)

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
                asyncio.to_thread(
                    STATE_FILE.write_text,
                    json.dumps(state_payload, ensure_ascii=False, indent=2) + '\n',
                    'utf-8',
                ),
            )

        logging.info('Finished %s', rel_path)

# ---- Entrypoint --------------------------------------------------------------

async def main_async(args: argparse.Namespace) -> None:
    load_env(ROOT / '.env')

    api_key = os.environ.get('GEMINI_API_KEY') or os.environ.get('GOOGLE_API_KEY')
    if not api_key:
        raise EnvironmentError('GEMINI_API_KEY (or GOOGLE_API_KEY) is not set. Add it to your environment or .env file.')

    ensure_logging(verbose=args.verbose)

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

    timeout = httpx.Timeout(120.0, connect=30.0)
    async with httpx.AsyncClient(
        base_url=GEMINI_API_BASE,
        headers={'x-goog-api-key': api_key},
        timeout=timeout,
    ) as client:
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
        STATE_FILE.write_text(
            json.dumps(state.to_json(), ensure_ascii=False, indent=2) + '\n',
            encoding='utf-8',
        )
        logging.info('Translation completed successfully.')


def parse_args(argv: Optional[Iterable[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description='Translate the frontend into Czech using Google Gemini (function calling).')
    parser.add_argument('--dry-run', action='store_true', help='Run all requests without modifying files.')
    parser.add_argument('--verbose', action='store_true', help='Enable verbose logging output.')
    parser.add_argument('--concurrency', type=int, default=CONCURRENCY, help='Maximum number of concurrent translation requests.')
    return parser.parse_args(list(argv) if argv is not None else None)


def main(argv: Optional[Iterable[str]] = None) -> None:
    args = parse_args(argv)
    asyncio.run(main_async(args))


if __name__ == '__main__':
    main()
