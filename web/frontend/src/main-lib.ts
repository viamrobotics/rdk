import './index.css';
import type { Credentials } from '@viamrobotics/rpc';
import type { RCOverrides } from './rc-override-types';
import RemoteControlCards from './components/remote-control-cards.svelte';

export const createRcApp = (props: {
  host: string;
  supportedAuthTypes: string[];
  webrtcEnabled: boolean;
  signalingAddress: string,
  bakedAuth?: { authEntity: string; creds: Credentials; };
  overrides?: RCOverrides,
}) => new RemoteControlCards({
  target: document.querySelector('#app')!,
  ...props,
});
