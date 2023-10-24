export const rcLogConditionally = (req: unknown) => {
  if (window.rcDebug || localStorage.getItem('rc_debug')) {
    // eslint-disable-next-line no-console
    console.log('gRPC call:', req);
  }
};
