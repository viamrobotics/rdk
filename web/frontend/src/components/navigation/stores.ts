import { currentWritable } from '@threlte/core';
import type { Obstacle } from '@/api/navigation';
import { persisted } from '@/stores/persisted';

export const obstacles = currentWritable<Obstacle[]>([]);

/** The currently selected tab. */
export const tab = persisted<'Obstacles' | 'Waypoints'>('cards.navigation.tab', 'Waypoints');
