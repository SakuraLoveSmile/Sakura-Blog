// Package wasmffi 为隔离 WASI 模块提供 GObject 使用的标量 libffi 调用。
package wasmffi

import (
	"context"
	"fmt"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const functionPrefix = "__stellar_ffi_"

// Install 注册显式宿主导入；只可调用模块自身的固定导出，不访问宿主指针。
func Install(ctx context.Context, r wazero.Runtime) error {
	_, err := r.NewHostModuleBuilder("stellar_ffi").NewFunctionBuilder().WithGoModuleFunction(api.GoModuleFunc(call), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32}, nil).Export("call").Instantiate(ctx)
	return err
}
func require(ok bool, message string) {
	if !ok {
		panic("stellar ffi: " + message)
	}
}
func read32(m api.Memory, p uint32) uint32 {
	v, ok := m.ReadUint32Le(p)
	require(ok, "pointer outside module memory")
	return v
}
func read16(m api.Memory, p uint32) uint16 {
	v, ok := m.ReadUint16Le(p)
	require(ok, "pointer outside module memory")
	return v
}
func read8(m api.Memory, p uint32) uint8 {
	v, ok := m.ReadByte(p)
	require(ok, "pointer outside module memory")
	return v
}
func read64(m api.Memory, p uint32) uint64 {
	v, ok := m.ReadUint64Le(p)
	require(ok, "pointer outside module memory")
	return v
}
func call(ctx context.Context, module api.Module, stack []uint64) {
	m := module.Memory()
	require(m != nil, "module has no memory")
	cif, function, result, arguments := uint32(stack[0]), uint32(stack[1]), uint32(stack[2]), uint32(stack[3])
	require(cif <= ^uint32(0)-28, "CIF pointer overflows")
	argc := read32(m, cif+4)
	require(argc <= 1024, "argument count exceeds limit")
	atypes := read32(m, cif+8)
	rtype := read32(m, cif+12)
	fn := module.ExportedFunction(fmt.Sprintf("%s%d", functionPrefix, function))
	require(fn != nil, "function pointer is not a fixed module function")
	definition := fn.Definition()
	require(len(definition.ParamTypes()) == int(argc), "function signature has a different argument count")
	require(rtype <= ^uint32(0)-8, "return type pointer overflows")
	returnKind := read16(m, rtype+6)
	if returnKind == 0 {
		require(len(definition.ResultTypes()) == 0, "void return signature mismatch")
	} else {
		require(len(definition.ResultTypes()) == 1, "return signature has multiple values")
		expected := api.ValueTypeI32
		switch returnKind {
		case 2:
			expected = api.ValueTypeF32
		case 3:
			expected = api.ValueTypeF64
		case 11, 12:
			expected = api.ValueTypeI64
		case 1, 5, 6, 7, 8, 9, 10, 14:
		default:
			panic("stellar ffi: unsupported return scalar")
		}
		require(definition.ResultTypes()[0] == expected, "return signature mismatch")
	}
	values := make([]uint64, argc)
	for i := uint32(0); i < argc; i++ {
		require(atypes <= ^uint32(0)-i*4 && arguments <= ^uint32(0)-i*4, "argument array overflows")
		t := read32(m, atypes+i*4)
		p := read32(m, arguments+i*4)
		require(t <= ^uint32(0)-8, "type pointer overflows")
		kind := read16(m, t+6)
		var value uint64
		switch kind {
		case 1, 9, 10, 14:
			value = uint64(read32(m, p))
		case 2:
			value = uint64(read32(m, p))
			require(definition.ParamTypes()[i] == api.ValueTypeF32, "float signature mismatch")
		case 3:
			value = read64(m, p)
			require(definition.ParamTypes()[i] == api.ValueTypeF64, "double signature mismatch")
		case 5:
			value = uint64(read8(m, p))
		case 6:
			value = uint64(uint32(int32(int8(read8(m, p)))))
		case 7:
			value = uint64(read16(m, p))
		case 8:
			value = uint64(uint32(int32(int16(read16(m, p)))))
		case 11, 12:
			value = read64(m, p)
		default:
			panic("stellar ffi: unsupported scalar type")
		}
		if kind != 2 && kind != 3 {
			expected := api.ValueTypeI32
			if kind == 11 || kind == 12 {
				expected = api.ValueTypeI64
			}
			require(definition.ParamTypes()[i] == expected, "integer signature mismatch")
		}
		values[i] = value
	}
	returned, err := fn.Call(ctx, values...)
	if err != nil {
		panic(fmt.Sprintf("stellar ffi: module callback failed: %v", err))
	}
	kind := returnKind
	if kind == 0 {
		require(len(returned) == 0, "void return signature mismatch")
		return
	}
	require(len(returned) == 1 && result != 0, "return value storage is missing")
	v := returned[0]
	var ok bool
	switch kind {
	case 1, 2, 9, 10, 14:
		ok = m.WriteUint32Le(result, uint32(v))
	case 3, 11, 12:
		ok = m.WriteUint64Le(result, v)
	case 5, 6:
		ok = m.WriteByte(result, byte(v))
	case 7, 8:
		ok = m.WriteUint16Le(result, uint16(v))
	default:
		panic("stellar ffi: unsupported return scalar")
	}
	require(ok, "return storage outside module memory")
}
