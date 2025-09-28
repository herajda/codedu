import { loadPyodide } from 'pyodide';

let pyodide: any = null;
let stdoutBuffer: string[] = [];
let stderrBuffer: string[] = [];
let reprHelper: any = null;
let inputBuffer: string[] = [];
let inputPatchVersion = 0;
const CURRENT_INPUT_PATCH_VERSION = 2;

function setInputBuffer(stdin?: string | null) {
  if (!stdin) {
    inputBuffer = [];
    return;
  }
  const normalized = stdin.replace(/\r\n/g, '\n');
  const pieces = normalized.split('\n');
  // Drop a trailing empty record caused by a terminal newline to emulate stdin behaviour.
  if (pieces.length && pieces[pieces.length - 1] === '') {
    pieces.pop();
  }
  inputBuffer = pieces;
}

function exposeInputBridge() {
  // Expose a function on the JS global (Worker scope) so Python can import it via `from js import ...`.
  (self as any)._codex_pop_input = (prompt?: any) => {
    if (!inputBuffer.length) {
      return '';
    }
    return inputBuffer.shift() ?? '';
  };
  (self as any)._codex_has_input = () => inputBuffer.length > 0;
}

async function patchPythonInput() {
  if (inputPatchVersion === CURRENT_INPUT_PATCH_VERSION) return;
  exposeInputBridge();
  await pyodide.runPythonAsync(`
import builtins, base64
from js import _codex_pop_input, _codex_has_input
def _codex_input(prompt=""):
    _p = str(prompt) if prompt is not None else ""
    if not _codex_has_input():
        _enc = base64.b64encode(_p.encode('utf-8')).decode('ascii')
        raise RuntimeError('__INPUT_REQUIRED__:' + _enc)
    return _codex_pop_input(_p)
builtins.input = _codex_input
  `);
  inputPatchVersion = CURRENT_INPUT_PATCH_VERSION;
}

async function ensurePyodide() {
  if (!pyodide) {
    pyodide = await loadPyodide({
      indexURL: 'https://cdn.jsdelivr.net/pyodide/v0.28.0/full/',
      stdout: (msg: string) => {
        stdoutBuffer.push(msg);
      },
      stderr: (msg: string) => {
        stderrBuffer.push(msg);
      }
    });
    await pyodide.loadPackage(['matplotlib', 'numpy', 'pandas']);
    if (!reprHelper) {
      reprHelper = pyodide.runPython('lambda value: repr(value)');
    }
    // Use a non-GUI backend so figures can be rendered to PNG in memory.
    await pyodide.runPythonAsync(`
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
def _silent_show(*args, **kwargs):
    pass
plt.show = _silent_show
    `);
  }
  // Always ensure input() is patched, even if Pyodide was already initialized earlier.
  await patchPythonInput();
}

function isPyProxy(value: any): value is { toJs: (opts?: any) => any; destroy?: () => void } {
  return value && typeof value === 'object' && typeof value.toJs === 'function';
}

function getRepr(value: any): string | undefined {
  if (!reprHelper) return undefined;
  try {
    const reprValue = reprHelper(value);
    let text: string | undefined;
    let destroyed = false;
    if (typeof reprValue === 'string') {
      text = reprValue;
    } else if (reprValue && typeof reprValue.toJs === 'function') {
      try {
        text = reprValue.toJs();
      } finally {
        if (typeof reprValue.destroy === 'function') {
          reprValue.destroy();
          destroyed = true;
        }
      }
    } else {
      text = String(reprValue);
    }
    if (!destroyed && reprValue && typeof reprValue === 'object' && typeof reprValue.destroy === 'function') {
      reprValue.destroy();
    }
    return text;
  } catch (err) {
    return undefined;
  }
}

function serializeResult(raw: any): { value: any; text?: string } {
  if (raw === undefined) {
    return { value: undefined, text: undefined };
  }
  if (raw === null) {
    return { value: null, text: undefined };
  }

  const text = getRepr(raw);

  if (isPyProxy(raw)) {
    try {
      const converted = raw.toJs({ create_proxies: false, dict_converter: Object.fromEntries });
      return { value: makeCloneable(converted, text), text };
    } catch (err) {
      return { value: makeCloneable(text ?? String(raw)), text };
    } finally {
      if (typeof raw.destroy === 'function') {
        raw.destroy();
      }
    }
  }

  return { value: makeCloneable(raw, text ?? String(raw)), text: text ?? String(raw) };
}

function makeCloneable(value: any, fallbackText?: string, seen?: WeakMap<object, any>) {
  if (value === null || value === undefined) return value;
  const type = typeof value;
  if (type === 'number' || type === 'string' || type === 'boolean') return value;
  if (type === 'bigint') return value;
  if (type === 'function' || type === 'symbol') {
    return fallbackText ?? `[${type}]`;
  }
  if (typeof structuredClone === 'function') {
    try {
      return structuredClone(value);
    } catch (err) {
      // fall back to manual cloning below
    }
  }
  try {
    if (value && typeof value === 'object') {
      const refMap = seen ?? new WeakMap<object, any>();
      if (refMap.has(value)) {
        return refMap.get(value);
      }
      const clone: any = Array.isArray(value) ? [] : {};
      refMap.set(value, clone);
      if (Array.isArray(value)) {
        for (const item of value) {
          clone.push(makeCloneable(item, undefined, refMap));
        }
      } else {
        for (const [key, val] of Object.entries(value)) {
          clone[key] = makeCloneable(val, undefined, refMap);
        }
      }
      return clone;
    }
    return fallbackText ?? String(value);
  }
  catch (err) {
    return fallbackText ?? String(value);
  }
}

self.onmessage = async (e: MessageEvent) => {
  const { id, type, code, stdin } = e.data as { id?: number; type: string; code?: string; stdin?: string | null };
  if (type === 'init') {
    await ensurePyodide();
    return;
  }
  if (type === 'run') {
    await ensurePyodide();
    setInputBuffer(stdin ?? null);
    stdoutBuffer = [];
    stderrBuffer = [];
    let result: any = null;
    let resultText: string | undefined;
    let images: string[] = [];
    try {
      const rawResult = await pyodide.runPythonAsync(code!);
      const serialized = serializeResult(rawResult);
      result = serialized.value;
      resultText = serialized.text;
      try {
        const imageProxy = pyodide.runPython(`
import base64, io
import matplotlib.pyplot as plt
_imgs = []
for _num in plt.get_fignums():
    _buf = io.BytesIO()
    plt.figure(_num).savefig(_buf, format='png')
    _imgs.append(base64.b64encode(_buf.getvalue()).decode('utf-8'))
plt.close('all')
_imgs
        `);
        try {
          images = imageProxy.toJs();
        } finally {
          if (typeof imageProxy.destroy === 'function') {
            imageProxy.destroy();
          }
        }
      } catch (err) {
        // ignore figure extraction errors
      }
    } catch (err) {
      const errStr = String(err ?? '');
      if (errStr.includes('__INPUT_REQUIRED__')) {
        // Extract optional base64-encoded prompt: match contiguous base64 after the marker
        let promptText = '';
        try {
          const m = errStr.match(/__INPUT_REQUIRED__:\s*([A-Za-z0-9+/=]+)/);
          if (m && m[1]) {
            const bin = atob(m[1]);
            const bytes = new Uint8Array(bin.length);
            for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
            promptText = new TextDecoder('utf-8').decode(bytes);
          }
        } catch {}
        const message = {
          id,
          result: undefined,
          resultText: undefined,
          stdout: stdoutBuffer.join('\n'),
          stderr: '',
          images: [],
          inputRequired: true,
          prompt: promptText,
        } as any;
        self.postMessage(message);
        return;
      }
      if (stderrBuffer.length === 0) {
        stderrBuffer.push(errStr);
      }
    }
    const message = {
      id,
      result,
      resultText,
      stdout: stdoutBuffer.join('\n'),
      stderr: stderrBuffer.join('\n'),
      images
    };
    if (typeof structuredClone === 'function') {
      try {
        structuredClone(message);
      } catch (cloneErr) {
        console.warn('Dropping non-transferable result from Pyodide worker:', cloneErr);
        message.result = undefined;
        if (!message.resultText) {
          message.resultText = '[unserializable result]';
        }
      }
    }
    self.postMessage(message);
  }
};
