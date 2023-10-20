import type { ServiceError } from '@viamrobotics/sdk';
import { notify } from '@viamrobotics/prime';

const nonUserErrors = new Set(['Response closed without headers']);

export const isServiceError = (error: unknown): boolean => {
  return error instanceof Object && 'message' in error;
};

export const displayError = (error: ServiceError | string | null) => {
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
