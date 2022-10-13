/* eslint-disable spaced-comment, multiline-comment-style */
/// <reference types="@types/google.maps" />
/// <reference types="@cypress" />
/// <reference types="vite/client" />
/// <reference types="vue/macros-global" />

declare global {
  interface Window {
    // Google
    googleMapsInit: () => void;

    /*
     * Our variables. @TODO: Remove most if not all of these. Do not add more.
     * This is an anti-pattern.
     */
    bakedAuth: {
      authEntity: string;
      creds: import('@viamrobotics/rpc/src/dial').Credentials;
    };
    connect: (
      authEntity?: string,
      creds?: import('@viamrobotics/rpc/src/dial').Credentials
    ) => Promise<void>;
    rcDebug: boolean;
    supportedAuthTypes: string[];
    webrtcAdditionalICEServers: { urls: string; }[];
    webrtcEnabled: boolean;
    webrtcHost: string;
    webrtcSignalingAddress: string;
  }
}

export { };
