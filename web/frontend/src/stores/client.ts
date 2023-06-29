import type { Client } from '@viamrobotics/sdk';
import { currentWritable } from '@threlte/core';

export const client = currentWritable<Client>(null!);
