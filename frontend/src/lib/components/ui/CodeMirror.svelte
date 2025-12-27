<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, placeholder as cmPlaceholder, tooltips } from '@codemirror/view';
  import { EditorState, type Extension, Compartment } from '@codemirror/state';
  import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
  import { syntaxHighlighting, defaultHighlightStyle, HighlightStyle } from '@codemirror/language';
  import { tags as t_tags } from '@lezer/highlight';
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
  const highlightCompartment = new Compartment();
  const placeholderCompartment = new Compartment();
  export let placeholder: string | null = null;

  // Modern Custom Theme
  const customTheme = EditorView.theme({
    "&": {
      fontSize: "inherit",
      fontFamily: '"JetBrains Mono", "Fira Code", "Source Code Pro", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
    },
    ".cm-content": {
      padding: "16px 0",
      lineHeight: "1.7",
    },
    ".cm-line": {
      padding: "0 16px 0 8px",
    },
    ".cm-gutters": {
      backgroundColor: "transparent",
      border: "none",
      color: "currentColor",
      opacity: "0.3",
      userSelect: "none",
      minWidth: "40px",
    },
    ".cm-gutterElement": {
      padding: "0 12px 0 8px !important",
      display: "flex",
      alignItems: "center",
      justifyContent: "flex-end",
    },
    ".cm-activeLine": {
      backgroundColor: "rgba(128, 128, 128, 0.05)",
      borderRadius: "0 4px 4px 0",
    },
    ".cm-activeLineGutter": {
      backgroundColor: "transparent",
      opacity: "1",
      color: "var(--p, #3b82f6)",
      fontWeight: "bold",
      borderRight: "2px solid var(--p, #3b82f6)",
    },
    ".cm-cursor": {
      borderLeftWidth: "2.5px",
      borderLeftColor: "var(--p, #3b82f6) !important",
    },
    "&.cm-focused": {
      outline: "none",
    },
    "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection": {
      backgroundColor: "rgba(59, 130, 246, 0.2) !important",
    },
    ".cm-panels": { backgroundColor: "var(--b1)", color: "var(--bc)" },
    ".cm-panels.cm-panels-top": { borderBottom: "2px solid var(--b2)" },
    ".cm-panels.cm-panels-bottom": { borderTop: "2px solid var(--b2)" },
    ".cm-searchMatch": {
      backgroundColor: "rgba(255, 255, 0, 0.2)",
      outline: "1px solid yellow",
    },
    ".cm-searchMatch.cm-searchMatch-selected": {
      backgroundColor: "rgba(255, 255, 0, 0.4)",
    },
  }, { dark: isDark() });

  // Light theme syntax highlighting improvement
  const lightHighlightStyle = HighlightStyle.define([
    { tag: t_tags.keyword, color: "#7e57c2", fontWeight: "bold" },
    { tag: t_tags.operator, color: "#ef5350" },
    { tag: t_tags.variableName, color: "#263238" },
    { tag: t_tags.propertyName, color: "#f57c00" },
    { tag: t_tags.comment, color: "#90a4ae", fontStyle: "italic" },
    { tag: t_tags.string, color: "#66bb6a" },
    { tag: t_tags.number, color: "#ec407a" },
    { tag: t_tags.className, color: "#29b6f6" },
    { tag: t_tags.typeName, color: "#00acc1" },
    { tag: t_tags.function(t_tags.variableName), color: "#1e88e5" },
  ]);

  onMount(() => {
    const placeholderExtension: Extension = placeholder ? cmPlaceholder(placeholder) : ([] as Extension);
    const extensions: Extension[] = [
      lineNumbers(),
      highlightActiveLineGutter(),
      history(),
      bracketMatching(),
      closeBrackets(),
      autocompletion(),
      tooltips({
        parent: document.body
      }),
      keymap.of([
        ...defaultKeymap,
        indentWithTab,
        ...historyKeymap,
        ...completionKeymap,
        ...closeBracketsKeymap
      ]),
      customTheme,
      themeCompartment.of(isDark() ? oneDark : []),
      highlightCompartment.of(syntaxHighlighting(isDark() ? defaultHighlightStyle : lightHighlightStyle, { fallback: true })),
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
      view.dispatch({ 
        effects: [
          themeCompartment.reconfigure(wantsDark ? oneDark : []),
          highlightCompartment.reconfigure(syntaxHighlighting(wantsDark ? defaultHighlightStyle : lightHighlightStyle, { fallback: true }))
        ]
      });
    });
    obs.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
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

<div bind:this={host} class="w-full h-full min-h-[4rem]"></div>

<style>
  :global(.cm-editor) {
    height: 100%;
    background: transparent !important;
  }
  :global(.cm-scroller) {
    font-family: inherit;
  }
  :global(.cm-gutters) {
    border-right: 1px solid rgba(128, 128, 128, 0.1) !important;
  }

  /* Premium Autocomplete Styling */
  :global(.cm-tooltip) {
    z-index: 10000 !important;
    border: none !important;
    background: transparent !important;
  }

  :global(.cm-tooltip-autocomplete) {
    background-color: var(--b1) !important;
    backdrop-filter: blur(12px) !important;
    color: var(--bc) !important;
    border: 1px solid var(--b2) !important;
    border-radius: 12px !important;
    box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.2), 0 10px 10px -5px rgba(0, 0, 0, 0.1) !important;
    overflow: hidden !important;
    padding: 4px !important;
    min-width: 280px !important;
  }

  :global(.cm-tooltip-autocomplete ul) {
    font-family: inherit !important;
    border: none !important;
  }

  :global(.cm-tooltip-autocomplete ul li) {
    padding: 8px 12px !important;
    border-radius: 8px !important;
    margin: 2px 0 !important;
    display: flex !important;
    align-items: center !important;
    gap: 8px !important;
    transition: all 0.2s ease !important;
  }

  :global(.cm-tooltip-autocomplete ul li[aria-selected]) {
    background-color: var(--p) !important;
    color: var(--pc) !important;
    box-shadow: 0 4px 12px hsl(var(--p) / 0.3) !important;
  }

  :global(.cm-completionLabel) {
    font-weight: 600 !important;
    font-size: 0.9rem !important;
  }

  :global(.cm-completionDetail) {
    font-size: 0.75rem !important;
    opacity: 0.6 !important;
    font-style: italic !important;
    margin-left: auto !important;
  }

  :global(.cm-completionIcon) {
    width: 20px !important;
    height: 20px !important;
    display: flex !important;
    align-items: center !important;
    justify-content: center !important;
    border-radius: 4px !important;
    background: rgba(128, 128, 128, 0.1) !important;
    font-size: 0.7rem !important;
    opacity: 0.8 !important;
  }
</style>
