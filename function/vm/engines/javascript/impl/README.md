# JavaScript WASM Engine

This engine is based off of changes to [QuickJS](https://bellard.org/quickjs/) to work inside WASM. It uses wasmer-go to run it. The changes made are at https://github.com/edaniels/quickjs/tree/wasm.

## Building

In the quickjs directory:

```
`CONFIG_WASM=y PATH=/opt/wasi-sdk/bin:$PATH make libquickjs.wasm`
```

1. Then copy `libquickjs.wasm` to your artifact data directory within `rdk`, typically `~/rdk/.artifact/data/function/vm/engines/javascript/`.
2. Then push the build with `artifact push`.
