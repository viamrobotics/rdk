import { toast } from './toast';

export const copyToClipboardWithToast = async (str: string) => {
  try {
    await navigator.clipboard.writeText(str);
    toast.success('Successfully copied to clipboard');
  } catch {
    toast.error('Unable to copy to clipboard');
  }
};
