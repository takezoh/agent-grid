export type VimMotionState = {
  line: number;
  pendingG: boolean;
  searchOpen: boolean;
  searchQuery: string;
  matchIndices: number[];
  matchCursor: number;
};

export type VimMotionCallbacks = {
  getLineCount: () => number;
  scrollToLine: (line: number) => void;
  getSearchableText: () => string;
};

export const MUTATION_KEYS = new Set(["i", "o", "a", "I", "A", "O", "s", "S", "c", "C"]);

export function createVimMotionState(): VimMotionState {
  return {
    line: 0,
    pendingG: false,
    searchOpen: false,
    searchQuery: "",
    matchIndices: [],
    matchCursor: -1,
  };
}

export function isMutationKey(key: string, state: VimMotionState): boolean {
  if (MUTATION_KEYS.has(key)) return true;
  if (key === ":" && state.searchQuery.startsWith("w")) return true;
  return false;
}

function findLineMatches(text: string, query: string): number[] {
  if (!query) return [];
  const lines = text.split("\n");
  const hits: number[] = [];
  for (let i = 0; i < lines.length; i++) {
    if (lines[i]?.includes(query)) hits.push(i);
  }
  return hits;
}

export type VimKeyResult =
  | { handled: true; preventDefault: true; state: VimMotionState }
  | { handled: false; preventDefault: boolean; state: VimMotionState };

export function dispatchVimKey(
  key: string,
  state: VimMotionState,
  cb: VimMotionCallbacks,
): VimKeyResult {
  if (state.searchOpen) {
    if (key === "Enter") {
      const text = cb.getSearchableText();
      const matches = findLineMatches(text, state.searchQuery);
      const next = { ...state, searchOpen: false, matchIndices: matches, matchCursor: 0 };
      if (matches.length > 0) {
        cb.scrollToLine(matches[0] ?? 0);
        next.line = matches[0] ?? 0;
      }
      return { handled: true, preventDefault: true, state: next };
    }
    if (key === "Escape") {
      return {
        handled: true,
        preventDefault: true,
        state: { ...state, searchOpen: false, searchQuery: "" },
      };
    }
    if (key === "Backspace") {
      return {
        handled: true,
        preventDefault: true,
        state: { ...state, searchQuery: state.searchQuery.slice(0, -1) },
      };
    }
    if (key.length === 1) {
      return {
        handled: true,
        preventDefault: true,
        state: { ...state, searchQuery: state.searchQuery + key },
      };
    }
    return { handled: false, preventDefault: true, state };
  }

  if (key === "/") {
    return {
      handled: true,
      preventDefault: true,
      state: { ...state, searchOpen: true, searchQuery: "" },
    };
  }

  if (key === "n" || key === "N") {
    if (state.matchIndices.length === 0) {
      return { handled: true, preventDefault: true, state };
    }
    const delta = key === "n" ? 1 : -1;
    let cursor = state.matchCursor + delta;
    if (cursor < 0) cursor = state.matchIndices.length - 1;
    if (cursor >= state.matchIndices.length) cursor = 0;
    const line = state.matchIndices[cursor] ?? state.line;
    cb.scrollToLine(line);
    return {
      handled: true,
      preventDefault: true,
      state: { ...state, matchCursor: cursor, line },
    };
  }

  const lineCount = cb.getLineCount();
  if (key === "j") {
    const line = Math.min(state.line + 1, Math.max(0, lineCount - 1));
    cb.scrollToLine(line);
    return { handled: true, preventDefault: true, state: { ...state, line, pendingG: false } };
  }
  if (key === "k") {
    const line = Math.max(state.line - 1, 0);
    cb.scrollToLine(line);
    return { handled: true, preventDefault: true, state: { ...state, line, pendingG: false } };
  }
  if (key === "g") {
    if (state.pendingG) {
      cb.scrollToLine(0);
      return { handled: true, preventDefault: true, state: { ...state, line: 0, pendingG: false } };
    }
    return { handled: true, preventDefault: true, state: { ...state, pendingG: true } };
  }
  if (key === "G") {
    const last = Math.max(0, lineCount - 1);
    cb.scrollToLine(last);
    return {
      handled: true,
      preventDefault: true,
      state: { ...state, line: last, pendingG: false },
    };
  }

  if (isMutationKey(key, state)) {
    return { handled: true, preventDefault: true, state };
  }

  return { handled: false, preventDefault: true, state };
}

export function attachWorkspaceVimKeymap(
  el: HTMLElement,
  cb: VimMotionCallbacks,
  onStateChange?: (state: VimMotionState) => void,
): () => void {
  let state = createVimMotionState();
  let ddPending = false;

  const handler = (ev: KeyboardEvent) => {
    if (!el.contains(document.activeElement) && document.activeElement !== el) return;
    const key = ev.key;
    if (key === "d" && !state.searchOpen) {
      if (ddPending) {
        ev.preventDefault();
        ev.stopPropagation();
        ddPending = false;
        return;
      }
      ddPending = true;
      ev.preventDefault();
      ev.stopPropagation();
      return;
    }
    ddPending = false;

    const result = dispatchVimKey(key, state, cb);
    state = result.state;
    onStateChange?.(state);
    if (result.preventDefault) {
      ev.preventDefault();
      ev.stopPropagation();
    }
  };

  el.addEventListener("keydown", handler, true);
  return () => el.removeEventListener("keydown", handler, true);
}
