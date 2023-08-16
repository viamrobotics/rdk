import type { LngLat } from '@/api/navigation';

export const calculateBoundingBox = (
  radius: number,
  center: LngLat
): [[west: number, south: number], [east: number, north: number]] => {
  // Earth's approximate radius in meters
  const earthRadius = 6_371_000;

  // Convert the center's latitude and longitude from degrees to radians
  const centerLatRad = (center.lat * Math.PI) / 180;
  const centerLngRad = (center.lng * Math.PI) / 180;

  // Calculate the differences in latitude and longitude for the bounding box
  const latDiff = (radius * 180) / (Math.PI * earthRadius);
  const lngDiff = ((radius * 180) / (Math.PI * earthRadius * Math.cos(centerLatRad))) * (1 / Math.cos(centerLngRad));

  // Calculate the bounding box coordinates
  const west = center.lng - latDiff;
  const south = center.lat - lngDiff;
  const east = center.lng + latDiff;
  const north = center.lat + lngDiff;

  return [
    [west, south],
    [east, north],
  ];
};
