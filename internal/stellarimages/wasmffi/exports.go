package wasmffi

import (
	"bytes"
	"fmt"
)

type reader struct {
	b []byte
	p int
}

func (r *reader) u32() (uint32, error) {
	var v uint32
	for shift := uint(0); shift < 35; shift += 7 {
		if r.p >= len(r.b) {
			return 0, fmt.Errorf("truncated wasm integer")
		}
		c := r.b[r.p]
		r.p++
		v |= uint32(c&127) << shift
		if c&128 == 0 {
			return v, nil
		}
	}
	return 0, fmt.Errorf("oversized wasm integer")
}
func (r *reader) skip(n uint32) error {
	if uint64(r.p)+uint64(n) > uint64(len(r.b)) {
		return fmt.Errorf("truncated wasm payload")
	}
	r.p += int(n)
	return nil
}
func leb(v uint32) []byte {
	out := []byte{}
	for {
		b := byte(v & 127)
		v >>= 7
		if v != 0 {
			b |= 128
		}
		out = append(out, b)
		if v == 0 {
			return out
		}
	}
}
func section(id byte, b []byte) []byte {
	return append(append([]byte{id}, leb(uint32(len(b)))...), b...)
}
func export(name string, index uint32) []byte {
	b := append(leb(uint32(len(name))), []byte(name)...)
	b = append(b, 0)
	return append(b, leb(index)...)
}

// ExportScalarFunctions 为固定函数表添加别名，宿主据此调用正确 WASM 签名。
// 只改导出表，不替换原函数、指令、ABI 或渲染实现。
func ExportScalarFunctions(wasm []byte) ([]byte, error) {
	if len(wasm) < 8 || !bytes.Equal(wasm[:8], []byte{0, 97, 115, 109, 1, 0, 0, 0}) {
		return nil, fmt.Errorf("invalid wasm header")
	}
	type part struct {
		id byte
		b  []byte
	}
	parts := []part{}
	r := reader{b: wasm, p: 8}
	aliases := []byte{}
	aliasCount := uint32(0)
	for r.p < len(r.b) {
		id := r.b[r.p]
		r.p++
		size, e := r.u32()
		if e != nil {
			return nil, e
		}
		start := r.p
		if e = r.skip(size); e != nil {
			return nil, e
		}
		b := r.b[start:r.p]
		parts = append(parts, part{id, b})
		if id != 9 {
			continue
		}
		elements := reader{b: b}
		count, e := elements.u32()
		if e != nil {
			return nil, e
		}
		for i := uint32(0); i < count; i++ {
			flags, e := elements.u32()
			if e != nil {
				return nil, e
			}
			if flags != 0 && flags != 2 {
				return nil, fmt.Errorf("unsupported static table segment flags %d", flags)
			}
			if flags == 2 {
				table, e := elements.u32()
				if e != nil {
					return nil, e
				}
				if table != 0 {
					return nil, fmt.Errorf("only the primary function table is supported")
				}
			}
			if elements.p >= len(b) || b[elements.p] != 0x41 {
				return nil, fmt.Errorf("function table offset is not constant")
			}
			elements.p++
			base, e := elements.u32()
			if e != nil {
				return nil, e
			}
			if elements.p >= len(b) || b[elements.p] != 0x0b {
				return nil, fmt.Errorf("invalid table offset expression")
			}
			elements.p++
			if flags == 2 {
				if elements.p >= len(b) || b[elements.p] != 0 {
					return nil, fmt.Errorf("invalid function table element kind")
				}
				elements.p++
			}
			n, e := elements.u32()
			if e != nil {
				return nil, e
			}
			for j := uint32(0); j < n; j++ {
				index, e := elements.u32()
				if e != nil {
					return nil, e
				}
				aliases = append(aliases, export(fmt.Sprintf("%s%d", functionPrefix, base+j), index)...)
				aliasCount++
			}
		}
		if elements.p != len(b) {
			return nil, fmt.Errorf("trailing function table data")
		}
	}
	out := append([]byte(nil), wasm[:8]...)
	found := false
	for _, p := range parts {
		payload := p.b
		if p.id == 7 {
			found = true
			e := reader{b: payload}
			count, err := e.u32()
			if err != nil {
				return nil, err
			}
			payload = append(append(leb(count+aliasCount), payload[e.p:]...), aliases...)
		}
		out = append(out, section(p.id, payload)...)
	}
	if !found {
		return nil, fmt.Errorf("module has no export section")
	}
	if aliasCount == 0 {
		return nil, fmt.Errorf("module has no fixed callback table")
	}
	return out, nil
}
