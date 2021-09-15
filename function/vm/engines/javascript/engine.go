package javascript

import (
	_ "embed" // for libquickjs.wasm
	"fmt"
	"strings"

	"github.com/go-errors/errors"
	"github.com/wasmerio/wasmer-go/wasmer"
	"go.uber.org/multierr"

	"go.viam.com/utils"

	functionvm "go.viam.com/core/function/vm"
)

//go:embed libquickjs.wasm
var libquickjsWASMBytes []byte

func init() {
	functionvm.RegisterEngine(functionvm.EngineNameJavaScript, func() (functionvm.Engine, error) {
		return newJavaScriptEngine()
	})
}

type javaScriptEngine struct {
	jsCtxPtr      interface{}
	rtPtr         interface{}
	hostFuncCFunc interface{}

	wasiEnv       *wasmer.WasiEnvironment
	memory        *wasmer.Memory
	exportedFuncs map[string]exportedFunc
	importedFuncs map[string]functionvm.Function

	VMLogs []string
}

type wasmerFunc func(args []wasmer.Value) ([]wasmer.Value, error)

var (
	newInstance func() (*wasmer.Instance, *wasmer.WasiEnvironment, *wasmerFunc, error)
)

func init() {
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	module, err := wasmer.NewModule(store, libquickjsWASMBytes)
	if err != nil {
		panic(err)
	}

	wasiEnv, err := wasmer.NewWasiStateBuilder("libquickjs").
		CaptureStdout().
		CaptureStderr().
		Finalize()
	if err != nil {
		panic(err)
	}

	importObject, err := wasiEnv.GenerateImportObject(store, module)
	if err != nil {
		panic(err)
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

	newInstance = func() (*wasmer.Instance, *wasmer.WasiEnvironment, *wasmerFunc, error) {
		instance, err := wasmer.NewInstance(module, importObject)
		if err != nil {
			return nil, nil, nil, err
		}
		return instance, wasiEnv, &actualHostFunc, err
	}

}

// newQuickJSInstance returns a new WASM based QuickJS instance in addition
// to a settable host function.
func newQuickJSInstance() (*wasmer.Instance, *wasmer.WasiEnvironment, *wasmerFunc, error) {
	return newInstance()
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
		"JS_NewRuntime":          {nil, false},
		"js_std_init_handlers":   {nil, false},
		"js_std_add_helpers":     {nil, false},
		"js_init_module_std":     {nil, false},
		"js_init_module_os":      {nil, false},
		"JS_NewContext":          {nil, false},
		"JS_Eval":                {nil, true},
		"js_std_loop":            {nil, false},
		"JS_FreeContext":         {nil, false},
		"JS_FreeRuntime":         {nil, false},
		"JS_IsException":         {nil, false},
		"JS_GetException":        {nil, false},
		"JS_ToCString":           {nil, true},
		"JS_FreeValue":           {nil, false},
		"JS_GetGlobalObject":     {nil, false},
		"JS_NewCFunction":        {nil, true},
		"JS_GetPropertyStr":      {nil, true},
		"JS_SetPropertyStr":      {nil, true},
		"JS_Call":                {nil, true},
		"JS_IsNumber":            {nil, false},
		"JS_IsInt":               {nil, false},
		"JS_IsBigInt":            {nil, false},
		"JS_IsBigFloat":          {nil, false},
		"JS_IsBigDecimal":        {nil, false},
		"JS_IsBool":              {nil, false},
		"JS_IsNull":              {nil, false},
		"JS_IsUndefined":         {nil, false},
		"JS_IsString":            {nil, false},
		"JS_IsSymbol":            {nil, false},
		"JS_IsObject":            {nil, false},
		"JS_IsError":             {nil, false},
		"JS_IsFunction":          {nil, false},
		"JS_IsArray":             {nil, false},
		"JS_NewString":           {nil, false},
		"JS_NewObject":           {nil, false},
		"JS_NewBool":             {nil, false},
		"JS_GetBool":             {nil, false},
		"JS_NewFloat64":          {nil, false},
		"JS_NewInt64":            {nil, false},
		"JS_GetFloat64":          {nil, false},
		"JS_GetInt64":            {nil, false},
		"getWASMHostFunction":    {nil, false},
		"getJSModuleLoader":      {nil, false},
		"JS_SetModuleLoaderFunc": {nil, false},
		"JS_Throw":               {nil, false},
		"malloc":                 {nil, false},
		"free":                   {nil, false},
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
	jsModuleLoaderPtr, err := exportedFuncs["getJSModuleLoader"].Func()
	if err != nil {
		return nil, err
	}
	_, err = exportedFuncs["JS_SetModuleLoaderFunc"].Func(rtPtr, 0, jsModuleLoaderPtr, 0)
	if err != nil {
		return nil, err
	}

	engine := &javaScriptEngine{
		jsCtxPtr:      jsCtxPtr,
		rtPtr:         rtPtr,
		wasiEnv:       wasiEnv,
		memory:        memory,
		exportedFuncs: exportedFuncs,
		importedFuncs: map[string]functionvm.Function{},
	}

	stdPtr, deallocStd, err := engine.allocateCString("std")
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(deallocStd)
	osPtr, deallocOS, err := engine.allocateCString("os")
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(deallocOS)

	_, err = exportedFuncs["js_init_module_std"].Func(jsCtxPtr, stdPtr)
	if err != nil {
		return nil, err
	}
	_, err = exportedFuncs["js_init_module_os"].Func(jsCtxPtr, osPtr)
	if err != nil {
		return nil, err
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
	engine.hostFuncCFunc = hostFuncCFunc

	// set up host function proxy
	*hostFuncPtr = func(args []wasmer.Value) (_ []wasmer.Value, err error) {
		defer func() {
			if err != nil {
				engine.VMLogs = append(engine.VMLogs, fmt.Sprintf("swallowing error due to early exit crashes: %s\n", err))
				err = nil
			}
		}()
		// Arguments
		// JSContext *ctx 			I32 - Unused, since it's constant
		// JSValueConst this_val 	I64 - Unused for now
		// int argc 				I32 - Loop over this
		// JSValueConst *argv 		I32 - Export each value
		argC := args[2].I32()
		argVBase := args[3].I32()
		exportedArgs := make([]functionvm.Value, 0, argC)
		for i := int32(0); i < argC; i++ {
			jsVal := engine.readJSValue(argVBase + (i * 8))
			exportedVal, err := engine.exportValue(jsVal)
			if err != nil {
				return nil, err
			}
			exportedArgs = append(exportedArgs, exportedVal)
		}

		if len(exportedArgs) == 0 {
			return nil, errors.New("expected to have at least 1 argument for the imported function name")
		}
		importedFuncName, err := exportedArgs[0].String()
		if err != nil {
			return nil, err
		}
		importedFunc, ok := engine.importedFuncs[importedFuncName]
		if !ok {
			return nil, fmt.Errorf("no imported function called %q", importedFuncName)
		}

		results, err := importedFunc(exportedArgs[1:]...)
		if err != nil {
			errStr, err := engine.newJSString(err.Error())
			if err != nil {
				return nil, err
			}
			exceptionVal, err := engine.callExportedFunction("JS_Throw", engine.jsCtxPtr, errStr.I64())
			if err != nil {
				return nil, err
			}
			return []wasmer.Value{wasmer.NewI64(exceptionVal.(int64))}, nil
		}

		importedResults := make([]wasmer.Value, 0, len(results))
		for _, result := range results {
			importedValue, err := engine.importValue(result)
			if err != nil {
				return nil, err
			}
			importedResults = append(importedResults, importedValue)
		}

		return importedResults, nil
	}

	return engine, nil
}

func (eng *javaScriptEngine) StandardOutput() string {
	return string(eng.wasiEnv.ReadStdout())
}

func (eng *javaScriptEngine) StandardError() string {
	stdErr := string(eng.wasiEnv.ReadStderr())
	if len(eng.VMLogs) == 0 {
		return stdErr
	}
	return stdErr + "\n" + strings.Join(eng.VMLogs, "\n")
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

func (eng *javaScriptEngine) ImportFunction(name string, f functionvm.Function) error {
	nameParts := strings.Split(name, ".")
	if len(nameParts) > 2 {
		return errors.New("can only nest a function name once right now")
	}
	eng.importedFuncs[name] = f

	globalObj, err := eng.callExportedFunction("JS_GetGlobalObject", eng.jsCtxPtr)
	if err != nil {
		return err
	}

	proxyCode := fmt.Sprintf(`(function(hostFunc) {return function() {return hostFunc.apply(null, ["%s"].concat([...arguments]));}});`, name)
	proxyCodePtr, deallocproxyCode, err := eng.allocateCString(proxyCode)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(deallocproxyCode)

	funcNamePtr, deallocFuncName, err := eng.allocateCString(name)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(deallocFuncName)

	evalRet, err := eng.callExportedFunction("JS_Eval",
		eng.jsCtxPtr,
		proxyCodePtr,
		len(proxyCode),
		funcNamePtr,
		0, // JS_EVAL_TYPE_GLOBAL
	)
	if err != nil {
		return err
	}
	funcCtorPtr := evalRet.(int64)

	argVarArr, deallocArgVarArr, err := eng.allocateData(8)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(deallocArgVarArr)
	eng.writeJSValue(argVarArr.(int32), eng.hostFuncCFunc.(int64))

	funcPtr, err := eng.callExportedFunction("JS_Call", eng.jsCtxPtr, funcCtorPtr, globalObj, 1, argVarArr)
	if err != nil {
		return err
	}

	objToSet := globalObj
	if len(nameParts) > 1 {
		// get or set nested object
		objNamePtr, deallocObjName, err := eng.allocateCString(nameParts[0])
		if err != nil {
			return err
		}
		defer utils.UncheckedErrorFunc(deallocObjName)
		objToSet, err = eng.callExportedFunction("JS_GetPropertyStr", eng.jsCtxPtr, globalObj, objNamePtr)
		if err != nil {
			return err
		}
		isUndefined, err := eng.callExportedFunction("JS_IsUndefined", objToSet)
		if err != nil {
			return err
		}
		if isUndefined.(int32) == 1 {
			objToSet, err = eng.callExportedFunction("JS_NewObject", eng.jsCtxPtr)
			if err != nil {
				return err
			}

			if _, err := eng.callExportedFunction("JS_SetPropertyStr", eng.jsCtxPtr, globalObj, objNamePtr, objToSet); err != nil {
				return err
			}
		}
	}

	nameToSet := nameParts[len(nameParts)-1]
	funcNamePtr, deallocFuncName, err = eng.allocateCString(nameToSet)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(deallocFuncName)

	if _, err := eng.callExportedFunction("JS_SetPropertyStr", eng.jsCtxPtr, objToSet, funcNamePtr, funcPtr); err != nil {
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

// ValidateSource ensures the given source can compile.
func (eng *javaScriptEngine) ValidateSource(source string) error {
	sourcePtr, deallocCode, err := eng.allocateCString(source)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(deallocCode)

	filename := "func"
	filenamePtr, deallocFilename, err := eng.allocateCString(filename)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(deallocFilename)

	_, err = eng.callExportedFunction("JS_Eval",
		eng.jsCtxPtr,
		sourcePtr,
		len(source),
		filenamePtr,
		1<<5, // JS_EVAL_FLAG_COMPILE_ONLY (1 << 5)
	)
	return err
}

// ExecuteSource evaluates the given source, followed by running the event loop. It returns
// the value returned from the function (promises not yet handled), unless an error occurs.
func (eng *javaScriptEngine) ExecuteSource(source string) ([]functionvm.Value, error) {

	sourcePtr, deallocCode, err := eng.allocateCString(source)
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
		sourcePtr,
		len(source),
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

func (eng *javaScriptEngine) newJSString(value string) (wasmer.Value, error) {
	valuePtr, deallocValue, err := eng.allocateCString(value)
	if err != nil {
		return wasmer.Value{}, err
	}
	defer utils.UncheckedErrorFunc(deallocValue)
	str, err := eng.callExportedFunction("JS_NewString", eng.jsCtxPtr, valuePtr)
	if err != nil {
		return wasmer.Value{}, err
	}
	return wasmer.NewI64(str.(int64)), nil
}

func (eng *javaScriptEngine) importValue(value functionvm.Value) (wasmer.Value, error) {
	vt := value.Type()
	switch vt {
	case functionvm.ValueTypeString:
		stringVal, err := value.String()
		if err != nil {
			return wasmer.Value{}, err
		}
		return eng.newJSString(stringVal)
	case functionvm.ValueTypeBool:
		boolVal, err := value.Bool()
		if err != nil {
			return wasmer.Value{}, err
		}
		var boolIntVal int32 = 0
		if boolVal {
			boolIntVal = 1
		}
		boolJSVal, err := eng.callExportedFunction("JS_NewBool", eng.jsCtxPtr, boolIntVal)
		if err != nil {
			return wasmer.Value{}, err
		}
		return wasmer.NewI64(boolJSVal.(int64)), nil
	case functionvm.ValueTypeFloat:
		floatVal, err := value.Float()
		if err != nil {
			return wasmer.Value{}, err
		}
		floatJSVal, err := eng.callExportedFunction("JS_NewFloat64", eng.jsCtxPtr, floatVal)
		if err != nil {
			return wasmer.Value{}, err
		}
		return wasmer.NewI64(floatJSVal.(int64)), nil
	case functionvm.ValueTypeInt:
		intVal, err := value.Int()
		if err != nil {
			return wasmer.Value{}, err
		}
		intJSVal, err := eng.callExportedFunction("JS_NewInt64", eng.jsCtxPtr, intVal)
		if err != nil {
			return wasmer.Value{}, err
		}
		return wasmer.NewI64(intJSVal.(int64)), nil
	default:
		return wasmer.Value{}, errors.Errorf("do not know how to import a %q", vt)
	}
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
		{"JS_IsInt", functionvm.ValueTypeInt, false}, // needs to come before number
		{"JS_IsNumber", functionvm.ValueTypeFloat, false},
		{"JS_IsBigInt", functionvm.ValueTypeUnknown, true},
		{"JS_IsBigFloat", functionvm.ValueTypeUnknown, false},
		{"JS_IsBigDecimal", functionvm.ValueTypeUnknown, false},
		{"JS_IsBool", functionvm.ValueTypeBool, false},
		{"JS_IsNull", functionvm.ValueTypeUnknown, false},
		{"JS_IsUndefined", functionvm.ValueTypeUndefined, false},
		{"JS_IsException", functionvm.ValueTypeUnknown, false},
		{"JS_IsString", functionvm.ValueTypeString, false},
		{"JS_IsSymbol", functionvm.ValueTypeUnknown, false},
		{"JS_IsError", functionvm.ValueTypeUnknown, true},
		{"JS_IsFunction", functionvm.ValueTypeUnknown, true},
		{"JS_IsArray", functionvm.ValueTypeUnknown, true},
		{"JS_IsObject", functionvm.ValueTypeUnknown, false},
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
	case functionvm.ValueTypeBool:
		boolVal, err := eng.callExportedFunction("JS_GetBool", value)
		if err != nil {
			return nil, err
		}
		return functionvm.NewBool(boolVal.(int32) == 1), nil
	case functionvm.ValueTypeInt:
		intVal, err := eng.callExportedFunction("JS_GetInt64", value)
		if err != nil {
			return nil, err
		}
		return functionvm.NewInt(intVal.(int64)), nil
	case functionvm.ValueTypeFloat:
		// TODO(erd): get more precise type
		floatVal, err := eng.callExportedFunction("JS_GetFloat64", value)
		if err != nil {
			return nil, err
		}
		return functionvm.NewFloat(floatVal.(float64)), nil
	case functionvm.ValueTypeUndefined:
		return functionvm.NewUndefined(), nil
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
