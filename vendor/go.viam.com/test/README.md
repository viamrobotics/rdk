# test

A golang test library. It's essentially a light wrapper around all assertions from https://github.com/smartystreets/assertions.

<p align="center">
  <a href="https://pkg.go.dev/go.viam.com/test/"><img src="https://pkg.go.dev/badge/go.viam.com/test" alt="PkgGoDev"></a>
</p>

## Usage

```golang
package test

import (
	"testing"

	"go.viam.com/test"
)

func TestExample(t *testing.T) {
	test.That(t, 1, ShouldEqual, 1)
	test.That(t, 1, ShouldNotEqual, "1")
}
```

## License 
Copyright 2021-2022 Viam Inc.

Apache 2.0 - See [LICENSE](https://github.com/viamrobotics/test/blob/main/LICENSE) file
