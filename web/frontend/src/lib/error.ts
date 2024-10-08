import { notify } from '@viamrobotics/prime';
import { ConnectError } from '@viamrobotics/sdk';

const nonUserErrors = new Set(['Response closed without headers']);

export const isServiceError = (error: unknown): boolean => {
  return (
    error instanceof ConnectError ||
    (error instanceof Object && 'message' in error)
  );
};

export const displayError = (error: ConnectError | string | null) => {
  // eslint-disable-next-line no-console
  console.error(error);
  if (typeof error === 'string') {
    if (!nonUserErrors.has(error)) {
      notify.danger(error);
    }
  } else if (isServiceError(error)) {
    const serviceError = error!;
    if (!nonUserErrors.has(serviceError.message)) {
      notify.danger(serviceError.message);
    }
  }
};
