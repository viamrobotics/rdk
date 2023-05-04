export const scheduleAsyncPoll = (callback: () => Promise<void>, interval: number) => {
  let cancelled = false;
  let timeoutId = -1;

  const refreshAndScheduleNext = async () => {
    await callback();

    // eslint-disable-next-line no-use-before-define
    scheduleNext();
  };

  const scheduleNext = () => {
    if (cancelled) {
      return;
    }

    timeoutId = window.setTimeout(refreshAndScheduleNext, interval);
  };

  const cancel = () => {
    cancelled = true;
    window.clearTimeout(timeoutId);
  };

  scheduleNext();

  return cancel;
};
