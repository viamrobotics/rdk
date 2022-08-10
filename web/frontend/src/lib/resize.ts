export const addResizeListeners = () => {
  const container = document.querySelector('#app')!;

  const observer = new ResizeObserver(() => {
    window.parent.postMessage(JSON.stringify({
      event: 'scroll-height',
      height: container.scrollHeight,
    }));
  });

  observer.observe(container);
};
