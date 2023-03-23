import type { ServiceError } from '@viamrobotics/sdk';
import { toast } from './toast';

export const isServiceError = (error: unknown): boolean => {
  return error instanceof Object && error && 'message' in error && 'code' in error && 'metadata' in error;
};

export const displayError = (error: ServiceError | string | null) => {
  if (error instanceof String) {
    toast.error(error as string);
    console.error(error);
  } else if (isServiceError(error)) {
    const serviceError = error as ServiceError;
    toast.error(serviceError.message);
    console.error(serviceError);
  }
};
