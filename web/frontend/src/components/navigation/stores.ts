import { currentWritable } from '@threlte/core';
import { persisted } from '@viamrobotics/prime-core';
import type { Obstacle } from '@viamrobotics/prime-blocks';

export const obstacles = currentWritable<Obstacle[]>([]);

/** The currently selected tab. */
export const tab = persisted<'Obstacles' | 'Waypoints'>('cards.navigation.tab', 'Waypoints');
