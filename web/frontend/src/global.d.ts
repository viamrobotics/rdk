declare global {
  interface Window {
    // Google
    googleMapsInit: () => void;

    /*
     * Our variables. @TODO: Remove most if not all of these. Do not add more.
     * This is an anti-pattern.
     */
    host: string;
    bakedAuth: {
      authEntity: string;
      creds: import('@viamrobotics/rpc/src/dial').Credentials;
    };
    rcDebug: boolean;
    supportedAuthTypes: string[];
    webrtcAdditionalICEServers: { urls: string; }[];
    webrtcEnabled: boolean;
    webrtcSignalingAddress: string;
  }
}

export { };
