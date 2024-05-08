import { readFileSync, writeFileSync } from 'node:fs';

const files = ['base-marker.png', 'destination-marker.png'];

for (const file of files) {
  const buffer = readFileSync(file);
  const base64String = buffer.toString('base64');
  writeFileSync(
    file.replace('png', 'txt'),
    `data:image/png;base64,${base64String}`,
    'utf8'
  );
}
