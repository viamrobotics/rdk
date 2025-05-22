package nlopt

import (
	"sync"
)

var (
	funcMutex sync.Mutex

	funcMap = make(map[*uint8]interface{})
)

func makeFuncPtr(f Func) *uint8 {
	return _setFuncMapEntry(f)
}

func makeMfuncPtr(f Mfunc) *uint8 {
	return _setFuncMapEntry(f)
}

func _setFuncMapEntry(f interface{}) *uint8 {
	funcMutex.Lock()
	defer funcMutex.Unlock()
	var funcPtr uint8
	funcMap[&funcPtr] = f

	return &funcPtr
}

func getFunc(ptr *uint8) Func {
	return _getFuncMapEntry(ptr).(Func)
}

func getMfunc(ptr *uint8) Mfunc {
	return _getFuncMapEntry(ptr).(Mfunc)
}

func _getFuncMapEntry(ptr *uint8) interface{} {
	funcMutex.Lock()
	defer funcMutex.Unlock()
	return funcMap[ptr]
}

func freeFuncPtr(ptr *uint8) {
	funcMutex.Lock()
	defer funcMutex.Unlock()

	delete(funcMap, ptr)
}
