/*
 * TODO: fix typecheck errors in https://viam.atlassian.net/browse/RSDK-1897
 * @ts-nocheck
 */
export const rcLogConditionally = (req: unknown) => {
  if (window.rcDebug) {
    console.log('gRPC call:', req);
  }
};
