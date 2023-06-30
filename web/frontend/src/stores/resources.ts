import { writable, derived } from 'svelte/store';
import type { commonApi } from '@viamrobotics/sdk';
import { sortByName } from '@/lib/resource';
import { currentWritable } from '@threlte/core';

type Resource = commonApi.ResourceName.AsObject

export const resources = currentWritable<Resource[]>([]);
export const statuses = writable<Record<string, unknown>>({});

export const rdkResources = derived(resources, (values) =>
  values.filter(({ namespace }) => namespace === 'rdk').sort(sortByName));

export const components = derived(rdkResources, (values) =>
  values.filter(({ type }) => type === 'component'));

export const services = derived(rdkResources, (values) =>
  values.filter(({ type }) => type === 'service'));
