
import { getContext } from 'svelte';

export interface CollapseContext {
  /**
   * Fires when a stop command is issued from the navbar of the `<Collapse>` component.
   */
  onStop: (callback: () => void) => void
}

export const collapseContextKey = Symbol('Collapse context')

export const useStop = (): CollapseContext => {
  const context = getContext<CollapseContext>(collapseContextKey)

  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
  if (!context) {
    throw new Error('No `useStop` context found. This hook must be called within a child of `<Collapse>`.')
  }

  return context
}
