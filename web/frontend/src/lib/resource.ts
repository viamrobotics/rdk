interface Resource {
  name: string
  type: string
  subtype: string
  namespace: string
}

export const normalizeRemoteName = (name: string) => {
  return name.replace(":", "-");
}

const sortByName = (a: Resource, b: Resource) => {
  if (a.name < b.name) {
    return -1;
  }
  if (a.name > b.name) {
    return 1;
  }
  return 0;
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
  return resources
    .filter((resource) =>
      resource.namespace === namespace &&
      resource.type === type &&
      resource.subtype === subtype
    )
    .sort(sortByName);
};

export const filterRdkComponentsWithStatus = (resources: Resource[], status: any, subtype: string) => {
  return resources
    .filter((resource) =>
      resource.namespace === 'rdk' &&
      resource.type === 'component' &&
      resource.subtype === subtype &&
      status[resourceNameToString(resource)]
    ).sort(sortByName);
};
