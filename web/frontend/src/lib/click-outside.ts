let callback: () => void = () => {
  // do nothing
};

export const clickOutside = (element: HTMLElement, cb: () => void) => {
  callback = cb;

  const onClick = (event: MouseEvent) => {
    if (event.target instanceof Node && !element.contains(event.target)) {
      callback();
    }
  };

  document.body.addEventListener('click', onClick);

  return {
    update (updatedCb: () => void) {
      callback = updatedCb;
    },
    destroy () {
      document.body.removeEventListener('click', onClick);
    },
  };
};
