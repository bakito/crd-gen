//nolint:staticcheck // S1008 "Simplify returning boolean expression" is hard to manage in generated code.
package test

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// DeepEqual returns true if all exported fields of Address are deeply equal to the other.
func (in *Address) DeepEqual(other *Address) bool {
	if in == nil && other == nil {
		return true
	}
	if in == nil || other == nil {
		return false
	}
	if in.Street != other.Street {
		return false
	}
	if in.City != other.City {
		return false
	}
	if in.Zip != other.Zip {
		return false
	}
	return true
}

// DeepCompare compares Address field by field in declaration order.
// Returns -1 if in < other, 0 if equal, 1 if in > other.
func (in *Address) DeepCompare(other *Address) int {
	if in == nil && other == nil {
		return 0
	}
	if in == nil {
		return -1
	}
	if other == nil {
		return 1
	}
	if _c := strings.Compare(in.Street, other.Street); _c != 0 {
		return _c
	}
	if _c := strings.Compare(in.City, other.City); _c != 0 {
		return _c
	}
	if _c := strings.Compare(in.Zip, other.Zip); _c != 0 {
		return _c
	}
	return 0
}

// DeepEqual returns true if all exported fields of Person are deeply equal to the other.
func (in *Person) DeepEqual(other *Person) bool {
	if in == nil && other == nil {
		return true
	}
	if in == nil || other == nil {
		return false
	}
	if in.ID != other.ID {
		return false
	}
	if in.Name != other.Name {
		return false
	}
	if in.Age != other.Age {
		return false
	}
	switch {
	case in.Email == nil && other.Email == nil:
	// both nil: ok
	case in.Email != nil && other.Email != nil:
		_da1, _db1 := *in.Email, *other.Email
		if _da1 != _db1 {
			return false
		}
	default:
		return false
	}
	if len(in.Tags) != len(other.Tags) {
		return false
	}
	for i1 := range in.Tags {
		if in.Tags[i1] != other.Tags[i1] {
			return false
		}
	}
	if len(in.Meta) != len(other.Meta) {
		return false
	}
	for k1, v1 := range in.Meta {
		_bv1, _ok1 := other.Meta[k1]
		if !_ok1 {
			return false
		}
		if v1 != _bv1 {
			return false
		}
	}
	if !in.Address.DeepEqual(new(other.Address)) {
		return false
	}
	if in.Score != other.Score {
		return false
	}
	if in.Active != other.Active {
		return false
	}
	if in.Int8 != other.Int8 {
		return false
	}
	if in.Int16 != other.Int16 {
		return false
	}
	if in.Int32 != other.Int32 {
		return false
	}
	if in.Int64 != other.Int64 {
		return false
	}
	if in.Uint != other.Uint {
		return false
	}
	if in.Uint8 != other.Uint8 {
		return false
	}
	if in.Uint16 != other.Uint16 {
		return false
	}
	if in.Uint32 != other.Uint32 {
		return false
	}
	if in.Uint64 != other.Uint64 {
		return false
	}
	if in.Uintptr != other.Uintptr {
		return false
	}
	if in.Float32 != other.Float32 {
		return false
	}
	if in.Complex64 != other.Complex64 {
		return false
	}
	if in.Complex128 != other.Complex128 {
		return false
	}
	if in.Byte != other.Byte {
		return false
	}
	if in.Rune != other.Rune {
		return false
	}
	switch {
	case in.PtrInt == nil && other.PtrInt == nil:
	// both nil: ok
	case in.PtrInt != nil && other.PtrInt != nil:
		_da1, _db1 := *in.PtrInt, *other.PtrInt
		if _da1 != _db1 {
			return false
		}
	default:
		return false
	}
	switch {
	case in.PtrBool == nil && other.PtrBool == nil:
	// both nil: ok
	case in.PtrBool != nil && other.PtrBool != nil:
		_da1, _db1 := *in.PtrBool, *other.PtrBool
		if _da1 != _db1 {
			return false
		}
	default:
		return false
	}
	switch {
	case in.PtrFloat == nil && other.PtrFloat == nil:
	// both nil: ok
	case in.PtrFloat != nil && other.PtrFloat != nil:
		_da1, _db1 := *in.PtrFloat, *other.PtrFloat
		if _da1 != _db1 {
			return false
		}
	default:
		return false
	}
	switch {
	case in.PtrStruct == nil && other.PtrStruct == nil:
	// both nil: ok
	case in.PtrStruct != nil && other.PtrStruct != nil:
		_da1, _db1 := *in.PtrStruct, *other.PtrStruct
		if !_da1.DeepEqual(new(_db1)) {
			return false
		}
	default:
		return false
	}
	if len(in.IntSlice) != len(other.IntSlice) {
		return false
	}
	for i1 := range in.IntSlice {
		if in.IntSlice[i1] != other.IntSlice[i1] {
			return false
		}
	}
	if !bytes.Equal(in.ByteSlice, other.ByteSlice) {
		return false
	}
	for i1 := range in.IntArray {
		if in.IntArray[i1] != other.IntArray[i1] {
			return false
		}
	}
	if len(in.MapIntString) != len(other.MapIntString) {
		return false
	}
	for k1, v1 := range in.MapIntString {
		_bv1, _ok1 := other.MapIntString[k1]
		if !_ok1 {
			return false
		}
		if v1 != _bv1 {
			return false
		}
	}
	if len(in.MapStringPtr) != len(other.MapStringPtr) {
		return false
	}
	for k1, v1 := range in.MapStringPtr {
		_bv1, _ok1 := other.MapStringPtr[k1]
		if !_ok1 {
			return false
		}
		switch {
		case v1 == nil && _bv1 == nil:
		// both nil: ok
		case v1 != nil && _bv1 != nil:
			_da2, _db2 := *v1, *_bv1
			if _da2 != _db2 {
				return false
			}
		default:
			return false
		}
	}
	if !in.Timestamp.Equal(other.Timestamp) {
		return false
	}
	switch {
	case in.PtrTime == nil && other.PtrTime == nil:
	// both nil: ok
	case in.PtrTime != nil && other.PtrTime != nil:
		_da1, _db1 := *in.PtrTime, *other.PtrTime
		if !_da1.Equal(_db1) {
			return false
		}
	default:
		return false
	}
	if !reflect.DeepEqual(in.Anything, other.Anything) {
		return false
	}
	return true
}

// DeepCompare compares Person field by field in declaration order.
// Returns -1 if in < other, 0 if equal, 1 if in > other.
func (in *Person) DeepCompare(other *Person) int {
	if in == nil && other == nil {
		return 0
	}
	if in == nil {
		return -1
	}
	if other == nil {
		return 1
	}
	if in.ID < other.ID {
		return -1
	}
	if in.ID > other.ID {
		return 1
	}
	if _c := strings.Compare(in.Name, other.Name); _c != 0 {
		return _c
	}
	if in.Age < other.Age {
		return -1
	}
	if in.Age > other.Age {
		return 1
	}
	if in.Email == nil && other.Email != nil {
		return -1
	}
	if in.Email != nil && other.Email == nil {
		return 1
	}
	if in.Email != nil {
		_da1, _db1 := *in.Email, *other.Email
		if _c := strings.Compare(_da1, _db1); _c != 0 {
			return _c
		}
	}
	{
		_la, _lb := len(in.Tags), len(other.Tags)
		_min := min(_lb, _la)
		for _i1 := range _min {
			if _c := strings.Compare(in.Tags[_i1], other.Tags[_i1]); _c != 0 {
				return _c
			}
		}
		if _la < _lb {
			return -1
		}
		if _la > _lb {
			return 1
		}
	}
	{
		if len(in.Meta) != len(other.Meta) {
			if len(in.Meta) < len(other.Meta) {
				return -1
			}
			return 1
		}
		_keys := make([]string, 0, len(in.Meta))
		for _k := range in.Meta {
			_keys = append(_keys, _k)
		}
		slices.Sort(_keys)
		for _, _ks := range _keys {
			_ = _ks // key-order compare via sorted string keys
		}
		// detailed map compare: fall back to string representation
		_sa, _sb := fmt.Sprint(in.Meta), fmt.Sprint(other.Meta)
		if _c := strings.Compare(_sa, _sb); _c != 0 {
			return _c
		}
	}
	if _c := in.Address.DeepCompare(new(other.Address)); _c != 0 {
		return _c
	}
	if in.Score < other.Score {
		return -1
	}
	if in.Score > other.Score {
		return 1
	}
	if !in.Active && other.Active {
		return -1
	}
	if in.Active && !other.Active {
		return 1
	}
	if in.Int8 < other.Int8 {
		return -1
	}
	if in.Int8 > other.Int8 {
		return 1
	}
	if in.Int16 < other.Int16 {
		return -1
	}
	if in.Int16 > other.Int16 {
		return 1
	}
	if in.Int32 < other.Int32 {
		return -1
	}
	if in.Int32 > other.Int32 {
		return 1
	}
	if in.Int64 < other.Int64 {
		return -1
	}
	if in.Int64 > other.Int64 {
		return 1
	}
	if in.Uint < other.Uint {
		return -1
	}
	if in.Uint > other.Uint {
		return 1
	}
	if in.Uint8 < other.Uint8 {
		return -1
	}
	if in.Uint8 > other.Uint8 {
		return 1
	}
	if in.Uint16 < other.Uint16 {
		return -1
	}
	if in.Uint16 > other.Uint16 {
		return 1
	}
	if in.Uint32 < other.Uint32 {
		return -1
	}
	if in.Uint32 > other.Uint32 {
		return 1
	}
	if in.Uint64 < other.Uint64 {
		return -1
	}
	if in.Uint64 > other.Uint64 {
		return 1
	}
	if in.Uintptr < other.Uintptr {
		return -1
	}
	if in.Uintptr > other.Uintptr {
		return 1
	}
	if in.Float32 < other.Float32 {
		return -1
	}
	if in.Float32 > other.Float32 {
		return 1
	}
	{
		_sa, _sb := fmt.Sprint(in.Complex64), fmt.Sprint(other.Complex64)
		if _c := strings.Compare(_sa, _sb); _c != 0 {
			return _c
		}
	}
	{
		_sa, _sb := fmt.Sprint(in.Complex128), fmt.Sprint(other.Complex128)
		if _c := strings.Compare(_sa, _sb); _c != 0 {
			return _c
		}
	}
	if in.Byte < other.Byte {
		return -1
	}
	if in.Byte > other.Byte {
		return 1
	}
	if in.Rune < other.Rune {
		return -1
	}
	if in.Rune > other.Rune {
		return 1
	}
	if in.PtrInt == nil && other.PtrInt != nil {
		return -1
	}
	if in.PtrInt != nil && other.PtrInt == nil {
		return 1
	}
	if in.PtrInt != nil {
		_da1, _db1 := *in.PtrInt, *other.PtrInt
		if _da1 < _db1 {
			return -1
		}
		if _da1 > _db1 {
			return 1
		}
	}
	if in.PtrBool == nil && other.PtrBool != nil {
		return -1
	}
	if in.PtrBool != nil && other.PtrBool == nil {
		return 1
	}
	if in.PtrBool != nil {
		_da1, _db1 := *in.PtrBool, *other.PtrBool
		if !_da1 && _db1 {
			return -1
		}
		if _da1 && !_db1 {
			return 1
		}
	}
	if in.PtrFloat == nil && other.PtrFloat != nil {
		return -1
	}
	if in.PtrFloat != nil && other.PtrFloat == nil {
		return 1
	}
	if in.PtrFloat != nil {
		_da1, _db1 := *in.PtrFloat, *other.PtrFloat
		if _da1 < _db1 {
			return -1
		}
		if _da1 > _db1 {
			return 1
		}
	}
	if in.PtrStruct == nil && other.PtrStruct != nil {
		return -1
	}
	if in.PtrStruct != nil && other.PtrStruct == nil {
		return 1
	}
	if in.PtrStruct != nil {
		_da1, _db1 := *in.PtrStruct, *other.PtrStruct
		if _c := _da1.DeepCompare(new(_db1)); _c != 0 {
			return _c
		}
	}
	{
		_la, _lb := len(in.IntSlice), len(other.IntSlice)
		_min := min(_lb, _la)
		for _i1 := range _min {
			if in.IntSlice[_i1] < other.IntSlice[_i1] {
				return -1
			}
			if in.IntSlice[_i1] > other.IntSlice[_i1] {
				return 1
			}
		}
		if _la < _lb {
			return -1
		}
		if _la > _lb {
			return 1
		}
	}
	if _c := bytes.Compare(in.ByteSlice, other.ByteSlice); _c != 0 {
		return _c
	}
	for _i1 := range in.IntArray {
		if in.IntArray[_i1] < other.IntArray[_i1] {
			return -1
		}
		if in.IntArray[_i1] > other.IntArray[_i1] {
			return 1
		}
	}
	{
		if len(in.MapIntString) != len(other.MapIntString) {
			if len(in.MapIntString) < len(other.MapIntString) {
				return -1
			}
			return 1
		}
		_keys := make([]string, 0, len(in.MapIntString))
		for _k := range in.MapIntString {
			_keys = append(_keys, strconv.Itoa(_k))
		}
		slices.Sort(_keys)
		for _, _ks := range _keys {
			_ = _ks // key-order compare via sorted string keys
		}
		// detailed map compare: fall back to string representation
		_sa, _sb := fmt.Sprint(in.MapIntString), fmt.Sprint(other.MapIntString)
		if _c := strings.Compare(_sa, _sb); _c != 0 {
			return _c
		}
	}
	{
		if len(in.MapStringPtr) != len(other.MapStringPtr) {
			if len(in.MapStringPtr) < len(other.MapStringPtr) {
				return -1
			}
			return 1
		}
		_keys := make([]string, 0, len(in.MapStringPtr))
		for _k := range in.MapStringPtr {
			_keys = append(_keys, _k)
		}
		slices.Sort(_keys)
		for _, _ks := range _keys {
			_ = _ks // key-order compare via sorted string keys
		}
		// detailed map compare: fall back to string representation
		_sa, _sb := fmt.Sprint(in.MapStringPtr), fmt.Sprint(other.MapStringPtr)
		if _c := strings.Compare(_sa, _sb); _c != 0 {
			return _c
		}
	}
	if in.Timestamp.Before(other.Timestamp) {
		return -1
	}
	if in.Timestamp.After(other.Timestamp) {
		return 1
	}
	if in.PtrTime == nil && other.PtrTime != nil {
		return -1
	}
	if in.PtrTime != nil && other.PtrTime == nil {
		return 1
	}
	if in.PtrTime != nil {
		_da1, _db1 := *in.PtrTime, *other.PtrTime
		if _da1.Before(_db1) {
			return -1
		}
		if _da1.After(_db1) {
			return 1
		}
	}
	{
		_sa, _sb := fmt.Sprint(in.Anything), fmt.Sprint(other.Anything)
		if _c := strings.Compare(_sa, _sb); _c != 0 {
			return _c
		}
	}
	return 0
}
