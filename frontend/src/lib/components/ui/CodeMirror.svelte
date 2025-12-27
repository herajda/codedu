<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, placeholder as cmPlaceholder } from '@codemirror/view';
  import { EditorState, type Extension, Compartment } from '@codemirror/state';
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
import { syntaxHighlighting, defaultHighlightStyle } from '@codemirror/language';
import { autocompletion, completionKeymap, closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete';
import { bracketMatching } from '@codemirror/language';
import { oneDark } from '@codemirror/theme-one-dark';

  export let value: string = '';
  export let lang: Extension | null = null;
  export let readOnly: boolean = false;

  function isDark(): boolean {
    return document.documentElement.getAttribute('data-theme') === 'dark';
  }

  const dispatch = createEventDispatcher();
  let host: HTMLDivElement;
  let view: EditorView;
  const themeCompartment = new Compartment();
  const placeholderCompartment = new Compartment();
  export let placeholder: string | null = null;

  onMount(() => {
    const placeholderExtension: Extension = placeholder ? cmPlaceholder(placeholder) : ([] as Extension);
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
      themeCompartment.of(isDark() ? oneDark : []),
      syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
      highlightActiveLine(),
      EditorView.updateListener.of((v) => {
        if (v.docChanged) {
          value = v.state.doc.toString();
          dispatch('change', value);
        }
      }),
      EditorView.editable.of(!readOnly),
      placeholderCompartment.of(placeholderExtension)
    ];
    if (lang) extensions.push(lang);
    view = new EditorView({
      state: EditorState.create({ doc: value, extensions }),
      parent: host
    });

    const obs = new MutationObserver(() => {
      const wantsDark = isDark();
      view.dispatch({ effects: themeCompartment.reconfigure(wantsDark ? oneDark : []) });
    });
    obs.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    // store observer for cleanup
    (view as any)._themeObserver = obs;
  });

  onDestroy(() => {
    const obs: MutationObserver | undefined = (view as any)?._themeObserver;
    obs?.disconnect();
    view?.destroy();
  });

  $: if (view && value !== view.state.doc.toString()) {
    const len = view.state.doc.length;
    view.dispatch({ changes: { from: 0, to: len, insert: value } });
  }

  $: if (view) {
    const nextPlaceholder: Extension = placeholder ? cmPlaceholder(placeholder) : ([] as Extension);
    view.dispatch({ effects: placeholderCompartment.reconfigure(nextPlaceholder) });
  }
</script>

<div bind:this={host} class="rounded-lg"></div>
