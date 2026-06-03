package test

import "time"

type Address struct {
	Street string
	City   string
	Zip    string
}

type Person struct {
	ID      int
	Name    string
	Age     int
	Email   *string
	Tags    []string
	Meta    map[string]string
	Address Address
	Score   float64

	// Extended types
	Active bool

	Int8  int8
	Int16 int16
	Int32 int32
	Int64 int64

	Uint    uint
	Uint8   uint8
	Uint16  uint16
	Uint32  uint32
	Uint64  uint64
	Uintptr uintptr

	Float32 float32

	Complex64  complex64
	Complex128 complex128

	Byte byte
	Rune rune

	PtrInt    *int
	PtrBool   *bool
	PtrFloat  *float64
	PtrStruct *Address

	IntSlice  []int
	ByteSlice []byte
	IntArray  [3]int

	MapIntString map[int]string
	MapStringPtr map[string]*string

	Timestamp time.Time
	PtrTime   *time.Time

	Anything any
}
