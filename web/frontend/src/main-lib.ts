import './index.css';
import RemoteControlCards from './components/remote-control-cards.svelte';
import type { ComponentProps } from 'svelte';

export const createRcApp = (
  target: HTMLElement,
  props: ComponentProps<RemoteControlCards>
) => new RemoteControlCards({ target, props });
