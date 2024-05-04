## unversion-soname

The unversion-soname patch in this folder changes the soname of x264 from `.so.VERSION_NUMBER` format to strict `.so`. We do this because the android toolchain removes the version-suffixed version of the SO, but our go build links against the soname it observes.
