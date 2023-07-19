import './index.css';
import type { Credentials } from '@viamrobotics/rpc';
import type { RCOverrides } from './rc-override-types';
import RemoteControlCards from './components/remote-control-cards.svelte';

export const createRcApp = (target: HTMLElement, props: {
  host: string;
  bakedAuth?: { authEntity: string; creds: Credentials; };
  supportedAuthTypes?: string[];
  webrtcEnabled: boolean;
  signalingAddress: string
  overrides?: RCOverrides,
}) => new RemoteControlCards({ target, props });
