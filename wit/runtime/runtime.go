package runtime

import (
	"fmt"
	"runtime"
	"unsafe"
)

type Handle struct {
	value int32
}

func (h *Handle) Use() int32 {
	if h.value == 0 {
		panic("nil handle")
	}
	return h.value
}

func (h *Handle) Take() int32 {
	if h.value == 0 {
		panic("nil handle")
	}
	value := h.value
	h.value = 0
	return value
}

func (h *Handle) Set(value int32) {
	if value == 0 {
		panic("nil handle")
	}
	if h.value != 0 {
		panic("handle already set")
	}
	h.value = value
}

func (h *Handle) TakeOrNil() int32 {
	value := h.value
	h.value = 0
	return value
}

func MakeHandle(value int32) *Handle {
	if value == 0 {
		panic("nil handle")
	}
	return &Handle{value}
}

func Allocate(pinner *runtime.Pinner, size, align uintptr) unsafe.Pointer {
	pointer := allocateRaw(size, align)
	pinner.Pin(pointer)
	return pointer
}

func allocateRaw(size, align uintptr) unsafe.Pointer {
	if size == 0 {
		return nil
	}

	if size%align != 0 {
		panic(fmt.Sprintf("size %v is not compatible with alignment %v", size, align))
	}

	switch align {
	case 1:
		return unsafe.Pointer(unsafe.SliceData(make([]uint8, size)))
	case 2:
		return unsafe.Pointer(unsafe.SliceData(make([]uint16, size/align)))
	case 4:
		return unsafe.Pointer(unsafe.SliceData(make([]uint32, size/align)))
	case 8:
		return unsafe.Pointer(unsafe.SliceData(make([]uint64, size/align)))
	default:
		panic(fmt.Sprintf("unsupported alignment: %v", align))
	}
}

// NB: `cabi_realloc` may be called before the Go runtime has been initialized,
// in which case we need to use `runtime.sbrk` to do allocations.  The following
// is an abbreviation of [Till's
// efforts](https://github.com/bytecodealliance/go-modules/pull/367).

//go:linkname sbrk runtime.sbrk
func sbrk(n uintptr) unsafe.Pointer

//nolint:unused
var useGCAllocations = false

func init() {
	useGCAllocations = true
}

//nolint:unused
func offset(ptr, align uintptr) uintptr {
	newptr := (ptr + align - 1) &^ (align - 1)
	return newptr - ptr
}

var pinner = runtime.Pinner{}

func Unpin() {
	pinner.Unpin()
}

//nolint:unused
//go:wasmimport wasi_snapshot_preview1 adapter_monotonic_clock_set_paused
func adapterMonotonicClockSetPaused(paused bool)

// `procPin` increments the current M's lock count, which prevents the Go
// runtime from starting a GC cycle or performing GC assist work until
// `procUnpin` is called. The runtime explicitly supports linking to these;
// see https://go.dev/issue/67401.

//nolint:unused
//go:linkname procPin runtime.procPin
func procPin() int

//nolint:unused
//go:linkname procUnpin runtime.procUnpin
func procUnpin()

//nolint:unused
//go:wasmexport cabi_realloc
func cabiRealloc(oldPointer unsafe.Pointer, oldSize, align, newSize uintptr) unsafe.Pointer {
	if oldPointer != nil || oldSize != 0 {
		panic("todo")
	}

	if useGCAllocations {
		// Calls to imports from `cabi_realloc` are forbidden by the
		// component model, so we must not let the Go garbage collector
		// run here. A GC cycle may call imports in several ways: it
		// reads the monotonic and realtime clocks, polls for I/O when
		// restarting the world, and may switch to other goroutines
		// (e.g. to park an assist), any of which may then call imports.
		// Pinning the M defers any GC work until the next allocation
		// made outside of `cabi_realloc`.
		//
		// We additionally call `adapter_monotonic_clock_set_paused`
		// before and after allocating in case anything else reads the
		// monotonic clock while allocating.  See
		// https://github.com/bytecodealliance/wasmtime/pull/13563 for
		// details.
		procPin()
		adapterMonotonicClockSetPaused(true)
		pointer := Allocate(&pinner, newSize, align)
		adapterMonotonicClockSetPaused(false)
		procUnpin()
		return pointer
	} else {
		alignedSize := newSize + offset(newSize, align)
		unaligned := sbrk(alignedSize)
		off := offset(uintptr(unaligned), align)
		return unsafe.Add(unaligned, off)
	}
}
