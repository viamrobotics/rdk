import type { RCOverrides } from '@/types/overrides';
import type { Credential } from '@viamrobotics/sdk';
import RemoteControlCards from './components/remote-control-cards.svelte';
import './index.css';

export const createRcApp = (
  target: HTMLElement,
  props: {
    host: string;
    bakedAuth?: {
      authEntity: string;
      creds: Credential;
    };
    supportedAuthTypes?: string[];
    webrtcEnabled: boolean;
    signalingAddress: string;
    overrides?: RCOverrides;
    hiddenSubtypes?: string[];
    hideDoCommand?: boolean;
    hideOperationsSessions?: boolean;
  }
) => new RemoteControlCards({ target, props });
