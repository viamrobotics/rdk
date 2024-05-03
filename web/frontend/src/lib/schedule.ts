/**
 * provides setInterval-like functionality for an async function.
 * @param interval - time to wait in between calls to `callback`. We wait for callback to complete, then wait interval, so callback calls won't overlap.
 * @param ignoreErrors - if false (the default), the first error in `callback` will stop the interval.
 * @returns a 'cancel' function you can call to stop the interval. It is safe to call this multiple times.
 */
export const setAsyncInterval = (
  callback: () => Promise<void>,
  interval: number,
  ignoreErrors = false
): (() => void) => {
  let cancelled = false;
  let timeoutId = -1;

  const refreshAndScheduleNext = async () => {
    try {
      await callback();
    } catch (error) {
      if (ignoreErrors) {
        console.warn('ignoring error in setAsyncInterval', error);
      } else {
        throw error;
      }
    }

    if (cancelled) {
      return;
    }

    timeoutId = window.setTimeout(refreshAndScheduleNext, interval);
  };

  const cancel = () => {
    cancelled = true;
    window.clearTimeout(timeoutId);
  };

  timeoutId = window.setTimeout(refreshAndScheduleNext, interval);

  return cancel;
};
