/* eslint-disable multiline-comment-style */
/* eslint-disable spaced-comment */

/// <reference types="svelte" />
/// <reference types="vite/client" />

declare module '*.txt' {
  const value: string;
  export default value;
}

declare global {
  interface Window {
    /*
     * Our variables. @TODO: Remove most if not all of these. Do not add more.
     * This is an anti-pattern.
     */
    host: string;
    bakedAuth: {
      authEntity: string;
      creds: import('@viamrobotics/sdk').Credential;
    };
    rcDebug: boolean;
    supportedAuthTypes: string[];
    webrtcEnabled: boolean;
    webrtcSignalingAddress: string;
  }
}

export {};
