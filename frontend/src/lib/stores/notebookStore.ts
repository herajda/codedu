import { writable } from "svelte/store";
import type { Notebook, NotebookCell } from "./notebook";
import { v4 as uuid } from "uuid";

export const notebookStore = writable<Notebook | null>(null);

export function moveCellUp(index: number) {
  notebookStore.update((nb) => {
    if (!nb || index <= 0 || index >= nb.cells.length) return nb;
    const cells = [...nb.cells];
    [cells[index - 1], cells[index]] = [cells[index], cells[index - 1]];
    return { ...nb, cells };
  });
}

export function moveCellDown(index: number) {
  notebookStore.update((nb) => {
    if (!nb || index < 0 || index >= nb.cells.length - 1) return nb;
    const cells = [...nb.cells];
    [cells[index], cells[index + 1]] = [cells[index + 1], cells[index]];
    return { ...nb, cells };
  });
}

export function insertCell(
  index: number,
  type: "code" | "markdown",
  position: "above" | "below"
) {
  notebookStore.update((nb) => {
    if (!nb) return nb;
    const cells = [...nb.cells];
    const newCell: NotebookCell = {
      id: uuid(),
      cell_type: type,
      source: "",
      outputs: []
    };
    const insertIndex = position === "above" ? index : index + 1;
    cells.splice(insertIndex, 0, newCell);
    return { ...nb, cells };
  });
}

export function deleteCell(index: number) {
  notebookStore.update((nb) => {
    if (!nb || index < 0 || index >= nb.cells.length) return nb;
    const cells = [...nb.cells];
    cells.splice(index, 1);
    return { ...nb, cells };
  });
}
