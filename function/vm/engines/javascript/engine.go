package javascript

import (
	"io/ioutil"

	"github.com/go-errors/errors"
	"github.com/wasmerio/wasmer-go/wasmer"
	"go.uber.org/multierr"

	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

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
	exportedFuncs map[string]exportedFunc
}

type wasmerFunc func(args []wasmer.Value) ([]wasmer.Value, error)

// newQuickJSInstance returns a new WASM based QuickJS instance in addition
// to a settable host function.
func newQuickJSInstance() (*wasmer.Instance, *wasmer.WasiEnvironment, *wasmerFunc, error) {
	// TODO(erd): embed later
	wasmBytes, err := ioutil.ReadFile(artifact.MustPath("function/vm/engines/javascript/libquickjs.wasm"))
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

type exportedFunc struct {
	Func         func(...interface{}) (interface{}, error)
	CanException bool
}

func newJavaScriptEngine() (*javaScriptEngine, error) {
	instance, wasiEnv, hostFuncPtr, err := newQuickJSInstance()
	if err != nil {
		return nil, err
	}

	exportedFuncs := map[string]exportedFunc{
		"JS_NewRuntime":        {nil, false},
		"js_std_init_handlers": {nil, false},
		"js_std_add_helpers":   {nil, false},
		"JS_NewContext":        {nil, false},
		"JS_Eval":              {nil, true},
		"js_std_loop":          {nil, false},
		"JS_FreeContext":       {nil, false},
		"JS_FreeRuntime":       {nil, false},
		"JS_IsException":       {nil, false},
		"JS_GetException":      {nil, false},
		"JS_ToCString":         {nil, true},
		"JS_FreeValue":         {nil, false},
		"JS_GetGlobalObject":   {nil, false},
		"JS_NewCFunction":      {nil, true},
		"JS_SetPropertyStr":    {nil, true},
		"JS_Call":              {nil, true},
		"JS_IsNumber":          {nil, false},
		"JS_IsBigInt":          {nil, false},
		"JS_IsBigFloat":        {nil, false},
		"JS_IsBigDecimal":      {nil, false},
		"JS_IsBool":            {nil, false},
		"JS_IsNull":            {nil, false},
		"JS_IsUndefined":       {nil, false},
		"JS_IsString":          {nil, false},
		"JS_IsSymbol":          {nil, false},
		"JS_IsObject":          {nil, false},
		"JS_IsError":           {nil, false},
		"JS_IsFunction":        {nil, false},
		"JS_IsArray":           {nil, false},
		"getWASMHostFunction":  {nil, false},
		"malloc":               {nil, false},
		"free":                 {nil, false},
	}
	for name, value := range exportedFuncs {
		exportedFunc, err := instance.Exports.GetFunction(name)
		if err != nil {
			return nil, errors.Errorf("failed to get function %q: %w", name, err)
		}
		value.Func = exportedFunc
		exportedFuncs[name] = value
	}

	memory, err := instance.Exports.GetMemory("memory")
	if err != nil {
		return nil, err
	}

	// basic initialization
	rtPtr, err := exportedFuncs["JS_NewRuntime"].Func()
	if err != nil {
		return nil, err
	}
	_, err = exportedFuncs["js_std_init_handlers"].Func(rtPtr)
	if err != nil {
		return nil, err
	}
	jsCtxPtr, err := exportedFuncs["JS_NewContext"].Func(rtPtr)
	if err != nil {
		return nil, err
	}
	_, err = exportedFuncs["js_std_add_helpers"].Func(jsCtxPtr, 0, 0)
	if err != nil {
		return nil, err
	}

	engine := &javaScriptEngine{
		jsCtxPtr:      jsCtxPtr,
		rtPtr:         rtPtr,
		wasiEnv:       wasiEnv,
		memory:        memory,
		exportedFuncs: exportedFuncs,
	}

	// our own initialization
	wasmHostFunctionPtr, err := engine.callExportedFunction("getWASMHostFunction")
	if err != nil {
		return nil, err
	}

	hostFuncName := "hostFunc"
	hostFuncNamePtr, deallocHostFuncName, err := engine.allocateCString(hostFuncName)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(deallocHostFuncName)

	hostFuncCFunc, err := engine.callExportedFunction("JS_NewCFunction", jsCtxPtr, wasmHostFunctionPtr, hostFuncNamePtr, 1)
	if err != nil {
		return nil, err
	}

	// set up host function proxy
	*hostFuncPtr = func(args []wasmer.Value) ([]wasmer.Value, error) {
		// Arguments
		// JSContext *ctx 			I32 - Unused, since it's constant
		// JSValueConst this_val 	I64 - Unused for now
		// int argc 				I32 - Loop over this
		// JSValueConst *argv 		I32 - Export each value

		argC := args[2].I32()
		argVBase := args[3].I32()
		for i := int32(0); i < argC; i++ {
			jsVal := engine.readJSValue(argVBase + (i * 8))
			exportedVal, err := engine.exportValue(jsVal)
			if err != nil {
				return nil, err
			}
			rlog.Logger.Debugf("exp %d %s", i, exportedVal.Stringer())
		}

		// TODO(erd): call proxied function and import values back out
		return []wasmer.Value{wasmer.NewI64(44)}, nil
	}

	if err := engine.initializeHostFunctions(hostFuncCFunc); err != nil {
		return nil, err
	}

	return engine, nil
}

func (eng *javaScriptEngine) callExportedFunction(name string, args ...interface{}) (interface{}, error) {
	expFunc, ok := eng.exportedFuncs[name]
	if !ok {
		return nil, errors.Errorf("no exported function called %q", name)
	}
	ret, err := expFunc.Func(args...)
	if err != nil {
		return nil, err
	}
	if !expFunc.CanException {
		return ret, nil
	}
	if err := eng.checkException(ret); err != nil {
		return nil, err
	}
	return ret, err
}

func (eng *javaScriptEngine) writeJSValue(at int32, val int64) {
	for i := int32(0); i < 8; i++ {
		eng.memory.Data()[at+i] = byte((val >> (i * 8)) & 0xFF)
	}
}

func (eng *javaScriptEngine) readJSValue(at int32) int64 {
	var val uint64
	for i := int32(0); i < 8; i++ {
		val |= uint64(eng.memory.Data()[at+i]) << (i * 8)
	}
	return int64(val)
}

func (eng *javaScriptEngine) valueToString(value interface{}) (string, error) {
	stringVal, err := eng.callExportedFunction("JS_ToCString", eng.jsCtxPtr, value)
	if err != nil {
		return "", err
	}
	if _, err = eng.callExportedFunction("JS_FreeValue", eng.jsCtxPtr, value); err != nil {
		return "", err
	}
	stringValPtr := stringVal.(int32)
	stringValPtrIdx := stringValPtr

	var toStringVal string
	for eng.memory.Data()[stringValPtrIdx] != 0 {
		toStringVal += string(eng.memory.Data()[stringValPtrIdx])
		stringValPtrIdx++
	}
	return toStringVal, nil
}

func (eng *javaScriptEngine) checkException(value interface{}) error {
	isExcep, err := eng.callExportedFunction("JS_IsException", value)
	if err != nil {
		return err
	}
	if isExcep.(int32) != 1 {
		return nil
	}
	exceptionVal, err := eng.callExportedFunction("JS_GetException", eng.jsCtxPtr)
	if err != nil {
		return err
	}

	exceptionStr, err := eng.valueToString(exceptionVal)
	if err != nil {
		return err
	}
	return errors.New(exceptionStr)
}

func (eng *javaScriptEngine) initializeHostFunctions(hostFunc interface{}) error {
	globalObj, err := eng.callExportedFunction("JS_GetGlobalObject", eng.jsCtxPtr)
	if err != nil {
		return err
	}

	libFunc1Code := `(function(hostFunc) {console.log("creating"); return function(arg1) {return hostFunc("libFunc1", arg1);}});`
	libFunc1CodePtr, deallocLibFunc1Code, err := eng.allocateCString(libFunc1Code)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(deallocLibFunc1Code)

	libFunc1FileName := "func"
	libFunc1FileNamePtr, deallocLibFunc1FileName, err := eng.allocateCString(libFunc1FileName)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(deallocLibFunc1FileName)

	libFunc1Ret, err := eng.callExportedFunction("JS_Eval",
		eng.jsCtxPtr,
		libFunc1CodePtr,
		len(libFunc1Code),
		libFunc1FileNamePtr,
		0, // JS_EVAL_TYPE_GLOBAL
	)
	if err != nil {
		return err
	}
	libFunc1CtorPtr := libFunc1Ret.(int64)

	argVarArr, deallocArgVarArr, err := eng.allocateData(8)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(deallocArgVarArr)
	eng.writeJSValue(argVarArr.(int32), hostFunc.(int64))

	libFunc1Ptr, err := eng.callExportedFunction("JS_Call", eng.jsCtxPtr, libFunc1CtorPtr, globalObj, 1, argVarArr)
	if err != nil {
		return err
	}

	libFunc1Name := "libFunc1"
	libFunc1NamePtr, deallocLibFunc1Name, err := eng.allocateCString(libFunc1Name)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(deallocLibFunc1Name)

	if _, err := eng.callExportedFunction("JS_SetPropertyStr", eng.jsCtxPtr, globalObj, libFunc1NamePtr, libFunc1Ptr); err != nil {
		return err
	}

	return nil
}

func (eng *javaScriptEngine) allocateData(size int) (interface{}, func() error, error) {
	dataPtr, err := eng.callExportedFunction("malloc", size)
	if err != nil {
		return nil, nil, err
	}
	return dataPtr, func() error {
		_, err := eng.callExportedFunction("free", dataPtr)
		return err
	}, nil
}

func (eng *javaScriptEngine) allocateCString(value string) (interface{}, func() error, error) {
	valuePtr, deallocateValue, err := eng.allocateData(len(value) + 1)
	if err != nil {
		return nil, nil, err
	}
	copy(eng.memory.Data()[valuePtr.(int32):], append([]byte(value), 0))
	return valuePtr, deallocateValue, nil
}

// ExecuteCode evaluates the given code, followed by running the event loop. It returns
// the value returned from the function (promises not yet handled), unless an error occurs.
func (eng *javaScriptEngine) ExecuteCode(code string) ([]functionvm.Value, error) {

	codePtr, deallocCode, err := eng.allocateCString(code)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(deallocCode)

	filename := "func"
	filenamePtr, deallocFilename, err := eng.allocateCString(filename)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(deallocFilename)

	ret, err := eng.callExportedFunction("JS_Eval",
		eng.jsCtxPtr,
		codePtr,
		len(code),
		filenamePtr,
		0, // JS_EVAL_TYPE_GLOBAL
	)
	if err != nil {
		return nil, err
	}

	if _, err := eng.callExportedFunction("js_std_loop", eng.jsCtxPtr); err != nil {
		return nil, err
	}

	val, err := eng.exportValue(ret)
	if err != nil {
		return nil, err
	}
	return []functionvm.Value{val}, nil
}

func (eng *javaScriptEngine) exportValue(value interface{}) (functionvm.Value, error) {
	vt := functionvm.ValueTypeUnknown
	for _, cc := range []struct {
		FuncName  string
		ValueType functionvm.ValueType
		NeedCtx   bool
	}{
		// TODO(erd): need to implement all of these and possibly
		// remove/reorder some. May need to find a faster way to do this
		// and return a API consistent type descriptor from QuickJS.
		{"JS_IsNumber", functionvm.ValueTypeUnknown, false},
		{"JS_IsBigInt", functionvm.ValueTypeUnknown, true},
		{"JS_IsBigFloat", functionvm.ValueTypeUnknown, false},
		{"JS_IsBigDecimal", functionvm.ValueTypeUnknown, false},
		{"JS_IsBool", functionvm.ValueTypeUnknown, false},
		{"JS_IsNull", functionvm.ValueTypeUnknown, false},
		{"JS_IsUndefined", functionvm.ValueTypeUnknown, false},
		{"JS_IsException", functionvm.ValueTypeUnknown, false},
		{"JS_IsString", functionvm.ValueTypeString, false},
		{"JS_IsSymbol", functionvm.ValueTypeUnknown, false},
		{"JS_IsError", functionvm.ValueTypeUnknown, false},
		{"JS_IsFunction", functionvm.ValueTypeUnknown, true},
		{"JS_IsArray", functionvm.ValueTypeUnknown, true},
		{"JS_IsObject", functionvm.ValueTypeUnknown, true},
	} {
		var args []interface{}
		if cc.NeedCtx {
			args = append(args, eng.jsCtxPtr)
		}
		args = append(args, value)
		ret, err := eng.callExportedFunction(cc.FuncName, args...)
		if err != nil {
			return nil, err
		}
		if ret.(int32) == 1 {
			vt = cc.ValueType
			break
		}
	}
	switch vt {
	case functionvm.ValueTypeString:
		str, err := eng.valueToString(value)
		if err != nil {
			return nil, err
		}
		return functionvm.NewString(str), nil
	default:
		return nil, errors.Errorf("do not know how to export a %q", vt)
	}

}

func (eng *javaScriptEngine) Close() error {
	var err error
	if _, freeErr := eng.callExportedFunction("JS_FreeContext", eng.jsCtxPtr); freeErr != nil {
		err = multierr.Combine(err, freeErr)
	}
	if _, freeErr := eng.callExportedFunction("JS_FreeRuntime", eng.rtPtr); freeErr != nil {
		err = multierr.Combine(err, freeErr)
	}
	return err
}
