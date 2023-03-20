
import { createMachine } from 'xstate';

interface Context {
  retries: number;
}

const sessionMachine = createMachine<Context>({
  id: 'stream',
  initial: 'idle',
  states: {
    idle: {
      on: {
        CONNECT: { target: 'connected' },
      },
    },
    connected: {
      on: {
        CLOSE: { target: 'idle' },
        DISCONNECT: { target: 'disconnected' },
      },
    },
    disconnected: {
      on: {
        CONNECT: { target: 'connected' },
        CLOSE: { target: 'idle' },
      },
    },
  },
});

console.log(sessionMachine);
