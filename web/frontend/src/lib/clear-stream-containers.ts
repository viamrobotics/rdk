export const hideStreamContainer = (camName: string, elName: string) => {
  const streamContainer = document.querySelector(
    `[data-stream="${camName}"]`
  );
  console.log('streamContainer', streamContainer);
  if (elName === 'video') {
    streamContainer?.querySelector('video')?.classList.add('hidden');
    streamContainer?.querySelector('img')?.classList.remove('hidden');
  } else {
    streamContainer?.querySelector('img')?.classList.add('hidden');
    streamContainer?.querySelector('video')?.classList.remove('hidden');
  }
};
