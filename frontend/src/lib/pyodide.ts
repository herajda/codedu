import { writable, get } from "svelte/store";

let worker: Worker | null = null;
let msgId = 0;
const pending = new Map<number, (res: PythonRunResult) => void>();
let preloadPosted = false;

function ensureWorker() {
  if (!worker) {
    worker = new Worker(new URL('./pyWorker.ts', import.meta.url), { type: 'module' });
    worker.onmessage = (e: MessageEvent) => {
      const { id, ...res } = e.data as any;
      const cb = pending.get(id);
      if (cb) {
        pending.delete(id);
        cb(res as PythonRunResult);
      }
    };
  }
}

export interface PythonRunResult {
  result: any;
  stdout: string;
  stderr: string;
  images?: string[];
  resultText?: string;
}

type PythonAPI = {
  /** Legacy: raw pyodide.runPythonAsync passthrough (no captured IO). */
  runPython: (code: string) => Promise<any>;
  /** Preferred: run + capture stdout/stderr. */
  runCell: (code: string, stdin?: string) => Promise<PythonRunResult>;
  globals: any;
};

export const pyodideStore = writable<PythonAPI | null>(null);

/**
 * Dynamically load the Pyodide bundle from CDN if it has not been loaded yet.
 * SSR is disabled on the notebook route, so this always runs in the browser.
 */
let loadingPromise: Promise<PythonAPI> | null = null;

export function preloadPyodide() {
  ensureWorker();
  if (!preloadPosted) {
    worker.postMessage({ type: 'init' });
    preloadPosted = true;
  }
}

export async function initPyodide(): Promise<PythonAPI> {
  const current = get(pyodideStore);
  if (current) return current;
  if (loadingPromise) return loadingPromise;

  loadingPromise = (async () => {
    ensureWorker();
    if (!preloadPosted) {
      worker.postMessage({ type: 'init' });
      preloadPosted = true;
    }

    async function runCell(code: string, stdin?: string): Promise<PythonRunResult> {
      return new Promise((resolve) => {
        const id = msgId++;
        pending.set(id, resolve);
        worker!.postMessage({ id, type: 'run', code, stdin });
      });
    }

    const api: PythonAPI = {
      runPython: async (code) => {
        const { result, resultText } = await runCell(code);
        return result !== undefined ? result : resultText;
      },
      runCell,
      globals: {}
    };
    pyodideStore.set(api);
    return api;
  })();

  return loadingPromise;
}

export function terminatePyodide() {
  if (worker) {
    worker.terminate();
    worker = null;
  }
  for (const cb of pending.values()) {
    cb({ result: null, stdout: '', stderr: 'Execution interrupted' });
  }
  pending.clear();
  loadingPromise = null;
  preloadPosted = false;
  pyodideStore.set(null);
}
