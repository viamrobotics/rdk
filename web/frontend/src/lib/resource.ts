export interface Resource {
  resources: Resource[] | undefined;
  name: string
  type: string
  subtype: string
  namespace: string
}

export const normalizeRemoteName = (name: string) => {
  return name.replace(':', '-');
};

const sortByName = (item1: Resource, item2: Resource) => {
  return item1.name > item2.name ? 1 : -1;
};

export const resourceNameToSubtypeString = (resource: Resource) => {
  if (!resource) {
    return '';
  }

  return `${resource.namespace}:${resource.type}:${resource.subtype}`;
};

export const resourceNameToString = (resource: Resource) => {
  if (!resource) {
    return '';
  }

  let strName = resourceNameToSubtypeString(resource);
  if (resource.name !== '') {
    strName += `/${resource.name}`;
  }
  return strName;
};

export const filterResources = (resources: Resource[], namespace: string, type: string, subtype: string) => {
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

export const filterRdkComponentsWithStatus = (
  resources: Resource[],
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

export const filterResourcesWithNames = (resources: Resource[]) => {
  return resources
    .filter((resource) => Boolean(resource.name))
    .sort(sortByName);
};
