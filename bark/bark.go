package bark

// #include <bark.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync/atomic"
	"unsafe"
)

// This is needed, because as of go 1.24
// type RustBuffer C.RustBuffer cannot have methods,
// RustBuffer is treated as non-local type
type GoRustBuffer struct {
	inner C.RustBuffer
}

type RustBufferI interface {
	AsReader() *bytes.Reader
	Free()
	ToGoBytes() []byte
	Data() unsafe.Pointer
	Len() uint64
	Capacity() uint64
}

func RustBufferFromExternal(b RustBufferI) GoRustBuffer {
	return GoRustBuffer{
		inner: C.RustBuffer{
			capacity: C.uint64_t(b.Capacity()),
			len:      C.uint64_t(b.Len()),
			data:     (*C.uchar)(b.Data()),
		},
	}
}

func (cb GoRustBuffer) Capacity() uint64 {
	return uint64(cb.inner.capacity)
}

func (cb GoRustBuffer) Len() uint64 {
	return uint64(cb.inner.len)
}

func (cb GoRustBuffer) Data() unsafe.Pointer {
	return unsafe.Pointer(cb.inner.data)
}

func (cb GoRustBuffer) AsReader() *bytes.Reader {
	b := unsafe.Slice((*byte)(cb.inner.data), C.uint64_t(cb.inner.len))
	return bytes.NewReader(b)
}

func (cb GoRustBuffer) Free() {
	rustCall(func(status *C.RustCallStatus) bool {
		C.ffi_bark_rustbuffer_free(cb.inner, status)
		return false
	})
}

func (cb GoRustBuffer) ToGoBytes() []byte {
	return C.GoBytes(unsafe.Pointer(cb.inner.data), C.int(cb.inner.len))
}

func stringToRustBuffer(str string) C.RustBuffer {
	return bytesToRustBuffer([]byte(str))
}

func bytesToRustBuffer(b []byte) C.RustBuffer {
	if len(b) == 0 {
		return C.RustBuffer{}
	}
	// We can pass the pointer along here, as it is pinned
	// for the duration of this call
	foreign := C.ForeignBytes{
		len:  C.int(len(b)),
		data: (*C.uchar)(unsafe.Pointer(&b[0])),
	}

	return rustCall(func(status *C.RustCallStatus) C.RustBuffer {
		return C.ffi_bark_rustbuffer_from_bytes(foreign, status)
	})
}

type BufLifter[GoType any] interface {
	Lift(value RustBufferI) GoType
}

type BufLowerer[GoType any] interface {
	Lower(value GoType) C.RustBuffer
}

type BufReader[GoType any] interface {
	Read(reader io.Reader) GoType
}

type BufWriter[GoType any] interface {
	Write(writer io.Writer, value GoType)
}

func LowerIntoRustBuffer[GoType any](bufWriter BufWriter[GoType], value GoType) C.RustBuffer {
	// This might be not the most efficient way but it does not require knowing allocation size
	// beforehand
	var buffer bytes.Buffer
	bufWriter.Write(&buffer, value)

	bytes, err := io.ReadAll(&buffer)
	if err != nil {
		panic(fmt.Errorf("reading written data: %w", err))
	}
	return bytesToRustBuffer(bytes)
}

func LiftFromRustBuffer[GoType any](bufReader BufReader[GoType], rbuf RustBufferI) GoType {
	defer rbuf.Free()
	reader := rbuf.AsReader()
	item := bufReader.Read(reader)
	if reader.Len() > 0 {
		// TODO: Remove this
		leftover, _ := io.ReadAll(reader)
		panic(fmt.Errorf("Junk remaining in buffer after lifting: %s", string(leftover)))
	}
	return item
}

func rustCallWithError[E any, U any](converter BufReader[*E], callback func(*C.RustCallStatus) U) (U, *E) {
	var status C.RustCallStatus
	returnValue := callback(&status)
	err := checkCallStatus(converter, status)
	return returnValue, err
}

func checkCallStatus[E any](converter BufReader[*E], status C.RustCallStatus) *E {
	switch status.code {
	case 0:
		return nil
	case 1:
		return LiftFromRustBuffer(converter, GoRustBuffer{inner: status.errorBuf})
	case 2:
		// when the rust code sees a panic, it tries to construct a rustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{inner: status.errorBuf})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		panic(fmt.Errorf("unknown status code: %d", status.code))
	}
}

func checkCallStatusUnknown(status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		panic(fmt.Errorf("function not returning an error returned an error"))
	case 2:
		// when the rust code sees a panic, it tries to construct a C.RustBuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(GoRustBuffer{
				inner: status.errorBuf,
			})))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func rustCall[U any](callback func(*C.RustCallStatus) U) U {
	returnValue, err := rustCallWithError[error](nil, callback)
	if err != nil {
		panic(err)
	}
	return returnValue
}

type NativeError interface {
	AsError() error
}

func writeInt8(writer io.Writer, value int8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint8(writer io.Writer, value uint8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt16(writer io.Writer, value int16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint16(writer io.Writer, value uint16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt32(writer io.Writer, value int32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint32(writer io.Writer, value uint32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt64(writer io.Writer, value int64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint64(writer io.Writer, value uint64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat32(writer io.Writer, value float32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat64(writer io.Writer, value float64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func readInt8(reader io.Reader) int8 {
	var result int8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint8(reader io.Reader) uint8 {
	var result uint8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt16(reader io.Reader) int16 {
	var result int16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint16(reader io.Reader) uint16 {
	var result uint16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt32(reader io.Reader) int32 {
	var result int32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint32(reader io.Reader) uint32 {
	var result uint32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt64(reader io.Reader) int64 {
	var result int64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint64(reader io.Reader) uint64 {
	var result uint64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat32(reader io.Reader) float32 {
	var result float32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat64(reader io.Reader) float64 {
	var result float64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func init() {

	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 26
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_bark_uniffi_contract_version()
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("bark: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_func_create_wallet()
		})
		if checksum != 59629 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_func_create_wallet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_func_open_wallet()
		})
		if checksum != 15440 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_func_open_wallet: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_ark_info()
		})
		if checksum != 5686 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_ark_info: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_board_all()
		})
		if checksum != 5752 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_board_all: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_bolt11_invoice()
		})
		if checksum != 65315 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_bolt11_invoice: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_claim_bolt11_payment()
		})
		if checksum != 37734 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_claim_bolt11_payment: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_exit_all()
		})
		if checksum != 45736 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_exit_all: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_exit_status()
		})
		if checksum != 1084 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_exit_status: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_maintenance()
		})
		if checksum != 48568 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_maintenance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_movements()
		})
		if checksum != 12620 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_movements: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_new_address()
		})
		if checksum != 11647 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_new_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_offboard_all()
		})
		if checksum != 38640 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_offboard_all: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_onchain_address()
		})
		if checksum != 45797 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_onchain_address: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_onchain_balance()
		})
		if checksum != 23885 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_onchain_balance: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_onchain_transactions()
		})
		if checksum != 57700 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_onchain_transactions: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_pay_bolt11()
		})
		if checksum != 50495 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_pay_bolt11: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_refresh_all()
		})
		if checksum != 16084 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_refresh_all: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_send()
		})
		if checksum != 55929 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_send: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_send_onchain()
		})
		if checksum != 399 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_send_onchain: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_sync()
		})
		if checksum != 20192 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_sync: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_utxos()
		})
		if checksum != 29454 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_utxos: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_vtxos()
		})
		if checksum != 31673 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_vtxos: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_bark_checksum_method_wallet_wallet_balance()
		})
		if checksum != 32002 {
			// If this happens try cleaning and rebuilding your project
			panic("bark: uniffi_bark_checksum_method_wallet_wallet_balance: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint16 struct{}

var FfiConverterUint16INSTANCE = FfiConverterUint16{}

func (FfiConverterUint16) Lower(value uint16) C.uint16_t {
	return C.uint16_t(value)
}

func (FfiConverterUint16) Write(writer io.Writer, value uint16) {
	writeUint16(writer, value)
}

func (FfiConverterUint16) Lift(value C.uint16_t) uint16 {
	return uint16(value)
}

func (FfiConverterUint16) Read(reader io.Reader) uint16 {
	return readUint16(reader)
}

type FfiDestroyerUint16 struct{}

func (FfiDestroyerUint16) Destroy(_ uint16) {}

type FfiConverterUint32 struct{}

var FfiConverterUint32INSTANCE = FfiConverterUint32{}

func (FfiConverterUint32) Lower(value uint32) C.uint32_t {
	return C.uint32_t(value)
}

func (FfiConverterUint32) Write(writer io.Writer, value uint32) {
	writeUint32(writer, value)
}

func (FfiConverterUint32) Lift(value C.uint32_t) uint32 {
	return uint32(value)
}

func (FfiConverterUint32) Read(reader io.Reader) uint32 {
	return readUint32(reader)
}

type FfiDestroyerUint32 struct{}

func (FfiDestroyerUint32) Destroy(_ uint32) {}

type FfiConverterUint64 struct{}

var FfiConverterUint64INSTANCE = FfiConverterUint64{}

func (FfiConverterUint64) Lower(value uint64) C.uint64_t {
	return C.uint64_t(value)
}

func (FfiConverterUint64) Write(writer io.Writer, value uint64) {
	writeUint64(writer, value)
}

func (FfiConverterUint64) Lift(value C.uint64_t) uint64 {
	return uint64(value)
}

func (FfiConverterUint64) Read(reader io.Reader) uint64 {
	return readUint64(reader)
}

type FfiDestroyerUint64 struct{}

func (FfiDestroyerUint64) Destroy(_ uint64) {}

type FfiConverterBool struct{}

var FfiConverterBoolINSTANCE = FfiConverterBool{}

func (FfiConverterBool) Lower(value bool) C.int8_t {
	if value {
		return C.int8_t(1)
	}
	return C.int8_t(0)
}

func (FfiConverterBool) Write(writer io.Writer, value bool) {
	if value {
		writeInt8(writer, 1)
	} else {
		writeInt8(writer, 0)
	}
}

func (FfiConverterBool) Lift(value C.int8_t) bool {
	return value != 0
}

func (FfiConverterBool) Read(reader io.Reader) bool {
	return readInt8(reader) != 0
}

type FfiDestroyerBool struct{}

func (FfiDestroyerBool) Destroy(_ bool) {}

type FfiConverterString struct{}

var FfiConverterStringINSTANCE = FfiConverterString{}

func (FfiConverterString) Lift(rb RustBufferI) string {
	defer rb.Free()
	reader := rb.AsReader()
	b, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("reading reader: %w", err))
	}
	return string(b)
}

func (FfiConverterString) Read(reader io.Reader) string {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil && err != io.EOF {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading string, expected %d, read %d", length, read_length))
	}
	return string(buffer)
}

func (FfiConverterString) Lower(value string) C.RustBuffer {
	return stringToRustBuffer(value)
}

func (FfiConverterString) Write(writer io.Writer, value string) {
	if len(value) > math.MaxInt32 {
		panic("String is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := io.WriteString(writer, value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing string, expected %d, written %d", len(value), write_length))
	}
}

type FfiDestroyerString struct{}

func (FfiDestroyerString) Destroy(_ string) {}

// Below is an implementation of synchronization requirements outlined in the link.
// https://github.com/mozilla/uniffi-rs/blob/0dc031132d9493ca812c3af6e7dd60ad2ea95bf0/uniffi_bindgen/src/bindings/kotlin/templates/ObjectRuntime.kt#L31

type FfiObject struct {
	pointer       unsafe.Pointer
	callCounter   atomic.Int64
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer
	freeFunction  func(unsafe.Pointer, *C.RustCallStatus)
	destroyed     atomic.Bool
}

func newFfiObject(
	pointer unsafe.Pointer,
	cloneFunction func(unsafe.Pointer, *C.RustCallStatus) unsafe.Pointer,
	freeFunction func(unsafe.Pointer, *C.RustCallStatus),
) FfiObject {
	return FfiObject{
		pointer:       pointer,
		cloneFunction: cloneFunction,
		freeFunction:  freeFunction,
	}
}

func (ffiObject *FfiObject) incrementPointer(debugName string) unsafe.Pointer {
	for {
		counter := ffiObject.callCounter.Load()
		if counter <= -1 {
			panic(fmt.Errorf("%v object has already been destroyed", debugName))
		}
		if counter == math.MaxInt64 {
			panic(fmt.Errorf("%v object call counter would overflow", debugName))
		}
		if ffiObject.callCounter.CompareAndSwap(counter, counter+1) {
			break
		}
	}

	return rustCall(func(status *C.RustCallStatus) unsafe.Pointer {
		return ffiObject.cloneFunction(ffiObject.pointer, status)
	})
}

func (ffiObject *FfiObject) decrementPointer() {
	if ffiObject.callCounter.Add(-1) == -1 {
		ffiObject.freeRustArcPtr()
	}
}

func (ffiObject *FfiObject) destroy() {
	if ffiObject.destroyed.CompareAndSwap(false, true) {
		if ffiObject.callCounter.Add(-1) == -1 {
			ffiObject.freeRustArcPtr()
		}
	}
}

func (ffiObject *FfiObject) freeRustArcPtr() {
	rustCall(func(status *C.RustCallStatus) int32 {
		ffiObject.freeFunction(ffiObject.pointer, status)
		return 0
	})
}

type WalletInterface interface {
	ArkInfo() (ArkInfo, error)
	BoardAll() error
	Bolt11Invoice(amountSats uint64) (Bolt11Invoice, error)
	ClaimBolt11Payment(invoice Bolt11Invoice) error
	ExitAll() error
	ExitStatus() (ExitStatus, error)
	Maintenance() error
	Movements() ([]Movement, error)
	NewAddress() (BarkAddress, error)
	OffboardAll() error
	OnchainAddress() (string, error)
	OnchainBalance() (OnchainBalance, error)
	OnchainTransactions() []OnchainTransaction
	PayBolt11(invoice Bolt11Invoice, amountSats *uint64) (string, error)
	RefreshAll() error
	Send(destination BarkAddress, amountSats uint64) ([]Vtxo, error)
	SendOnchain(address string, amountSats uint64) (string, error)
	Sync() error
	Utxos() []Utxo
	Vtxos() ([]Vtxo, error)
	WalletBalance() (WalletBalance, error)
}
type Wallet struct {
	ffiObject FfiObject
}

func (_self *Wallet) ArkInfo() (ArkInfo, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_ark_info(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue ArkInfo
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterArkInfoINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) BoardAll() error {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_bark_fn_method_wallet_board_all(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *Wallet) Bolt11Invoice(amountSats uint64) (Bolt11Invoice, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_bolt11_invoice(
				_pointer, FfiConverterUint64INSTANCE.Lower(amountSats), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue Bolt11Invoice
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTypeBolt11InvoiceINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) ClaimBolt11Payment(invoice Bolt11Invoice) error {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_bark_fn_method_wallet_claim_bolt11_payment(
			_pointer, FfiConverterTypeBolt11InvoiceINSTANCE.Lower(invoice), _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *Wallet) ExitAll() error {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_bark_fn_method_wallet_exit_all(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *Wallet) ExitStatus() (ExitStatus, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_exit_status(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue ExitStatus
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterExitStatusINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) Maintenance() error {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_bark_fn_method_wallet_maintenance(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *Wallet) Movements() ([]Movement, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_movements(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []Movement
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceMovementINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) NewAddress() (BarkAddress, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_new_address(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue BarkAddress
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTypeBarkAddressINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) OffboardAll() error {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_bark_fn_method_wallet_offboard_all(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *Wallet) OnchainAddress() (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_onchain_address(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) OnchainBalance() (OnchainBalance, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_onchain_balance(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue OnchainBalance
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOnchainBalanceINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) OnchainTransactions() []OnchainTransaction {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceOnchainTransactionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_onchain_transactions(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Wallet) PayBolt11(invoice Bolt11Invoice, amountSats *uint64) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_pay_bolt11(
				_pointer, FfiConverterTypeBolt11InvoiceINSTANCE.Lower(invoice), FfiConverterOptionalUint64INSTANCE.Lower(amountSats), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) RefreshAll() error {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_bark_fn_method_wallet_refresh_all(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *Wallet) Send(destination BarkAddress, amountSats uint64) ([]Vtxo, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_send(
				_pointer, FfiConverterTypeBarkAddressINSTANCE.Lower(destination), FfiConverterUint64INSTANCE.Lower(amountSats), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []Vtxo
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceVtxoINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) SendOnchain(address string, amountSats uint64) (string, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_send_onchain(
				_pointer, FfiConverterStringINSTANCE.Lower(address), FfiConverterUint64INSTANCE.Lower(amountSats), _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) Sync() error {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_bark_fn_method_wallet_sync(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr.AsError()
}

func (_self *Wallet) Utxos() []Utxo {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUtxoINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_utxos(
				_pointer, _uniffiStatus),
		}
	}))
}

func (_self *Wallet) Vtxos() ([]Vtxo, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_vtxos(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []Vtxo
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceVtxoINSTANCE.Lift(_uniffiRV), nil
	}
}

func (_self *Wallet) WalletBalance() (WalletBalance, error) {
	_pointer := _self.ffiObject.incrementPointer("*Wallet")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return GoRustBuffer{
			inner: C.uniffi_bark_fn_method_wallet_wallet_balance(
				_pointer, _uniffiStatus),
		}
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue WalletBalance
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWalletBalanceINSTANCE.Lift(_uniffiRV), nil
	}
}
func (object *Wallet) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterWallet struct{}

var FfiConverterWalletINSTANCE = FfiConverterWallet{}

func (c FfiConverterWallet) Lift(pointer unsafe.Pointer) *Wallet {
	result := &Wallet{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) unsafe.Pointer {
				return C.uniffi_bark_fn_clone_wallet(pointer, status)
			},
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_bark_fn_free_wallet(pointer, status)
			},
		),
	}
	runtime.SetFinalizer(result, (*Wallet).Destroy)
	return result
}

func (c FfiConverterWallet) Read(reader io.Reader) *Wallet {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterWallet) Lower(value *Wallet) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Wallet")
	defer value.ffiObject.decrementPointer()
	return pointer

}

func (c FfiConverterWallet) Write(writer io.Writer, value *Wallet) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerWallet struct{}

func (_ FfiDestroyerWallet) Destroy(value *Wallet) {
	value.Destroy()
}

type ArkInfo struct {
	Network           Network
	AspPubkey         PublicKey
	RoundIntervalSec  uint64
	NbRoundNonces     uint64
	VtxoExitDelta     uint16
	VtxoExpiryDelta   uint16
	MaxVtxoAmountSats *uint64
}

func (r *ArkInfo) Destroy() {
	FfiDestroyerTypeNetwork{}.Destroy(r.Network)
	FfiDestroyerTypePublicKey{}.Destroy(r.AspPubkey)
	FfiDestroyerUint64{}.Destroy(r.RoundIntervalSec)
	FfiDestroyerUint64{}.Destroy(r.NbRoundNonces)
	FfiDestroyerUint16{}.Destroy(r.VtxoExitDelta)
	FfiDestroyerUint16{}.Destroy(r.VtxoExpiryDelta)
	FfiDestroyerOptionalUint64{}.Destroy(r.MaxVtxoAmountSats)
}

type FfiConverterArkInfo struct{}

var FfiConverterArkInfoINSTANCE = FfiConverterArkInfo{}

func (c FfiConverterArkInfo) Lift(rb RustBufferI) ArkInfo {
	return LiftFromRustBuffer[ArkInfo](c, rb)
}

func (c FfiConverterArkInfo) Read(reader io.Reader) ArkInfo {
	return ArkInfo{
		FfiConverterTypeNetworkINSTANCE.Read(reader),
		FfiConverterTypePublicKeyINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint16INSTANCE.Read(reader),
		FfiConverterUint16INSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterArkInfo) Lower(value ArkInfo) C.RustBuffer {
	return LowerIntoRustBuffer[ArkInfo](c, value)
}

func (c FfiConverterArkInfo) Write(writer io.Writer, value ArkInfo) {
	FfiConverterTypeNetworkINSTANCE.Write(writer, value.Network)
	FfiConverterTypePublicKeyINSTANCE.Write(writer, value.AspPubkey)
	FfiConverterUint64INSTANCE.Write(writer, value.RoundIntervalSec)
	FfiConverterUint64INSTANCE.Write(writer, value.NbRoundNonces)
	FfiConverterUint16INSTANCE.Write(writer, value.VtxoExitDelta)
	FfiConverterUint16INSTANCE.Write(writer, value.VtxoExpiryDelta)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.MaxVtxoAmountSats)
}

type FfiDestroyerArkInfo struct{}

func (_ FfiDestroyerArkInfo) Destroy(value ArkInfo) {
	value.Destroy()
}

type Config struct {
	Network        Network
	AspAddress     string
	EsploraAddress string
}

func (r *Config) Destroy() {
	FfiDestroyerTypeNetwork{}.Destroy(r.Network)
	FfiDestroyerString{}.Destroy(r.AspAddress)
	FfiDestroyerString{}.Destroy(r.EsploraAddress)
}

type FfiConverterConfig struct{}

var FfiConverterConfigINSTANCE = FfiConverterConfig{}

func (c FfiConverterConfig) Lift(rb RustBufferI) Config {
	return LiftFromRustBuffer[Config](c, rb)
}

func (c FfiConverterConfig) Read(reader io.Reader) Config {
	return Config{
		FfiConverterTypeNetworkINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterConfig) Lower(value Config) C.RustBuffer {
	return LowerIntoRustBuffer[Config](c, value)
}

func (c FfiConverterConfig) Write(writer io.Writer, value Config) {
	FfiConverterTypeNetworkINSTANCE.Write(writer, value.Network)
	FfiConverterStringINSTANCE.Write(writer, value.AspAddress)
	FfiConverterStringINSTANCE.Write(writer, value.EsploraAddress)
}

type FfiDestroyerConfig struct{}

func (_ FfiDestroyerConfig) Destroy(value Config) {
	value.Destroy()
}

type ExitStatus struct {
	Done   bool
	Height *uint32
}

func (r *ExitStatus) Destroy() {
	FfiDestroyerBool{}.Destroy(r.Done)
	FfiDestroyerOptionalUint32{}.Destroy(r.Height)
}

type FfiConverterExitStatus struct{}

var FfiConverterExitStatusINSTANCE = FfiConverterExitStatus{}

func (c FfiConverterExitStatus) Lift(rb RustBufferI) ExitStatus {
	return LiftFromRustBuffer[ExitStatus](c, rb)
}

func (c FfiConverterExitStatus) Read(reader io.Reader) ExitStatus {
	return ExitStatus{
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterOptionalUint32INSTANCE.Read(reader),
	}
}

func (c FfiConverterExitStatus) Lower(value ExitStatus) C.RustBuffer {
	return LowerIntoRustBuffer[ExitStatus](c, value)
}

func (c FfiConverterExitStatus) Write(writer io.Writer, value ExitStatus) {
	FfiConverterBoolINSTANCE.Write(writer, value.Done)
	FfiConverterOptionalUint32INSTANCE.Write(writer, value.Height)
}

type FfiDestroyerExitStatus struct{}

func (_ FfiDestroyerExitStatus) Destroy(value ExitStatus) {
	value.Destroy()
}

type Movement struct {
	Id                uint32
	Kind              MovementKind
	AmountSentSat     uint64
	AmountReceivedSat uint64
	FeesSat           uint64
	CreatedAt         string
}

func (r *Movement) Destroy() {
	FfiDestroyerUint32{}.Destroy(r.Id)
	FfiDestroyerMovementKind{}.Destroy(r.Kind)
	FfiDestroyerUint64{}.Destroy(r.AmountSentSat)
	FfiDestroyerUint64{}.Destroy(r.AmountReceivedSat)
	FfiDestroyerUint64{}.Destroy(r.FeesSat)
	FfiDestroyerString{}.Destroy(r.CreatedAt)
}

type FfiConverterMovement struct{}

var FfiConverterMovementINSTANCE = FfiConverterMovement{}

func (c FfiConverterMovement) Lift(rb RustBufferI) Movement {
	return LiftFromRustBuffer[Movement](c, rb)
}

func (c FfiConverterMovement) Read(reader io.Reader) Movement {
	return Movement{
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterMovementKindINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterMovement) Lower(value Movement) C.RustBuffer {
	return LowerIntoRustBuffer[Movement](c, value)
}

func (c FfiConverterMovement) Write(writer io.Writer, value Movement) {
	FfiConverterUint32INSTANCE.Write(writer, value.Id)
	FfiConverterMovementKindINSTANCE.Write(writer, value.Kind)
	FfiConverterUint64INSTANCE.Write(writer, value.AmountSentSat)
	FfiConverterUint64INSTANCE.Write(writer, value.AmountReceivedSat)
	FfiConverterUint64INSTANCE.Write(writer, value.FeesSat)
	FfiConverterStringINSTANCE.Write(writer, value.CreatedAt)
}

type FfiDestroyerMovement struct{}

func (_ FfiDestroyerMovement) Destroy(value Movement) {
	value.Destroy()
}

type OnchainBalance struct {
	TrustedSpendableSat uint64
	TotalSat            uint64
}

func (r *OnchainBalance) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.TrustedSpendableSat)
	FfiDestroyerUint64{}.Destroy(r.TotalSat)
}

type FfiConverterOnchainBalance struct{}

var FfiConverterOnchainBalanceINSTANCE = FfiConverterOnchainBalance{}

func (c FfiConverterOnchainBalance) Lift(rb RustBufferI) OnchainBalance {
	return LiftFromRustBuffer[OnchainBalance](c, rb)
}

func (c FfiConverterOnchainBalance) Read(reader io.Reader) OnchainBalance {
	return OnchainBalance{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterOnchainBalance) Lower(value OnchainBalance) C.RustBuffer {
	return LowerIntoRustBuffer[OnchainBalance](c, value)
}

func (c FfiConverterOnchainBalance) Write(writer io.Writer, value OnchainBalance) {
	FfiConverterUint64INSTANCE.Write(writer, value.TrustedSpendableSat)
	FfiConverterUint64INSTANCE.Write(writer, value.TotalSat)
}

type FfiDestroyerOnchainBalance struct{}

func (_ FfiDestroyerOnchainBalance) Destroy(value OnchainBalance) {
	value.Destroy()
}

type OnchainTransaction struct {
	Txid             string
	AmountSat        uint64
	CreatedAt        uint64
	State            string
	TxType           string
	NumConfirmations uint32
}

func (r *OnchainTransaction) Destroy() {
	FfiDestroyerString{}.Destroy(r.Txid)
	FfiDestroyerUint64{}.Destroy(r.AmountSat)
	FfiDestroyerUint64{}.Destroy(r.CreatedAt)
	FfiDestroyerString{}.Destroy(r.State)
	FfiDestroyerString{}.Destroy(r.TxType)
	FfiDestroyerUint32{}.Destroy(r.NumConfirmations)
}

type FfiConverterOnchainTransaction struct{}

var FfiConverterOnchainTransactionINSTANCE = FfiConverterOnchainTransaction{}

func (c FfiConverterOnchainTransaction) Lift(rb RustBufferI) OnchainTransaction {
	return LiftFromRustBuffer[OnchainTransaction](c, rb)
}

func (c FfiConverterOnchainTransaction) Read(reader io.Reader) OnchainTransaction {
	return OnchainTransaction{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
	}
}

func (c FfiConverterOnchainTransaction) Lower(value OnchainTransaction) C.RustBuffer {
	return LowerIntoRustBuffer[OnchainTransaction](c, value)
}

func (c FfiConverterOnchainTransaction) Write(writer io.Writer, value OnchainTransaction) {
	FfiConverterStringINSTANCE.Write(writer, value.Txid)
	FfiConverterUint64INSTANCE.Write(writer, value.AmountSat)
	FfiConverterUint64INSTANCE.Write(writer, value.CreatedAt)
	FfiConverterStringINSTANCE.Write(writer, value.State)
	FfiConverterStringINSTANCE.Write(writer, value.TxType)
	FfiConverterUint32INSTANCE.Write(writer, value.NumConfirmations)
}

type FfiDestroyerOnchainTransaction struct{}

func (_ FfiDestroyerOnchainTransaction) Destroy(value OnchainTransaction) {
	value.Destroy()
}

type OutPoint struct {
	Txid string
	Vout uint32
}

func (r *OutPoint) Destroy() {
	FfiDestroyerString{}.Destroy(r.Txid)
	FfiDestroyerUint32{}.Destroy(r.Vout)
}

type FfiConverterOutPoint struct{}

var FfiConverterOutPointINSTANCE = FfiConverterOutPoint{}

func (c FfiConverterOutPoint) Lift(rb RustBufferI) OutPoint {
	return LiftFromRustBuffer[OutPoint](c, rb)
}

func (c FfiConverterOutPoint) Read(reader io.Reader) OutPoint {
	return OutPoint{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
	}
}

func (c FfiConverterOutPoint) Lower(value OutPoint) C.RustBuffer {
	return LowerIntoRustBuffer[OutPoint](c, value)
}

func (c FfiConverterOutPoint) Write(writer io.Writer, value OutPoint) {
	FfiConverterStringINSTANCE.Write(writer, value.Txid)
	FfiConverterUint32INSTANCE.Write(writer, value.Vout)
}

type FfiDestroyerOutPoint struct{}

func (_ FfiDestroyerOutPoint) Destroy(value OutPoint) {
	value.Destroy()
}

type Vtxo struct {
	Point        OutPoint
	AmountSat    uint64
	UserPubkey   PublicKey
	AspPubkey    PublicKey
	ExpiryHeight uint32
	IsArkoor     bool
}

func (r *Vtxo) Destroy() {
	FfiDestroyerOutPoint{}.Destroy(r.Point)
	FfiDestroyerUint64{}.Destroy(r.AmountSat)
	FfiDestroyerTypePublicKey{}.Destroy(r.UserPubkey)
	FfiDestroyerTypePublicKey{}.Destroy(r.AspPubkey)
	FfiDestroyerUint32{}.Destroy(r.ExpiryHeight)
	FfiDestroyerBool{}.Destroy(r.IsArkoor)
}

type FfiConverterVtxo struct{}

var FfiConverterVtxoINSTANCE = FfiConverterVtxo{}

func (c FfiConverterVtxo) Lift(rb RustBufferI) Vtxo {
	return LiftFromRustBuffer[Vtxo](c, rb)
}

func (c FfiConverterVtxo) Read(reader io.Reader) Vtxo {
	return Vtxo{
		FfiConverterOutPointINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterTypePublicKeyINSTANCE.Read(reader),
		FfiConverterTypePublicKeyINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterBoolINSTANCE.Read(reader),
	}
}

func (c FfiConverterVtxo) Lower(value Vtxo) C.RustBuffer {
	return LowerIntoRustBuffer[Vtxo](c, value)
}

func (c FfiConverterVtxo) Write(writer io.Writer, value Vtxo) {
	FfiConverterOutPointINSTANCE.Write(writer, value.Point)
	FfiConverterUint64INSTANCE.Write(writer, value.AmountSat)
	FfiConverterTypePublicKeyINSTANCE.Write(writer, value.UserPubkey)
	FfiConverterTypePublicKeyINSTANCE.Write(writer, value.AspPubkey)
	FfiConverterUint32INSTANCE.Write(writer, value.ExpiryHeight)
	FfiConverterBoolINSTANCE.Write(writer, value.IsArkoor)
}

type FfiDestroyerVtxo struct{}

func (_ FfiDestroyerVtxo) Destroy(value Vtxo) {
	value.Destroy()
}

type WalletBalance struct {
	SpendableSat            uint64
	PendingLightningSendSat uint64
	PendingExitSat          uint64
}

func (r *WalletBalance) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.SpendableSat)
	FfiDestroyerUint64{}.Destroy(r.PendingLightningSendSat)
	FfiDestroyerUint64{}.Destroy(r.PendingExitSat)
}

type FfiConverterWalletBalance struct{}

var FfiConverterWalletBalanceINSTANCE = FfiConverterWalletBalance{}

func (c FfiConverterWalletBalance) Lift(rb RustBufferI) WalletBalance {
	return LiftFromRustBuffer[WalletBalance](c, rb)
}

func (c FfiConverterWalletBalance) Read(reader io.Reader) WalletBalance {
	return WalletBalance{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterWalletBalance) Lower(value WalletBalance) C.RustBuffer {
	return LowerIntoRustBuffer[WalletBalance](c, value)
}

func (c FfiConverterWalletBalance) Write(writer io.Writer, value WalletBalance) {
	FfiConverterUint64INSTANCE.Write(writer, value.SpendableSat)
	FfiConverterUint64INSTANCE.Write(writer, value.PendingLightningSendSat)
	FfiConverterUint64INSTANCE.Write(writer, value.PendingExitSat)
}

type FfiDestroyerWalletBalance struct{}

func (_ FfiDestroyerWalletBalance) Destroy(value WalletBalance) {
	value.Destroy()
}

type Error struct {
	err error
}

// Convience method to turn *Error into error
// Avoiding treating nil pointer as non nil error interface
func (err *Error) AsError() error {
	if err == nil {
		return nil
	} else {
		return err
	}
}

func (err Error) Error() string {
	return fmt.Sprintf("Error: %s", err.err.Error())
}

func (err Error) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrErrorBarkDbFileNotAccessible = fmt.Errorf("ErrorBarkDbFileNotAccessible")
var ErrErrorBarkDbFileAlreadyExists = fmt.Errorf("ErrorBarkDbFileAlreadyExists")
var ErrErrorInvalidNetwork = fmt.Errorf("ErrorInvalidNetwork")
var ErrErrorInvalidPublicKey = fmt.Errorf("ErrorInvalidPublicKey")
var ErrErrorInvalidMnemonic = fmt.Errorf("ErrorInvalidMnemonic")
var ErrErrorInvalidBolt11Invoice = fmt.Errorf("ErrorInvalidBolt11Invoice")
var ErrErrorInvalidBitcoinAddress = fmt.Errorf("ErrorInvalidBitcoinAddress")
var ErrErrorInvalidBarkAddress = fmt.Errorf("ErrorInvalidBarkAddress")
var ErrErrorBarkFailed = fmt.Errorf("ErrorBarkFailed")

// Variant structs
type ErrorBarkDbFileNotAccessible struct {
	message string
}

func NewErrorBarkDbFileNotAccessible() *Error {
	return &Error{err: &ErrorBarkDbFileNotAccessible{}}
}

func (e ErrorBarkDbFileNotAccessible) destroy() {
}

func (err ErrorBarkDbFileNotAccessible) Error() string {
	return fmt.Sprintf("BarkDbFileNotAccessible: %s", err.message)
}

func (self ErrorBarkDbFileNotAccessible) Is(target error) bool {
	return target == ErrErrorBarkDbFileNotAccessible
}

type ErrorBarkDbFileAlreadyExists struct {
	message string
}

func NewErrorBarkDbFileAlreadyExists() *Error {
	return &Error{err: &ErrorBarkDbFileAlreadyExists{}}
}

func (e ErrorBarkDbFileAlreadyExists) destroy() {
}

func (err ErrorBarkDbFileAlreadyExists) Error() string {
	return fmt.Sprintf("BarkDbFileAlreadyExists: %s", err.message)
}

func (self ErrorBarkDbFileAlreadyExists) Is(target error) bool {
	return target == ErrErrorBarkDbFileAlreadyExists
}

type ErrorInvalidNetwork struct {
	message string
}

func NewErrorInvalidNetwork() *Error {
	return &Error{err: &ErrorInvalidNetwork{}}
}

func (e ErrorInvalidNetwork) destroy() {
}

func (err ErrorInvalidNetwork) Error() string {
	return fmt.Sprintf("InvalidNetwork: %s", err.message)
}

func (self ErrorInvalidNetwork) Is(target error) bool {
	return target == ErrErrorInvalidNetwork
}

type ErrorInvalidPublicKey struct {
	message string
}

func NewErrorInvalidPublicKey() *Error {
	return &Error{err: &ErrorInvalidPublicKey{}}
}

func (e ErrorInvalidPublicKey) destroy() {
}

func (err ErrorInvalidPublicKey) Error() string {
	return fmt.Sprintf("InvalidPublicKey: %s", err.message)
}

func (self ErrorInvalidPublicKey) Is(target error) bool {
	return target == ErrErrorInvalidPublicKey
}

type ErrorInvalidMnemonic struct {
	message string
}

func NewErrorInvalidMnemonic() *Error {
	return &Error{err: &ErrorInvalidMnemonic{}}
}

func (e ErrorInvalidMnemonic) destroy() {
}

func (err ErrorInvalidMnemonic) Error() string {
	return fmt.Sprintf("InvalidMnemonic: %s", err.message)
}

func (self ErrorInvalidMnemonic) Is(target error) bool {
	return target == ErrErrorInvalidMnemonic
}

type ErrorInvalidBolt11Invoice struct {
	message string
}

func NewErrorInvalidBolt11Invoice() *Error {
	return &Error{err: &ErrorInvalidBolt11Invoice{}}
}

func (e ErrorInvalidBolt11Invoice) destroy() {
}

func (err ErrorInvalidBolt11Invoice) Error() string {
	return fmt.Sprintf("InvalidBolt11Invoice: %s", err.message)
}

func (self ErrorInvalidBolt11Invoice) Is(target error) bool {
	return target == ErrErrorInvalidBolt11Invoice
}

type ErrorInvalidBitcoinAddress struct {
	message string
}

func NewErrorInvalidBitcoinAddress() *Error {
	return &Error{err: &ErrorInvalidBitcoinAddress{}}
}

func (e ErrorInvalidBitcoinAddress) destroy() {
}

func (err ErrorInvalidBitcoinAddress) Error() string {
	return fmt.Sprintf("InvalidBitcoinAddress: %s", err.message)
}

func (self ErrorInvalidBitcoinAddress) Is(target error) bool {
	return target == ErrErrorInvalidBitcoinAddress
}

type ErrorInvalidBarkAddress struct {
	message string
}

func NewErrorInvalidBarkAddress() *Error {
	return &Error{err: &ErrorInvalidBarkAddress{}}
}

func (e ErrorInvalidBarkAddress) destroy() {
}

func (err ErrorInvalidBarkAddress) Error() string {
	return fmt.Sprintf("InvalidBarkAddress: %s", err.message)
}

func (self ErrorInvalidBarkAddress) Is(target error) bool {
	return target == ErrErrorInvalidBarkAddress
}

type ErrorBarkFailed struct {
	message string
}

func NewErrorBarkFailed() *Error {
	return &Error{err: &ErrorBarkFailed{}}
}

func (e ErrorBarkFailed) destroy() {
}

func (err ErrorBarkFailed) Error() string {
	return fmt.Sprintf("BarkFailed: %s", err.message)
}

func (self ErrorBarkFailed) Is(target error) bool {
	return target == ErrErrorBarkFailed
}

type FfiConverterError struct{}

var FfiConverterErrorINSTANCE = FfiConverterError{}

func (c FfiConverterError) Lift(eb RustBufferI) *Error {
	return LiftFromRustBuffer[*Error](c, eb)
}

func (c FfiConverterError) Lower(value *Error) C.RustBuffer {
	return LowerIntoRustBuffer[*Error](c, value)
}

func (c FfiConverterError) Read(reader io.Reader) *Error {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &Error{&ErrorBarkDbFileNotAccessible{message}}
	case 2:
		return &Error{&ErrorBarkDbFileAlreadyExists{message}}
	case 3:
		return &Error{&ErrorInvalidNetwork{message}}
	case 4:
		return &Error{&ErrorInvalidPublicKey{message}}
	case 5:
		return &Error{&ErrorInvalidMnemonic{message}}
	case 6:
		return &Error{&ErrorInvalidBolt11Invoice{message}}
	case 7:
		return &Error{&ErrorInvalidBitcoinAddress{message}}
	case 8:
		return &Error{&ErrorInvalidBarkAddress{message}}
	case 9:
		return &Error{&ErrorBarkFailed{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterError.Read()", errorID))
	}

}

func (c FfiConverterError) Write(writer io.Writer, value *Error) {
	switch variantValue := value.err.(type) {
	case *ErrorBarkDbFileNotAccessible:
		writeInt32(writer, 1)
	case *ErrorBarkDbFileAlreadyExists:
		writeInt32(writer, 2)
	case *ErrorInvalidNetwork:
		writeInt32(writer, 3)
	case *ErrorInvalidPublicKey:
		writeInt32(writer, 4)
	case *ErrorInvalidMnemonic:
		writeInt32(writer, 5)
	case *ErrorInvalidBolt11Invoice:
		writeInt32(writer, 6)
	case *ErrorInvalidBitcoinAddress:
		writeInt32(writer, 7)
	case *ErrorInvalidBarkAddress:
		writeInt32(writer, 8)
	case *ErrorBarkFailed:
		writeInt32(writer, 9)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterError.Write", value))
	}
}

type FfiDestroyerError struct{}

func (_ FfiDestroyerError) Destroy(value *Error) {
	switch variantValue := value.err.(type) {
	case ErrorBarkDbFileNotAccessible:
		variantValue.destroy()
	case ErrorBarkDbFileAlreadyExists:
		variantValue.destroy()
	case ErrorInvalidNetwork:
		variantValue.destroy()
	case ErrorInvalidPublicKey:
		variantValue.destroy()
	case ErrorInvalidMnemonic:
		variantValue.destroy()
	case ErrorInvalidBolt11Invoice:
		variantValue.destroy()
	case ErrorInvalidBitcoinAddress:
		variantValue.destroy()
	case ErrorInvalidBarkAddress:
		variantValue.destroy()
	case ErrorBarkFailed:
		variantValue.destroy()
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiDestroyerError.Destroy", value))
	}
}

type MovementKind uint

const (
	MovementKindBoard                   MovementKind = 1
	MovementKindRound                   MovementKind = 2
	MovementKindOffboard                MovementKind = 3
	MovementKindExit                    MovementKind = 4
	MovementKindArkoorSend              MovementKind = 5
	MovementKindArkoorReceive           MovementKind = 6
	MovementKindLightningSend           MovementKind = 7
	MovementKindLightningSendRevocation MovementKind = 8
	MovementKindLightningReceive        MovementKind = 9
)

type FfiConverterMovementKind struct{}

var FfiConverterMovementKindINSTANCE = FfiConverterMovementKind{}

func (c FfiConverterMovementKind) Lift(rb RustBufferI) MovementKind {
	return LiftFromRustBuffer[MovementKind](c, rb)
}

func (c FfiConverterMovementKind) Lower(value MovementKind) C.RustBuffer {
	return LowerIntoRustBuffer[MovementKind](c, value)
}
func (FfiConverterMovementKind) Read(reader io.Reader) MovementKind {
	id := readInt32(reader)
	return MovementKind(id)
}

func (FfiConverterMovementKind) Write(writer io.Writer, value MovementKind) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerMovementKind struct{}

func (_ FfiDestroyerMovementKind) Destroy(value MovementKind) {
}

type Utxo interface {
	Destroy()
}
type UtxoLocal struct {
	Outpoint           OutPoint
	AmountSat          uint64
	ConfirmationHeight *uint32
}

func (e UtxoLocal) Destroy() {
	FfiDestroyerOutPoint{}.Destroy(e.Outpoint)
	FfiDestroyerUint64{}.Destroy(e.AmountSat)
	FfiDestroyerOptionalUint32{}.Destroy(e.ConfirmationHeight)
}

type UtxoExit struct {
	Vtxo   Vtxo
	Height uint32
}

func (e UtxoExit) Destroy() {
	FfiDestroyerVtxo{}.Destroy(e.Vtxo)
	FfiDestroyerUint32{}.Destroy(e.Height)
}

type FfiConverterUtxo struct{}

var FfiConverterUtxoINSTANCE = FfiConverterUtxo{}

func (c FfiConverterUtxo) Lift(rb RustBufferI) Utxo {
	return LiftFromRustBuffer[Utxo](c, rb)
}

func (c FfiConverterUtxo) Lower(value Utxo) C.RustBuffer {
	return LowerIntoRustBuffer[Utxo](c, value)
}
func (FfiConverterUtxo) Read(reader io.Reader) Utxo {
	id := readInt32(reader)
	switch id {
	case 1:
		return UtxoLocal{
			FfiConverterOutPointINSTANCE.Read(reader),
			FfiConverterUint64INSTANCE.Read(reader),
			FfiConverterOptionalUint32INSTANCE.Read(reader),
		}
	case 2:
		return UtxoExit{
			FfiConverterVtxoINSTANCE.Read(reader),
			FfiConverterUint32INSTANCE.Read(reader),
		}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterUtxo.Read()", id))
	}
}

func (FfiConverterUtxo) Write(writer io.Writer, value Utxo) {
	switch variant_value := value.(type) {
	case UtxoLocal:
		writeInt32(writer, 1)
		FfiConverterOutPointINSTANCE.Write(writer, variant_value.Outpoint)
		FfiConverterUint64INSTANCE.Write(writer, variant_value.AmountSat)
		FfiConverterOptionalUint32INSTANCE.Write(writer, variant_value.ConfirmationHeight)
	case UtxoExit:
		writeInt32(writer, 2)
		FfiConverterVtxoINSTANCE.Write(writer, variant_value.Vtxo)
		FfiConverterUint32INSTANCE.Write(writer, variant_value.Height)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterUtxo.Write", value))
	}
}

type FfiDestroyerUtxo struct{}

func (_ FfiDestroyerUtxo) Destroy(value Utxo) {
	value.Destroy()
}

type FfiConverterOptionalUint32 struct{}

var FfiConverterOptionalUint32INSTANCE = FfiConverterOptionalUint32{}

func (c FfiConverterOptionalUint32) Lift(rb RustBufferI) *uint32 {
	return LiftFromRustBuffer[*uint32](c, rb)
}

func (_ FfiConverterOptionalUint32) Read(reader io.Reader) *uint32 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint32INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint32) Lower(value *uint32) C.RustBuffer {
	return LowerIntoRustBuffer[*uint32](c, value)
}

func (_ FfiConverterOptionalUint32) Write(writer io.Writer, value *uint32) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint32INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint32 struct{}

func (_ FfiDestroyerOptionalUint32) Destroy(value *uint32) {
	if value != nil {
		FfiDestroyerUint32{}.Destroy(*value)
	}
}

type FfiConverterOptionalUint64 struct{}

var FfiConverterOptionalUint64INSTANCE = FfiConverterOptionalUint64{}

func (c FfiConverterOptionalUint64) Lift(rb RustBufferI) *uint64 {
	return LiftFromRustBuffer[*uint64](c, rb)
}

func (_ FfiConverterOptionalUint64) Read(reader io.Reader) *uint64 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint64INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint64) Lower(value *uint64) C.RustBuffer {
	return LowerIntoRustBuffer[*uint64](c, value)
}

func (_ FfiConverterOptionalUint64) Write(writer io.Writer, value *uint64) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint64 struct{}

func (_ FfiDestroyerOptionalUint64) Destroy(value *uint64) {
	if value != nil {
		FfiDestroyerUint64{}.Destroy(*value)
	}
}

type FfiConverterSequenceMovement struct{}

var FfiConverterSequenceMovementINSTANCE = FfiConverterSequenceMovement{}

func (c FfiConverterSequenceMovement) Lift(rb RustBufferI) []Movement {
	return LiftFromRustBuffer[[]Movement](c, rb)
}

func (c FfiConverterSequenceMovement) Read(reader io.Reader) []Movement {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Movement, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterMovementINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceMovement) Lower(value []Movement) C.RustBuffer {
	return LowerIntoRustBuffer[[]Movement](c, value)
}

func (c FfiConverterSequenceMovement) Write(writer io.Writer, value []Movement) {
	if len(value) > math.MaxInt32 {
		panic("[]Movement is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterMovementINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceMovement struct{}

func (FfiDestroyerSequenceMovement) Destroy(sequence []Movement) {
	for _, value := range sequence {
		FfiDestroyerMovement{}.Destroy(value)
	}
}

type FfiConverterSequenceOnchainTransaction struct{}

var FfiConverterSequenceOnchainTransactionINSTANCE = FfiConverterSequenceOnchainTransaction{}

func (c FfiConverterSequenceOnchainTransaction) Lift(rb RustBufferI) []OnchainTransaction {
	return LiftFromRustBuffer[[]OnchainTransaction](c, rb)
}

func (c FfiConverterSequenceOnchainTransaction) Read(reader io.Reader) []OnchainTransaction {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]OnchainTransaction, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterOnchainTransactionINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceOnchainTransaction) Lower(value []OnchainTransaction) C.RustBuffer {
	return LowerIntoRustBuffer[[]OnchainTransaction](c, value)
}

func (c FfiConverterSequenceOnchainTransaction) Write(writer io.Writer, value []OnchainTransaction) {
	if len(value) > math.MaxInt32 {
		panic("[]OnchainTransaction is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterOnchainTransactionINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceOnchainTransaction struct{}

func (FfiDestroyerSequenceOnchainTransaction) Destroy(sequence []OnchainTransaction) {
	for _, value := range sequence {
		FfiDestroyerOnchainTransaction{}.Destroy(value)
	}
}

type FfiConverterSequenceVtxo struct{}

var FfiConverterSequenceVtxoINSTANCE = FfiConverterSequenceVtxo{}

func (c FfiConverterSequenceVtxo) Lift(rb RustBufferI) []Vtxo {
	return LiftFromRustBuffer[[]Vtxo](c, rb)
}

func (c FfiConverterSequenceVtxo) Read(reader io.Reader) []Vtxo {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Vtxo, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterVtxoINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceVtxo) Lower(value []Vtxo) C.RustBuffer {
	return LowerIntoRustBuffer[[]Vtxo](c, value)
}

func (c FfiConverterSequenceVtxo) Write(writer io.Writer, value []Vtxo) {
	if len(value) > math.MaxInt32 {
		panic("[]Vtxo is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterVtxoINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceVtxo struct{}

func (FfiDestroyerSequenceVtxo) Destroy(sequence []Vtxo) {
	for _, value := range sequence {
		FfiDestroyerVtxo{}.Destroy(value)
	}
}

type FfiConverterSequenceUtxo struct{}

var FfiConverterSequenceUtxoINSTANCE = FfiConverterSequenceUtxo{}

func (c FfiConverterSequenceUtxo) Lift(rb RustBufferI) []Utxo {
	return LiftFromRustBuffer[[]Utxo](c, rb)
}

func (c FfiConverterSequenceUtxo) Read(reader io.Reader) []Utxo {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Utxo, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterUtxoINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceUtxo) Lower(value []Utxo) C.RustBuffer {
	return LowerIntoRustBuffer[[]Utxo](c, value)
}

func (c FfiConverterSequenceUtxo) Write(writer io.Writer, value []Utxo) {
	if len(value) > math.MaxInt32 {
		panic("[]Utxo is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterUtxoINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceUtxo struct{}

func (FfiDestroyerSequenceUtxo) Destroy(sequence []Utxo) {
	for _, value := range sequence {
		FfiDestroyerUtxo{}.Destroy(value)
	}
}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type BarkAddress = string
type FfiConverterTypeBarkAddress = FfiConverterString
type FfiDestroyerTypeBarkAddress = FfiDestroyerString

var FfiConverterTypeBarkAddressINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type Bolt11Invoice = string
type FfiConverterTypeBolt11Invoice = FfiConverterString
type FfiDestroyerTypeBolt11Invoice = FfiDestroyerString

var FfiConverterTypeBolt11InvoiceINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type Network = string
type FfiConverterTypeNetwork = FfiConverterString
type FfiDestroyerTypeNetwork = FfiDestroyerString

var FfiConverterTypeNetworkINSTANCE = FfiConverterString{}

/**
 * Typealias from the type name used in the UDL file to the builtin type.  This
 * is needed because the UDL type name is used in function/method signatures.
 * It's also what we have an external type that references a custom type.
 */
type PublicKey = string
type FfiConverterTypePublicKey = FfiConverterString
type FfiDestroyerTypePublicKey = FfiDestroyerString

var FfiConverterTypePublicKeyINSTANCE = FfiConverterString{}

func CreateWallet(path string, mnemonic string, config Config) (*Wallet, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_bark_fn_func_create_wallet(FfiConverterStringINSTANCE.Lower(path), FfiConverterStringINSTANCE.Lower(mnemonic), FfiConverterConfigINSTANCE.Lower(config), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Wallet
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWalletINSTANCE.Lift(_uniffiRV), nil
	}
}

func OpenWallet(path string, mnemonic string) (*Wallet, error) {
	_uniffiRV, _uniffiErr := rustCallWithError[Error](FfiConverterError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_bark_fn_func_open_wallet(FfiConverterStringINSTANCE.Lower(path), FfiConverterStringINSTANCE.Lower(mnemonic), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Wallet
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterWalletINSTANCE.Lift(_uniffiRV), nil
	}
}
