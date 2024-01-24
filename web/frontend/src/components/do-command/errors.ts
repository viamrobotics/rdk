import get from 'lodash-es/get';

const GRPC_MESSAGE_KEY = 'grpc-message';

export interface GRPCUnimplementedError extends Error {
  metadata: {
    headersMap: {
      [GRPC_MESSAGE_KEY]: string[];
    };
  };
}

export const isUnimplementedError = (error: unknown) => {
  const errorMessages = get(
    error,
    `metadata.headersMap.${GRPC_MESSAGE_KEY}`,
    [] as string[]
  );

  return errorMessages.includes('DoCommand unimplemented');
};
