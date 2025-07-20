import { loadPyodide } from 'pyodide';

let pyodide: any = null;
let stdoutBuffer: string[] = [];
let stderrBuffer: string[] = [];

async function ensurePyodide() {
  if (pyodide) return;
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

self.onmessage = async (e: MessageEvent) => {
  const { id, type, code } = e.data as { id: number; type: string; code?: string };
  if (type === 'run') {
    await ensurePyodide();
    stdoutBuffer = [];
    stderrBuffer = [];
    let result: any = null;
    let images: string[] = [];
    try {
      result = await pyodide.runPythonAsync(code!);
      try {
        images = pyodide.runPython(`
import base64, io
import matplotlib.pyplot as plt
_imgs = []
for _num in plt.get_fignums():
    _buf = io.BytesIO()
    plt.figure(_num).savefig(_buf, format='png')
    _imgs.append(base64.b64encode(_buf.getvalue()).decode('utf-8'))
plt.close('all')
_imgs
        `).toJs();
      } catch (err) {
        // ignore figure extraction errors
      }
    } catch (err) {
      if (stderrBuffer.length === 0) {
        stderrBuffer.push(String(err));
      }
    }
    self.postMessage({
      id,
      result,
      stdout: stdoutBuffer.join('\n'),
      stderr: stderrBuffer.join('\n'),
      images
    });
  }
};
