import type { commonApi } from '@viamrobotics/sdk';

const sortByName = (item1: commonApi.ResourceName.AsObject, item2: commonApi.ResourceName.AsObject) => {
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

export const resourceNameToSubtypeString = (resource: commonApi.ResourceName.AsObject) => {
  if (!resource) {
    return '';
  }

  return `${resource.namespace}:${resource.type}:${resource.subtype}`;
};

export const resourceNameToString = (resource: commonApi.ResourceName.AsObject) => {
  if (!resource) {
    return '';
  }

  let strName = resourceNameToSubtypeString(resource);
  if (resource.name !== '') {
    strName += `/${resource.name}`;
  }
  return strName;
};

export const filterResources = (
  resources: commonApi.ResourceName.AsObject[],
  namespace: string,
  type: string,
  subtype: string
) => {
  const results = [];

  for (const resource of resources) {
    if (
      resource.namespace === namespace &&
      resource.type === type &&
      resource.subtype === subtype
    ) {
      results.push(resource);
    }
  }

  return results.sort(sortByName);
};

export const filterNonRemoteResources = (
  resources: commonApi.ResourceName.AsObject[],
  namespace: string,
  type: string,
  subtype: string
) => {
  return filterResources(resources, namespace, type, subtype).filter((resource) => !resource.name.includes(':'));
};

export const filterRdkComponentsWithStatus = (
  resources: commonApi.ResourceName.AsObject[],
  status: Record<string, unknown>,
  subtype: string
) => {
  return resources
    .filter((resource) =>
      resource.namespace === 'rdk' &&
      resource.type === 'component' &&
      resource.subtype === subtype &&
      status[resourceNameToString(resource)]).sort(sortByName);
};

export const filterComponentsWithNames = (resources: commonApi.ResourceName.AsObject[]) => {
  return resources
    .filter((resource) => Boolean(resource.name) && resource.type === 'component')
    .sort(sortByName);
};
