package test

import (
	"testing"
	"time"
)

func FuzzPersonDeep(f *testing.F) {
	f.Add(1, "John", 30, 95.5, true, int64(123456789), "tag1", "key1", "val1", []byte{1, 2, 3})
	f.Fuzz(
		func(t *testing.T, id int, name string, age int, score float64, active bool, unixNano int64, tag, key, val string, data []byte) {
			ts := time.Unix(0, unixNano)
			p1 := &Person{
				ID:        id,
				Name:      name,
				Age:       age,
				Score:     score,
				Active:    active,
				Timestamp: ts,
				Tags:      []string{tag},
				Meta:      map[string]string{key: val},
				ByteSlice: data,
			}
			p2 := &Person{
				ID:        id,
				Name:      name,
				Age:       age,
				Score:     score,
				Active:    active,
				Timestamp: ts,
				Tags:      []string{tag},
				Meta:      map[string]string{key: val},
				ByteSlice: data,
			}

			if !p1.DeepEqual(p2) {
				t.Error("Expected p1 and p2 to be equal")
			}
			if p1.DeepCompare(p2) != 0 {
				t.Error("Expected p1 and p2 to compare as equal")
			}

			// Self equality
			if !p1.DeepEqual(p1) {
				t.Error("Expected p1 to be equal to itself")
			}
			if p1.DeepCompare(p1) != 0 {
				t.Error("Expected p1 to compare as equal to itself")
			}

			// Mutate p2 and check inequality
			p2.ID++
			if p1.DeepEqual(p2) {
				t.Error("Expected p1 and p2 to be not equal after mutation")
			}
			if p1.DeepCompare(p2) == 0 {
				t.Error("Expected p1 and p2 to compare as not equal after mutation")
			}
		},
	)
}
