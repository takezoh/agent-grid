// SessionContextMenu — right-click menu on a session row in SessionList.
//
// Wraps a single session row (the trigger child must be a plain DOM element;
// Radix Slot merges the contextmenu handler + ref onto it) and shows
// session-scoped actions:
//   - Open           → onOpen(sessionId)         (same as left-click activate)
//   - Copy session ID → clipboard + info toast
//   - Stop session…  → onRequestTerminate(id, label, opener); the ConfirmDialog
//     flow is owned by App (same contract as SessionTerminateButton).
//
// Reuses the .overflow-menu CSS block (FR-012) so dropdown and context menus
// share one visual language.

import * as ContextMenu from "@radix-ui/react-context-menu";
import type { JSX, ReactNode } from "react";
import { useRef } from "react";
import { useNotificationsStore } from "../store/notifications";

export interface SessionContextMenuProps {
  sessionId: string;
  /** Confirm dialog title (titleText(card)). */
  sessionLabel: string;
  /** Disables Open / Stop while the daemon connection is down. */
  daemonDisconnected: boolean;
  onOpen: (sessionId: string) => void;
  /** Absent when the host view has no terminate flow wired (e.g. tests). */
  onRequestTerminate?: (sessionId: string, label: string, opener: HTMLElement) => void;
  children: ReactNode;
}

async function copySessionId(sessionId: string): Promise<void> {
  const add = useNotificationsStore.getState().add;
  try {
    await navigator.clipboard.writeText(sessionId);
    add({ level: "info", message: `Session ID copied: ${sessionId}` });
  } catch {
    // clipboard API missing (insecure context) or permission denied — the
    // toast carries the ID itself so the user can still select-copy it.
    add({ level: "warn", message: `Could not access clipboard. Session ID: ${sessionId}` });
  }
}

export function SessionContextMenu({
  sessionId,
  sessionLabel,
  daemonDisconnected,
  onOpen,
  onRequestTerminate,
  children,
}: SessionContextMenuProps): JSX.Element {
  // Focus-restore target for the ConfirmDialog (WCAG 2.4.3 / 3.2.1): the row
  // element the menu was opened from. Radix merges this ref onto the child.
  const triggerRef = useRef<HTMLElement>(null);

  return (
    <ContextMenu.Root>
      <ContextMenu.Trigger asChild ref={triggerRef}>
        {children}
      </ContextMenu.Trigger>
      <ContextMenu.Portal>
        <ContextMenu.Content
          className="overflow-menu session-context-menu"
          data-session-id={sessionId}
        >
          <ContextMenu.Label className="overflow-menu__label">{sessionLabel}</ContextMenu.Label>
          <ContextMenu.Item
            className="overflow-menu__item"
            disabled={daemonDisconnected}
            onSelect={() => onOpen(sessionId)}
          >
            Open
          </ContextMenu.Item>
          <ContextMenu.Item
            className="overflow-menu__item"
            onSelect={() => void copySessionId(sessionId)}
          >
            Copy session ID
          </ContextMenu.Item>
          {onRequestTerminate !== undefined && (
            <>
              <ContextMenu.Separator className="overflow-menu__separator" />
              <ContextMenu.Item
                className="overflow-menu__item overflow-menu__item--danger"
                disabled={daemonDisconnected}
                onSelect={() => {
                  const opener = triggerRef.current;
                  if (opener !== null) {
                    onRequestTerminate(sessionId, sessionLabel, opener);
                  }
                }}
              >
                Stop session…
              </ContextMenu.Item>
            </>
          )}
        </ContextMenu.Content>
      </ContextMenu.Portal>
    </ContextMenu.Root>
  );
}
