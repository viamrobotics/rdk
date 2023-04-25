import './index.css';
import { createApp } from 'vue';
import RemoteControlCards, { RemoteControlCardsProps } from './components/remote-control-cards.vue';

export const createRcApp = (props: RemoteControlCardsProps) =>
  createApp(RemoteControlCards, { ...props });
