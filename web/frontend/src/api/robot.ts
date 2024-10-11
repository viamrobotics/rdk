import { type Client } from '@viamrobotics/sdk';

export const getOperations = async (robotClient: Client) => {
  return robotClient.getOperations();
};

export const getResourceNames = async (robotClient: Client) => {
  return robotClient.resourceNames();
};

export const getSessions = async (robotClient: Client) => {
  return robotClient.getSessions();
};
