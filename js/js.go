package js

import (
	"context"
	"log"
	"reflect"
	"strings"
	"sync"
	"unicode"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/buffer"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/process"
	"github.com/dop251/goja_nodejs/require"
)

var vmsPool *RuntimePool

type RuntimePool struct {
	pool sync.Pool
}

func NewRuntimePool() *RuntimePool {

	requireRegistry := new(require.Registry)

	sharedBinds := func(vm *goja.Runtime) {
		requireRegistry.Enable(vm)
		console.Enable(vm)
		process.Enable(vm)
		buffer.Enable(vm)
		vm.SetFieldNameMapper(FieldMapper{})
	}

	return &RuntimePool{
		pool: sync.Pool{
			New: func() any {
				vm := goja.New()
				sharedBinds(vm)
				return vm
			},
		},
	}
}

func (rp *RuntimePool) Get() *goja.Runtime {
	return rp.pool.Get().(*goja.Runtime)
}

func (rp *RuntimePool) Put(vm *goja.Runtime) {
	rp.pool.Put(vm)
}

func InitJSVM() *goja.Runtime {
	if vmsPool == nil {
		vmsPool = NewRuntimePool()
	}
	return vmsPool.Get()
}

func FreeJSVM(vm *goja.Runtime) {
	if vmsPool != nil {
		vmsPool.Put(vm)
	}
}

func Eval(ctx context.Context, code string, params map[string]any) (any, error) {
	vm := InitJSVM()
	defer FreeJSVM(vm)
	for k, v := range params {
		vm.Set(k, v)
	}
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in Eval:", r)
		}
		for k := range params {
			vm.Set(k, nil)
		}
	}()
	result, err := vm.RunString(code)
	if err != nil {
		return nil, err
	}
	return result.Export(), nil
}

var (
	_ goja.FieldNameMapper = (*FieldMapper)(nil)
)

type FieldMapper struct{}

func (u FieldMapper) FieldName(_ reflect.Type, f reflect.StructField) string {
	return convertGoToJSName(f.Name)
}

func (u FieldMapper) MethodName(_ reflect.Type, m reflect.Method) string {
	return convertGoToJSName(m.Name)
}

var nameExceptions = map[string]string{
	"OAuth2": "oauth2",
}

func convertGoToJSName(name string) string {

	if v, ok := nameExceptions[name]; ok {
		return v
	}

	startUppercase := make([]rune, 0, len(name))

	for _, c := range name {
		if c != '_' && !unicode.IsUpper(c) && !unicode.IsDigit(c) {
			break
		}

		startUppercase = append(startUppercase, c)
	}

	totalStartUppercase := len(startUppercase)

	// all uppercase eg. "JSON" -> "json"
	if len(name) == totalStartUppercase {
		return strings.ToLower(name)
	}

	// eg. "JSONField" -> "jsonField"
	if totalStartUppercase > 1 {
		return strings.ToLower(name[0:totalStartUppercase-1]) + name[totalStartUppercase-1:]
	}

	// eg. "GetField" -> "getField"
	if totalStartUppercase == 1 {
		return strings.ToLower(name[0:1]) + name[1:]
	}

	return name
}
