export const clickOutside = (element: HTMLElement) => {
  let callback = () => {};

  const onClick = (event: MouseEvent) => {
    if (event.target instanceof Node && !element.contains(event.target)) {
      callback();
    }
  };

  document.body.addEventListener('click', onClick);

  return {
    update (cb: () => void) {
      callback = cb;
    },
    destroy () {
      document.body.removeEventListener('click', callback);
    },
  };
};
