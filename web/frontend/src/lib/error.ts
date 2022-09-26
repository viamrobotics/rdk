import type { ServiceError } from '../gen/proto/stream/v1/stream_pb_service.esm';
import { toast } from './toast';

export const displayError = (error: ServiceError | null) => {
  if (error) {
    toast.error(`${error}`);
  }
};
