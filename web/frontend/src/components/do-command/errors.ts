import { ConnectError } from '@viamrobotics/sdk';

export const isUnimplementedError = (error: unknown) => {
  return (
    error instanceof ConnectError &&
    error.message.includes('DoCommand unimplemented')
  );
};
