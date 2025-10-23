import { marked } from 'marked';
import markedKatex from 'marked-katex-extension';

const globalKey = Symbol.for('codedu.marked.katex');
const globalRegistry = globalThis as Record<string | symbol, unknown>;

if (!globalRegistry[globalKey]) {
  marked.use(
    markedKatex({
      throwOnError: false
    })
  );
  globalRegistry[globalKey] = true;
}

export { marked };

export function renderMarkdown(input: string | string[] | null | undefined): string {
  const source = Array.isArray(input) ? input.join('') : input ?? '';
  return marked.parse(source) as string;
}
