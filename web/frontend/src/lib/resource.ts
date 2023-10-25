import type { commonApi } from '@viamrobotics/sdk';

type Resource = commonApi.ResourceName.AsObject

export const sortByName = (item1: Resource, item2: Resource) => {
  if (item1.name > item2.name) {
    return 1;
  } else if (item1.name < item2.name) {
    return -1;
  }
  if (item1.subtype > item2.subtype) {
    return 1;
  } else if (item1.subtype < item2.subtype) {
    return -1;
  }
  if (item1.type > item2.type) {
    return 1;
  } else if (item1.type < item2.type) {
    return -1;
  }
  return item1.namespace > item2.namespace ? 1 : -1;
};

export const resourceNameToSubtypeString = (resource: Resource | undefined) => {
  if (!resource) {
    return '';
  }

  return `${resource.namespace}:${resource.type}:${resource.subtype}`;
};

export const resourceNameToString = (resource: Resource | undefined) => {
  if (!resource) {
    return '';
  }

  let strName = resourceNameToSubtypeString(resource);
  if (resource.name !== '') {
    strName += `/${resource.name}`;
  }
  return strName;
};

export const filterSubtype = (
  resources: Resource[],
  subtype: string,
  options: {
    remote?: boolean;
    name?: boolean;
  } = {}
) => {
  const results: Resource[] = [];
  const { remote = true, name = false } = options;

  for (const resource of resources) {
    if (!remote && resource.name.includes(':')) {
      continue;
    }

    if (name && !resource.name) {
      continue;
    }

    if (resource.subtype === subtype) {
      results.push(resource);
    }
  }

  return results;
};

export const filterWithStatus = (
  resources: Resource[],
  status: Record<string, unknown>,
  subtype: string
) => {
  return resources
    .filter((resource) =>
      resource.subtype === subtype &&
      status[resourceNameToString(resource)]);
};
