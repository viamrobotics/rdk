export const rcLogConditionally = (req: unknown) => {
  if (window.rcDebug) {
    console.log('gRPC call:', req);
  }
};
