export const addResizeListeners = () => {
  const container = document.querySelector('#app')!;

  const observer = new ResizeObserver(() => {
    window.postMessage(JSON.stringify({
      event: 'scroll-height',
      height: container.scrollHeight,
    }));
  });

  observer.observe(container);
};
