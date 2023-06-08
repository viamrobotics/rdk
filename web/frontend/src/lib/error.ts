import type { ServiceError } from '@viamrobotics/sdk';
import { toast } from './toast';

const nonUserErrors = new Set(['Response closed without headers']);

export const isServiceError = (error: unknown): boolean => {
  return error instanceof Object && error && 'message' in error && 'code' in error && 'metadata' in error;
};

export const displayError = (error: ServiceError | string | null) => {
  if (typeof error === 'string') {
    if (!nonUserErrors.has(error)) {
      toast.error(error);
    }
    console.error(error);
  } else if (isServiceError(error)) {
    const serviceError = error as ServiceError;
    if (!nonUserErrors.has(serviceError.message)) {
      toast.error(serviceError.message);
    }
    console.error(serviceError);
  }
};
