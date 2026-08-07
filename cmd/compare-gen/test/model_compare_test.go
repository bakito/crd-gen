package test

import (
	"bytes"
	"testing"
	"time"
)

func TestAddress(t *testing.T) {
	tests := []struct {
		name    string
		in      *Address
		other   *Address
		equal   bool
		compare int
	}{
		{
			name:    "both nil",
			in:      nil,
			other:   nil,
			equal:   true,
			compare: 0,
		},
		{
			name:    "in nil",
			in:      nil,
			other:   &Address{Street: "A"},
			equal:   false,
			compare: -1,
		},
		{
			name:    "other nil",
			in:      &Address{Street: "A"},
			other:   nil,
			equal:   false,
			compare: 1,
		},
		{
			name:    "equal",
			in:      &Address{Street: "A", City: "B", Zip: "C"},
			other:   &Address{Street: "A", City: "B", Zip: "C"},
			equal:   true,
			compare: 0,
		},
		{
			name:    "diff street (less)",
			in:      &Address{Street: "A"},
			other:   &Address{Street: "B"},
			equal:   false,
			compare: -1,
		},
		{
			name:    "diff street (greater)",
			in:      &Address{Street: "B"},
			other:   &Address{Street: "A"},
			equal:   false,
			compare: 1,
		},
		{
			name:    "diff city",
			in:      &Address{Street: "A", City: "A"},
			other:   &Address{Street: "A", City: "B"},
			equal:   false,
			compare: -1,
		},
		{
			name:    "diff zip",
			in:      &Address{Street: "A", City: "A", Zip: "A"},
			other:   &Address{Street: "A", City: "A", Zip: "B"},
			equal:   false,
			compare: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.DeepEqual(tt.other); got != tt.equal {
				t.Errorf("DeepEqual() = %v, want %v", got, tt.equal)
			}
			if got := tt.in.DeepCompare(tt.other); got != tt.compare {
				t.Errorf("DeepCompare() = %v, want %v", got, tt.compare)
			}
		})
	}
}

func TestPerson(t *testing.T) {
	now := time.Now()
	email1 := "a@b.com"
	email2 := "a@c.com"

	base := Person{
		ID:           10,
		Name:         "John",
		Age:          30,
		Email:        &email1,
		Tags:         []string{"t1", "t2"},
		Meta:         map[string]string{"k1": "v1"},
		Address:      Address{Street: "Main"},
		Score:        10.5,
		Active:       true,
		Int8:         8,
		Int16:        16,
		Int32:        32,
		Int64:        64,
		Uint:         1,
		Uint8:        8,
		Uint16:       16,
		Uint32:       32,
		Uint64:       64,
		Uintptr:      123,
		Float32:      32.0,
		Complex64:    complex(1, 1),
		Complex128:   complex(2, 2),
		Byte:         1,
		Rune:         'a',
		PtrInt:       new(100),
		PtrBool:      new(true),
		PtrFloat:     new(1.1),
		PtrStruct:    &Address{Street: "Sub"},
		IntSlice:     []int{1, 2},
		ByteSlice:    []byte{3, 4},
		IntArray:     [3]int{5, 6, 7},
		MapIntString: map[int]string{1: "one"},
		MapStringPtr: map[string]*string{"k": &email1},
		Timestamp:    now,
		PtrTime:      &now,
		Anything:     "val",
	}

	clone := func(p Person) *Person {
		res := p
		res.Tags = append([]string(nil), p.Tags...)
		res.Meta = make(map[string]string)
		for k, v := range p.Meta {
			res.Meta[k] = v
		}
		res.IntSlice = append([]int(nil), p.IntSlice...)
		res.ByteSlice = bytes.Clone(p.ByteSlice)
		res.MapIntString = make(map[int]string)
		for k, v := range p.MapIntString {
			res.MapIntString[k] = v
		}
		res.MapStringPtr = make(map[string]*string)
		for k, v := range p.MapStringPtr {
			res.MapStringPtr[k] = v
		}
		return &res
	}

	tests := []struct {
		name    string
		in      *Person
		other   *Person
		equal   bool
		compare int
	}{
		{
			name:    "equal",
			in:      clone(base),
			other:   clone(base),
			equal:   true,
			compare: 0,
		},
		{
			name:    "ID diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.ID = 11; return p }(),
			equal:   false,
			compare: -1,
		},
		{
			name:    "Name diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.Name = "A"; return p }(),
			equal:   false,
			compare: 1, // "John" > "A"
		},
		{
			name:    "Age diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.Age = 31; return p }(),
			equal:   false,
			compare: -1,
		},
		{
			name:    "Email nil (in)",
			in:      func() *Person { p := clone(base); p.Email = nil; return p }(),
			other:   clone(base),
			equal:   false,
			compare: -1,
		},
		{
			name:    "Email diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.Email = &email2; return p }(),
			equal:   false,
			compare: -1, // "a@b.com" < "a@c.com"
		},
		{
			name:    "Tags len diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.Tags = []string{"t1"}; return p }(),
			equal:   false,
			compare: 1, // longer > shorter
		},
		{
			name:    "Tags content diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.Tags[1] = "t0"; return p }(),
			equal:   false,
			compare: 1, // "t2" > "t0"
		},
		{
			name:    "Meta diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.Meta["k1"] = "v2"; return p }(),
			equal:   false,
			compare: -1, // fmt.Sprint(meta) diff
		},
		{
			name:    "Address diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.Address.Street = "Other"; return p }(),
			equal:   false,
			compare: -1, // "Main" < "Other"
		},
		{
			name:    "Score diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.Score = 11.0; return p }(),
			equal:   false,
			compare: -1,
		},
		{
			name:    "Active diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.Active = false; return p }(),
			equal:   false,
			compare: 1, // true > false
		},
		{
			name:    "Complex diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.Complex64 = complex(2, 2); return p }(),
			equal:   false,
			compare: -1, // "(1+1i)" < "(2+2i)"
		},
		{
			name:    "PtrInt diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.PtrInt = new(101); return p }(),
			equal:   false,
			compare: -1,
		},
		{
			name:    "PtrStruct nil (other)",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.PtrStruct = nil; return p }(),
			equal:   false,
			compare: 1,
		},
		{
			name:    "IntSlice diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.IntSlice[0] = 0; return p }(),
			equal:   false,
			compare: 1,
		},
		{
			name:    "ByteSlice diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.ByteSlice[0] = 0; return p }(),
			equal:   false,
			compare: 1,
		},
		{
			name:    "IntArray diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.IntArray[0] = 0; return p }(),
			equal:   false,
			compare: 1,
		},
		{
			name:    "Timestamp diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.Timestamp = now.Add(time.Second); return p }(),
			equal:   false,
			compare: -1,
		},
		{
			name:    "Anything diff",
			in:      clone(base),
			other:   func() *Person { p := clone(base); p.Anything = "other"; return p }(),
			equal:   false,
			compare: 1, // "val" > "other"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.DeepEqual(tt.other); got != tt.equal {
				t.Errorf("DeepEqual() = %v, want %v", got, tt.equal)
			}
			if got := tt.in.DeepCompare(tt.other); got != tt.compare {
				t.Errorf("DeepCompare() = %v, want %v", got, tt.compare)
			}
		})
	}
}
