package functionvm_test

import (
	"testing"

	"go.viam.com/test"

	functionvm "go.viam.com/rdk/function/vm"
	_ "go.viam.com/rdk/function/vm/engines/javascript"
)

func TestFunctionConfigValidate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config functionvm.FunctionConfig
		err    string
	}{
		{
			name: "no name", err: `"name" is required`},
		{
			name: "no engine",
			config: functionvm.FunctionConfig{
				Name: "hello",
			},
			err: `"engine" is required`,
		},
		{
			name: "no source",
			config: functionvm.FunctionConfig{
				Name:                    "hello",
				AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{Engine: "foo"},
			},
			err: `"source" is required`,
		},
		{
			name: "no engine",
			config: functionvm.FunctionConfig{
				Name:                    "hello",
				AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{Engine: "foo", Source: "1+"},
			},
			err: `no engine`,
		},
		{
			name: "bad source",
			config: functionvm.FunctionConfig{
				Name:                    "hello",
				AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{Engine: "javascript", Source: "1+"},
			},
			err: `unexpected token`,
		},
		{
			name: "valid",
			config: functionvm.FunctionConfig{
				Name:                    "hello",
				AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{Engine: "javascript", Source: "1"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate("p")
			if tc.err == "" {
				test.That(t, err, test.ShouldBeNil)
				return
			}
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, tc.err)
		})
	}
}
