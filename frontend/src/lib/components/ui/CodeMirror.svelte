<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter } from '@codemirror/view';
import { EditorState, type Extension } from '@codemirror/state';
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
import { syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language';
import { autocompletion, completionKeymap, closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete';
import { bracketMatching } from '@codemirror/language';
import { oneDark } from '@codemirror/theme-one-dark';

  export let value: string = '';
  export let lang: Extension | null = null;
  export let readOnly: boolean = false;

  const dispatch = createEventDispatcher();
  let host: HTMLDivElement;
  let view: EditorView;

  onMount(() => {
    const extensions: Extension[] = [
      lineNumbers(),
      highlightActiveLineGutter(),
      history(),
      bracketMatching(),
      closeBrackets(),
      autocompletion(),
      keymap.of([
        ...defaultKeymap,
        indentWithTab,
        ...historyKeymap,
        ...completionKeymap,
        ...closeBracketsKeymap
      ]),
      oneDark,
      syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
      highlightActiveLine(),
      EditorView.updateListener.of((v) => {
        if (v.docChanged) {
          value = v.state.doc.toString();
          dispatch('change', value);
        }
      }),
      EditorView.editable.of(!readOnly)
    ];
    if (lang) extensions.push(lang);
    view = new EditorView({
      state: EditorState.create({ doc: value, extensions }),
      parent: host
    });
  });

  onDestroy(() => {
    view?.destroy();
  });

  $: if (view && value !== view.state.doc.toString()) {
    const len = view.state.doc.length;
    view.dispatch({ changes: { from: 0, to: len, insert: value } });
  }
</script>

<div bind:this={host} class="border bg-gray-50 dark:bg-zinc-700"></div>
