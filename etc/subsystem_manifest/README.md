This go script is used to create metadata that is uploaded to gcs along with each upload of viam-server. This script is invoked primarily from `package.make` and all of the magic-values should be stored in there rather than in this script (if possible).

It accepts the following cli arguments:

--subsystem -> defaults to viam-server

--binary-path -> used to calculate the sha256 and get resource registrations

--upload-path -> path in gcs where the binary will end up

--version -> ex: v0.13.0

--arch -> result of uname -m (translated from x86_64 -> linux/amd64)

--output-path -> path to write the output manifest to. Ex: packaging/static/manifest/viam-server-v0.14.0-x86_64.json

Sample manifest:

```json5
{
    "subsystem": "viam-server",
    "version": "0.14.0",
    "platform": "linux/amd64",
    "upload-path": "packages.viam.com/app/viam-server/viam-server-v0.14.0-x86_64"
    "sha256": "1d4a2e31d79b6231b32eec2046f3da59a1c151139af413a34f0c950421c10552"
    "metadata": {
        "resource_registrations": [
            {
                "api": "rdk:component:camera",
                "model": "rdk:builtin:camera",
                "attribute_schema": {
                     // ...
                 }
            },
            {
                "api": "rdk:service:ml_model",
                "model": "rdk:builtin:fake"
                "attribute_schema": {
                     "..."
                 },
            },
            // ...
        ]
    }
}
```
