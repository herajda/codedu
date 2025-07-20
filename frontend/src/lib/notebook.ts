export interface NotebookCell {
  id: string;
  cell_type: "markdown" | "code";
  source: string;                    // always normalized to a plain string
  metadata?: Record<string, any>;
  outputs?: any[];                   // nbformat outputs; we'll stuff stdout/stderr/result objects in here
}

export interface Notebook {
  nbformat: number;
  nbformat_minor: number;
  metadata: Record<string, any>;
  cells: NotebookCell[];
}

/* ------------------------------------------------------------------ */
/* Helpers                                                            */
/* ------------------------------------------------------------------ */

// Generate a reasonably unique id (works in browser & Node)
function genId(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    // @ts-ignore
    return crypto.randomUUID();
  }
  return Math.random().toString(36).slice(2);
}

/** Convert nbformat `source` (string | string[] | unknown) -> plain string. */
export function cellSourceToString(src: unknown): string {
  if (Array.isArray(src)) {
    return src.join("");
  }
  return typeof src === "string" ? src : "";
}

/* ------------------------------------------------------------------ */
/* Main loader                                                        */
/* ------------------------------------------------------------------ */

export function loadNotebook(data: string): Notebook {
  const trimmed = data.trim();

  /* ---------- VS Code / XML style (your existing behavior) ---------- */
  if (trimmed.startsWith("<")) {
    const parser = new DOMParser();
    const xmlDoc = parser.parseFromString(data, "application/xml");
    const cellNodes = Array.from(xmlDoc.getElementsByTagName("VSCode.Cell"));

    const cells: NotebookCell[] = cellNodes.map((cellNode) => {
      const language = (cellNode.getAttribute("language") as "markdown" | "code") ?? "code";
      const id = cellNode.getAttribute("id") ?? genId();

      // Collect raw text + inline markup children (as before)
      let source = "";
      for (const child of cellNode.childNodes) {
        if (child.nodeType === Node.TEXT_NODE) {
          source += child.nodeValue ?? "";
        } else if (child.nodeType === Node.ELEMENT_NODE) {
          source += (child as Element).outerHTML;
        }
      }

      return {
        id,
        cell_type: language,
        source: source.trim(),
        metadata: {},
        outputs: []
      };
    });

    return {
      nbformat: 5,
      nbformat_minor: 1,
      metadata: {},
      cells
    };
  }

  /* ---------- nbformat JSON (normal .ipynb) ---------- */
  try {
    const raw = JSON.parse(data) as any;

    const rawCells: any[] = Array.isArray(raw?.cells) ? raw.cells : [];
    const cells: NotebookCell[] = rawCells.map((c) => {
      const cell_type = c.cell_type === "markdown" ? "markdown" : "code";
      return {
        id: c.id ?? genId(),
        cell_type,
        source: cellSourceToString(c.source),
        metadata: c.metadata ?? {},
        outputs: Array.isArray(c.outputs) ? c.outputs : []
      };
    });

    return {
      nbformat: typeof raw.nbformat === "number" ? raw.nbformat : 5,
      nbformat_minor: typeof raw.nbformat_minor === "number" ? raw.nbformat_minor : 1,
      metadata: raw.metadata ?? {},
      cells
    };
  } catch {
    /* ---------- Fallback: treat as plain text file ---------- */
    return {
      nbformat: 5,
      nbformat_minor: 1,
      metadata: {},
      cells: [
        {
          id: genId(),
          cell_type: "code",
          source: data,
          metadata: {},
          outputs: []
        }
      ]
    };
  }
}

/* ------------------------------------------------------------------ */
/* Serialize & factory helpers                                        */
/* ------------------------------------------------------------------ */

export function serializeNotebook(nb: Notebook): string {
  // NOTE: We're writing `source` as a single string. If you want stricter
  // nbformat compliance, you could split into lines. For now, this keeps
  // roundtripping simple for this prototype.
  return JSON.stringify(nb, null, 2);
}

// Utility to create a fresh empty notebook
export function createEmptyNotebook(): Notebook {
  return {
    nbformat: 5,
    nbformat_minor: 1,
    metadata: {},
    cells: []
  };
}
