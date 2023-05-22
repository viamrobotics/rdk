
Unfortunately right now we don't have a good strategy for serving static assets like images within the multiple environments that remote control exists in (local, standalone, embedded in app) and since these are small PNGs, it's a bit easier and safer to just encode them as strings so that they're embedded directly in production JS code.

Transforming images into base64 strings is quite simple. For, example, the process is as follows for Node.js:

```
import { readFileSync } from 'fs';

// Read the PNG file as a Buffer
const buffer = readFileSync('./path/to/your/file.png');

// Convert the Buffer to a base64 string
const base64String = buffer.toString('base64');
```
