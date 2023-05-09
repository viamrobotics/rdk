export const scheduleAsyncPoll = (callback: () => Promise<void>, interval: number) => {
  let cancelled = false;
  let timeoutId = -1;

  const refreshAndScheduleNext = async () => {
    console.log('here')
    await callback();

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
