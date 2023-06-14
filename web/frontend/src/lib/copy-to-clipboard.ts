import { notify } from '@viamrobotics/prime';

export const copyToClipboard = async (str: string) => {
  try {
    await navigator.clipboard.writeText(str);
    notify.success('Successfully copied to clipboard');
  } catch {
    notify.error('Unable to copy to clipboard');
  }
};
