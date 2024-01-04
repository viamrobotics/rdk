const GRPC_MESSAGE_KEY = 'grpc-message';

export interface GRPCUnimplementedError {
  metadata: {
    headersMap: {
      [GRPC_MESSAGE_KEY]: string[];
    };
  };
}

export const isUnimplementedError = (error: unknown) => {
  const unimplemented = error as GRPCUnimplementedError;
  if (
    typeof unimplemented !== 'object' ||
    typeof unimplemented.metadata !== 'object' ||
    typeof unimplemented.metadata.headersMap !== 'object' ||
    unimplemented.metadata.headersMap[GRPC_MESSAGE_KEY].length === 0
  ) {
    return false;
  }

  return (error as GRPCUnimplementedError).metadata.headersMap[
    GRPC_MESSAGE_KEY
  ].includes('DoCommand unimplemented');
};
