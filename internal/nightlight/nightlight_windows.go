// Package nightlight reads whether Windows' own Night Light is on.
//
// It matters because the two effects do not add: Windows applies its tint after
// ours, so a screen already warmed by Night Light gets warmed twice and looks
// orange-brown. The tool cannot silently correct for it — it has no idea how
// much the OS is applying — so it reports the conflict and lets the user pick.
//
// The state lives in a CloudStore blob in Bond CompactBinary v1, an
// undocumented format reverse engineered by zomfg/NightLightLibrary and
// Enyium/sem-reg-rs. Only one bit is needed here: whether the inner state struct
// carries field 0, which is present exactly when Night Light is running.
package nightlight

import (
	"errors"
	"syscall"
	"unsafe"
)

// stateKey is the CloudStore path holding the on/off state. The settings blob
// next to it holds the schedule and temperature, which this tool does not need.
//
// Note the path appears twice: CloudStore stores every entry under a container
// key named after the entry, and the value lives one level below that. Reading
// only the container opens successfully and returns zero values, which looks
// exactly like "Night Light was never configured".
const stateKey = `Software\Microsoft\Windows\CurrentVersion\CloudStore\Store\DefaultAccount\Current\default$windows.data.bluelightreduction.bluelightreductionstate\windows.data.bluelightreduction.bluelightreductionstate`

var (
	advapi32          = syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyExW = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValue = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey   = advapi32.NewProc("RegCloseKey")
)

const (
	hkeyCurrentUser = 0x80000001
	keyRead         = 0x00020019
	regBinary       = 3
	// The value is always named Data; the long name in the key path is the key,
	// not the value. Using the path's last segment here silently reads nothing.
	valueName = "Data"
)

// ErrNotConfigured means the user has never opened Night Light settings, so
// there is no state blob at all. That is not an error condition for the caller.
var ErrNotConfigured = errors.New("nightlight: not configured")

// Active reports whether Windows Night Light is on right now.
func Active() (bool, error) {
	blob, err := readState()
	if err != nil {
		return false, err
	}
	inner, err := innerPayload(blob)
	if err != nil {
		return false, err
	}
	return innerOn(inner), nil
}

func readState() ([]byte, error) {
	path, err := syscall.UTF16PtrFromString(stateKey)
	if err != nil {
		return nil, err
	}
	var key syscall.Handle
	if r, _, _ := procRegOpenKeyExW.Call(uintptr(hkeyCurrentUser), uintptr(unsafe.Pointer(path)),
		0, keyRead, uintptr(unsafe.Pointer(&key))); r != 0 {
		return nil, ErrNotConfigured
	}
	defer procRegCloseKey.Call(uintptr(key))

	name, err := syscall.UTF16PtrFromString(valueName)
	if err != nil {
		return nil, err
	}
	var typ, size uint32
	if r, _, _ := procRegQueryValue.Call(uintptr(key), uintptr(unsafe.Pointer(name)), 0,
		uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&size))); r != 0 || typ != regBinary || size == 0 {
		return nil, ErrNotConfigured
	}
	buf := make([]byte, size)
	if r, _, _ := procRegQueryValue.Call(uintptr(key), uintptr(unsafe.Pointer(name)), 0,
		uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r != 0 {
		return nil, ErrNotConfigured
	}
	return buf[:size], nil
}

// innerPayload digs the nested payload out of the CloudStore envelope.
//
// Rather than walking the fixed envelope shape (which differs between Windows
// builds), it searches for the first byte list that itself starts with the
// CompactBinary header. That list is the state struct by construction.
//
// The search has to recurse: the list sits inside two nested structs
// (payload container -> data wrapper -> list), so a top-level scan only ever
// sees structs and walks straight past it.
func innerPayload(blob []byte) ([]byte, error) {
	if len(blob) < 4 || blob[0] != 0x43 || blob[1] != 0x42 {
		return nil, errors.New("nightlight: not a compact-binary blob")
	}
	p := &parser{buf: blob, pos: 4}
	if body, ok := p.findByteList(); ok {
		return body, nil
	}
	return nil, errors.New("nightlight: no inner payload")
}

// findByteList consumes the fields from the current position and returns the
// body of the first int8/uint8 list that is itself a CompactBinary payload.
func (p *parser) findByteList() ([]byte, bool) {
	for p.pos < len(p.buf) {
		typ, ok := p.header()
		if !ok {
			return nil, false
		}
		switch typ {
		case btStop:
			continue
		case btList, btSet:
			save := p.pos
			if body, ok := p.listOfBytes(); ok {
				if len(body) >= 4 && body[0] == 0x43 && body[1] == 0x42 {
					return body, true
				}
				continue
			}
			// Not a byte list: rewind and step over it generically.
			p.pos = save
			p.skip(typ)
		case btStruct:
			if body, ok := p.findByteList(); ok {
				return body, true
			}
		default:
			p.skip(typ)
		}
	}
	return nil, false
}

// innerOn reports whether the state struct carries field 0, which Windows sets
// only while Night Light is on.
func innerOn(inner []byte) bool {
	p := &parser{buf: inner, pos: 4}
	for p.pos < len(p.buf) {
		id, typ, ok := p.fieldHeader()
		if !ok {
			return false
		}
		if typ == btStop {
			continue
		}
		if id == 0 {
			return true
		}
		p.skip(typ)
	}
	return false
}

// Bond CompactBinary v1 type ids. Only the ones the state blob can contain are
// named; everything else is skipped by its generic shape.
const (
	btStop    = 0
	btBool    = 2
	btUint8   = 3
	btUint16  = 4
	btUint32  = 5
	btUint64  = 6
	btFloat   = 7
	btDouble  = 8
	btString  = 9
	btStruct  = 10
	btList    = 11
	btSet     = 12
	btMap     = 13
	btInt8    = 14
	btInt16   = 15
	btInt32   = 16
	btInt64   = 17
	btWString = 18
)

type parser struct {
	buf []byte
	pos int
}

// fieldHeader reads a field header and returns the absolute field id and type.
func (p *parser) fieldHeader() (uint32, byte, bool) {
	if p.pos >= len(p.buf) {
		return 0, 0, false
	}
	b := p.buf[p.pos]
	p.pos++
	idBits := b >> 5
	typ := b & 0x1F
	switch idBits {
	case 0, 1, 2, 3, 4, 5:
		return uint32(idBits), typ, true
	case 6:
		if p.pos >= len(p.buf) {
			return 0, 0, false
		}
		id := uint32(p.buf[p.pos])
		p.pos++
		return id, typ, true
	default:
		if p.pos+1 >= len(p.buf) {
			return 0, 0, false
		}
		id := uint32(p.buf[p.pos]) | uint32(p.buf[p.pos+1])<<8
		p.pos += 2
		return id, typ, true
	}
}

// header reads a field header when the id is not needed.
func (p *parser) header() (byte, bool) {
	_, typ, ok := p.fieldHeader()
	return typ, ok
}

// listOfBytes consumes a list whose element type is int8 or uint8 and returns
// its body.
func (p *parser) listOfBytes() ([]byte, bool) {
	if p.pos >= len(p.buf) {
		return nil, false
	}
	elem := p.buf[p.pos]
	p.pos++
	if elem != btInt8 && elem != btUint8 {
		return nil, false
	}
	n, ok := p.varint()
	if !ok || p.pos+int(n) > len(p.buf) {
		return nil, false
	}
	body := p.buf[p.pos : p.pos+int(n)]
	p.pos += int(n)
	return body, true
}

func (p *parser) varint() (uint64, bool) {
	var out uint64
	var shift uint
	for {
		if p.pos >= len(p.buf) || shift > 63 {
			return 0, false
		}
		b := p.buf[p.pos]
		p.pos++
		out |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return out, true
		}
		shift += 7
	}
}

// skip advances past one value of the given type.
func (p *parser) skip(typ byte) {
	switch typ {
	case btBool, btUint8, btInt8:
		p.pos++
	case btUint16, btUint32, btUint64, btInt16, btInt32, btInt64:
		p.varint()
	case btFloat:
		p.pos += 4
	case btDouble:
		p.pos += 8
	case btString:
		if n, ok := p.varint(); ok {
			p.pos += int(n)
		}
	case btWString:
		if n, ok := p.varint(); ok {
			p.pos += int(n) * 2
		}
	case btStruct:
		for p.pos < len(p.buf) {
			t, ok := p.header()
			if !ok {
				return
			}
			if t == btStop {
				return
			}
			p.skip(t)
		}
	case btList, btSet:
		if p.pos >= len(p.buf) {
			return
		}
		elem := p.buf[p.pos]
		p.pos++
		n, ok := p.varint()
		if !ok {
			return
		}
		for i := uint64(0); i < n && p.pos < len(p.buf); i++ {
			p.skip(elem)
		}
	case btMap:
		if p.pos+1 >= len(p.buf) {
			return
		}
		key, val := p.buf[p.pos], p.buf[p.pos+1]
		p.pos += 2
		n, ok := p.varint()
		if !ok {
			return
		}
		for i := uint64(0); i < n && p.pos < len(p.buf); i++ {
			p.skip(key)
			p.skip(val)
		}
	}
}
