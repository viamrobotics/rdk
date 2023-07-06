type MappingDetails = {
  mode: 'localize' | 'create' | 'update',
  name?: string,
  version?: string,
}

export interface SLAMOverrides {
  getSLAMPosition: string
  getPointCloudMap: string
  mappingDetails: MappingDetails
}
export interface RCOverrides {
  slam?: SLAMOverrides
}
