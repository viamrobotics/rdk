import { persisted } from '@viamrobotics/prime-core';

/** The currently selected tab. */
export const tab = persisted<'Obstacles' | 'Waypoints'>('cards.navigation.tab', 'Waypoints');
