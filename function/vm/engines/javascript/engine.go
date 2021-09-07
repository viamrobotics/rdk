package javascript

import (
	"io/ioutil"

	"github.com/go-errors/errors"
	"github.com/wasmerio/wasmer-go/wasmer"
	"go.uber.org/multierr"

	functionvm "go.viam.com/core/function/vm"
	"go.viam.com/core/rlog"
)

func init() {
	functionvm.RegisterEngine(functionvm.EngineNameJavaScript, func() (functionvm.Engine, error) {
		return newJavaScriptEngine()
	})
}

type javaScriptEngine struct {
	jsCtxPtr interface{}
	rtPtr    interface{}

	wasiEnv       *wasmer.WasiEnvironment
	memory        *wasmer.Memory
	exportedFuncs map[string]func(...interface{}) (interface{}, error)
}

type wasmerFunc func(args []wasmer.Value) ([]wasmer.Value, error)

// newQuickJSInstance returns a new WASM based QuickJS instance in addition
// to a settable host function.
func newQuickJSInstance() (*wasmer.Instance, *wasmer.WasiEnvironment, *wasmerFunc, error) {
	// TODO(erd): build and embed
	wasmBytes, err := ioutil.ReadFile("/Users/eric/Downloads/quickjs/libquickjs.wasm")
	if err != nil {
		return nil, nil, nil, err
	}

	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	module, err := wasmer.NewModule(store, wasmBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	wasiEnv, err := wasmer.NewWasiStateBuilder("libquickjs").
		CaptureStdout().
		CaptureStderr().
		Finalize()
	if err != nil {
		return nil, nil, nil, err
	}

	importObject, err := wasiEnv.GenerateImportObject(store, module)
	if err != nil {
		return nil, nil, nil, err
	}

	actualHostFunc := wasmerFunc(func(args []wasmer.Value) ([]wasmer.Value, error) {
		return nil, nil
	})
	hostFunction := wasmer.NewFunction(
		store,
		wasmer.NewFunctionType(wasmer.NewValueTypes(
			wasmer.I32, // JSContext *ctx
			wasmer.I64, // JSValueConst this_val
			wasmer.I32, // int argc
			wasmer.I32, // JSValueConst *argv
		), wasmer.NewValueTypes(wasmer.I64)),
		func(args []wasmer.Value) ([]wasmer.Value, error) {
			return actualHostFunc(args)
		},
	)

	importObject.Register(
		"env",
		map[string]wasmer.IntoExtern{
			"wasmHostFunction": hostFunction,
		},
	)

	instance, err := wasmer.NewInstance(module, importObject)
	if err != nil {
		return nil, nil, nil, err
	}

	return instance, wasiEnv, &actualHostFunc, err
}

// TODO(erd): spots to check for exception
func newJavaScriptEngine() (*javaScriptEngine, error) {
	instance, wasiEnv, hostFuncPtr, err := newQuickJSInstance()
	if err != nil {
		return nil, err
	}

	exportedFuncs := map[string]func(...interface{}) (interface{}, error){
		"JS_NewRuntime":        nil,
		"js_std_init_handlers": nil,
		"js_std_add_helpers":   nil,
		"JS_NewContext":        nil,
		"JS_Eval":              nil,
		"js_std_loop":          nil,
		"JS_FreeContext":       nil,
		"JS_FreeRuntime":       nil,
		"JS_IsException":       nil,
		"JS_GetException":      nil,
		"JS_ToCString":         nil,
		"JS_FreeValue":         nil,
		"JS_GetGlobalObject":   nil,
		"JS_NewCFunction":      nil,
		"JS_SetPropertyStr":    nil,
		"JS_Call":              nil,
		"getWASMHostFunction":  nil,
		"malloc":               nil,
		"free":                 nil,
	}
	for name := range exportedFuncs {
		exportedFunc, err := instance.Exports.GetFunction(name)
		if err != nil {
			return nil, errors.Errorf("failed to get function %q: %w", name, err)
		}
		exportedFuncs[name] = exportedFunc
	}

	memory, err := instance.Exports.GetMemory("memory")
	if err != nil {
		return nil, err
	}

	// basic initialization
	rtPtr, err := exportedFuncs["JS_NewRuntime"]()
	if err != nil {
		return nil, err
	}
	_, err = exportedFuncs["js_std_init_handlers"](rtPtr)
	if err != nil {
		return nil, err
	}
	jsCtxPtr, err := exportedFuncs["JS_NewContext"](rtPtr)
	if err != nil {
		return nil, err
	}
	_, err = exportedFuncs["js_std_add_helpers"](jsCtxPtr, 0, 0)
	if err != nil {
		return nil, err
	}

	// our own initialization
	wasmHostFunctionPtr, err := exportedFuncs["getWASMHostFunction"]()
	if err != nil {
		return nil, err
	}

	hostFuncName := "hostFunc"
	// TODO(erd): free
	hostFuncNamePtr, err := exportedFuncs["malloc"](len(hostFuncName) + 1)
	if err != nil {
		return nil, err
	}
	// TODO(erd): refactor to cstring helper
	copy(memory.Data()[hostFuncNamePtr.(int32):], append([]byte(hostFuncName), 0))

	hostFuncCFunc, err := exportedFuncs["JS_NewCFunction"](jsCtxPtr, wasmHostFunctionPtr, hostFuncNamePtr, 1)
	if err != nil {
		return nil, err
	}

	// set up host function proxy
	*hostFuncPtr = func(args []wasmer.Value) ([]wasmer.Value, error) {
		readJSValue := func(memory *wasmer.Memory, at int32) int64 {
			var val uint64
			for i := int32(0); i < 8; i++ {
				val |= uint64(memory.Data()[at+i]) << (i * 8)
			}
			return int64(val)
		}

		stringVal, err := exportedFuncs["JS_ToCString"](args[0].I32(), readJSValue(memory, args[3].I32()))
		if err != nil {
			return nil, err
		}
		stringValPtr := stringVal.(int32)
		stringValPtrIdx := stringValPtr

		var toStringVal string
		for memory.Data()[stringValPtrIdx] != 0 {
			toStringVal += string(memory.Data()[stringValPtrIdx])
			stringValPtrIdx++
		}
		rlog.Logger.Debug("HOST -- ARG 0", toStringVal)

		if args[2].I32() > 1 {
			stringVal, err := exportedFuncs["JS_ToCString"](args[0].I32(), readJSValue(memory, args[3].I32()+8))
			if err != nil {
				return nil, err
			}
			stringValPtr := stringVal.(int32)
			stringValPtrIdx := stringValPtr

			var toStringVal string
			for memory.Data()[stringValPtrIdx] != 0 {
				toStringVal += string(memory.Data()[stringValPtrIdx])
				stringValPtrIdx++
			}
			rlog.Logger.Debug("HOST -- ARG 1", toStringVal)
		}

		return []wasmer.Value{wasmer.NewI64(44)}, nil
	}

	engine := &javaScriptEngine{
		jsCtxPtr:      jsCtxPtr,
		rtPtr:         rtPtr,
		wasiEnv:       wasiEnv,
		memory:        memory,
		exportedFuncs: exportedFuncs,
	}
	if err := engine.initializeHostFunctions(hostFuncCFunc); err != nil {
		return nil, err
	}

	return engine, nil
}

func (eng *javaScriptEngine) initializeHostFunctions(hostFunc interface{}) error {
	globalObj, err := eng.exportedFuncs["JS_GetGlobalObject"](eng.jsCtxPtr)
	if err != nil {
		return err
	}

	libFunc1Code := `(function(hostFunc) {console.log("creating"); return function(arg1) {return hostFunc("libFunc1", arg1);}});`
	libFunc1CodePtr, err := eng.exportedFuncs["malloc"](len(libFunc1Code) + 1)
	if err != nil {
		return err
	}
	// TODO(erd): refactor to cstring helper
	copy(eng.memory.Data()[libFunc1CodePtr.(int32):], append([]byte(libFunc1Code), 0))

	libFunc1FileName := "func"
	// TODO(erd): free
	libFunc1FileNamePtr, err := eng.exportedFuncs["malloc"](len(libFunc1FileName) + 1)
	if err != nil {
		return err
	}
	// TODO(erd): refactor to cstring helper
	copy(eng.memory.Data()[libFunc1FileNamePtr.(int32):], append([]byte(libFunc1FileName), 0))

	libFunc1Ret, err := eng.exportedFuncs["JS_Eval"](
		eng.jsCtxPtr,
		libFunc1CodePtr,
		len(libFunc1Code),
		libFunc1FileNamePtr,
		0, // JS_EVAL_TYPE_GLOBAL
	)
	if err != nil {
		return err
	}

	// TODO(erd): refactor
	if retIsExcep, err := eng.exportedFuncs["JS_IsException"](libFunc1Ret); err == nil && retIsExcep.(int32) == 1 {
		exceptionVal, err := eng.exportedFuncs["JS_GetException"](eng.jsCtxPtr)
		if err != nil {
			return err
		}
		stringVal, err := eng.exportedFuncs["JS_ToCString"](eng.jsCtxPtr, exceptionVal)
		if err != nil {
			return err
		}
		// TODO(erd): always need to free exceptions since they get orphaned after JS_GetException?
		if _, err = eng.exportedFuncs["JS_FreeValue"](eng.jsCtxPtr, exceptionVal); err != nil {
			return err
		}
		stringValPtr := stringVal.(int32)
		stringValPtrIdx := stringValPtr

		var toStringVal string
		for eng.memory.Data()[stringValPtrIdx] != 0 {
			toStringVal += string(eng.memory.Data()[stringValPtrIdx])
			stringValPtrIdx++
		}
		return errors.New(toStringVal)
	} else if err != nil {
		return err
	}
	libFunc1CtorPtr := libFunc1Ret.(int64)

	// can do this on stack?
	// TODO(erd): free
	argVarArr, err := eng.exportedFuncs["malloc"](8)
	if err != nil {
		return err
	}

	writeJSValue := func(memory *wasmer.Memory, at int32, val int64) {
		for i := int32(0); i < 8; i++ {
			memory.Data()[at+i] = byte((val >> (i * 8)) & 0xFF)
		}
	}
	writeJSValue(eng.memory, argVarArr.(int32), hostFunc.(int64))
	libFunc1Ptr, err := eng.exportedFuncs["JS_Call"](eng.jsCtxPtr, libFunc1CtorPtr, globalObj, 1, argVarArr)
	if err != nil {
		return err
	}

	// TODO(erd): refactor
	if retIsExcep, err := eng.exportedFuncs["JS_IsException"](libFunc1Ptr); err == nil && retIsExcep.(int32) == 1 {
		exceptionVal, err := eng.exportedFuncs["JS_GetException"](eng.jsCtxPtr)
		if err != nil {
			return err
		}
		stringVal, err := eng.exportedFuncs["JS_ToCString"](eng.jsCtxPtr, exceptionVal)
		if err != nil {
			return err
		}
		// TODO(erd): always need to free exceptions since they get orphaned after JS_GetException?
		if _, err = eng.exportedFuncs["JS_FreeValue"](eng.jsCtxPtr, exceptionVal); err != nil {
			return err
		}
		stringValPtr := stringVal.(int32)
		stringValPtrIdx := stringValPtr

		var toStringVal string
		for eng.memory.Data()[stringValPtrIdx] != 0 {
			toStringVal += string(eng.memory.Data()[stringValPtrIdx])
			stringValPtrIdx++
		}
	} else if err != nil {
		return err
	}

	libFunc1Name := "libFunc1"
	// TODO(erd): free
	libFunc1NamePtr, err := eng.exportedFuncs["malloc"](len(libFunc1Name) + 1)
	if err != nil {
		return err
	}
	// TODO(erd): refactor to cstring helper
	copy(eng.memory.Data()[libFunc1NamePtr.(int32):], append([]byte(libFunc1Name), 0))

	if _, err := eng.exportedFuncs["JS_SetPropertyStr"](eng.jsCtxPtr, globalObj, libFunc1NamePtr, libFunc1Ptr); err != nil {
		return err
	}

	return nil
}

func (eng *javaScriptEngine) ExecuteCode(code string) ([]functionvm.Value, error) {

	// TODO(erd): free
	funcCodePtr, err := eng.exportedFuncs["malloc"](len(code) + 1)
	if err != nil {
		return nil, err
	}
	// TODO(erd): refactor to cstring helper
	copy(eng.memory.Data()[funcCodePtr.(int32):], append([]byte(code), 0))

	filename := "func"
	// TODO(erd): free
	filenamePtr, err := eng.exportedFuncs["malloc"](len(filename) + 1)
	if err != nil {
		return nil, err
	}
	// TODO(erd): refactor to cstring helper
	copy(eng.memory.Data()[filenamePtr.(int32):], append([]byte(filename), 0))

	ret, err := eng.exportedFuncs["JS_Eval"](
		eng.jsCtxPtr,
		funcCodePtr,
		len(code),
		filenamePtr,
		0, // JS_EVAL_TYPE_GLOBAL
	)
	if err != nil {
		return nil, err
	}

	if retIsExcep, err := eng.exportedFuncs["JS_IsException"](ret); err == nil && retIsExcep.(int32) == 1 {
		exceptionVal, err := eng.exportedFuncs["JS_GetException"](eng.jsCtxPtr)
		if err != nil {
			return nil, err
		}
		stringVal, err := eng.exportedFuncs["JS_ToCString"](eng.jsCtxPtr, exceptionVal)
		if err != nil {
			return nil, err
		}
		// TODO(erd): always need to free exceptions since they get orphaned after JS_GetException?
		if _, err = eng.exportedFuncs["JS_FreeValue"](eng.jsCtxPtr, exceptionVal); err != nil {
			return nil, err
		}
		stringValPtr := stringVal.(int32)
		stringValPtrIdx := stringValPtr

		var toStringVal string
		for eng.memory.Data()[stringValPtrIdx] != 0 {
			toStringVal += string(eng.memory.Data()[stringValPtrIdx])
			stringValPtrIdx++
		}
		return nil, errors.New(toStringVal)

	} else if err != nil {
		return nil, err
	}

	// TODO(erd): put this in other execute blocks
	if _, err := eng.exportedFuncs["js_std_loop"](eng.jsCtxPtr); err != nil {
		return nil, err
	}

	rlog.Logger.Debug("STDOUT:\n", string(eng.wasiEnv.ReadStdout()))
	rlog.Logger.Debug("STDERR:\n", string(eng.wasiEnv.ReadStderr()))

	// val, err := eng.exportValue(ret)
	// if err != nil {
	// 	return nil, err
	// }
	// return []functionvm.Value{val}, nil
	return []functionvm.Value{functionvm.NewString("hello")}, nil
}

func (eng *javaScriptEngine) Close() error {
	var err error
	if _, freeErr := eng.exportedFuncs["JS_FreeContext"](eng.jsCtxPtr); freeErr != nil {
		err = multierr.Combine(err, freeErr)
	}
	if _, freeErr := eng.exportedFuncs["JS_FreeRuntime"](eng.rtPtr); freeErr != nil {
		err = multierr.Combine(err, freeErr)
	}
	return err
}
