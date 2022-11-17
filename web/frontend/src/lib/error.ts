import type { ServiceError } from '@viamrobotics/sdk';
import { toast } from './toast';

export const displayError = (error: ServiceError | null) => {
  if (error) {
    toast.error(error.message);
    console.error(error);
  }
};
