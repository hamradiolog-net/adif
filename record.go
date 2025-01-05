package adif

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/hamradiolog-net/adif-spec/src/pkg/adifield"
)

// Interface implementations
var (
	_ io.WriterTo   = &Record{}
	_ io.ReaderFrom = &Record{}
	_ fmt.Stringer  = &Record{}
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 1024)
		return &b
	},
}

// NewRecord creates a new Record with the default initial capacity.
func NewRecord() *Record {
	return NewRecordWithCapacity(-1)
}

// NewRecordWithCapacity creates a new Record with a specific initial capacity.
// If initialCapacity is negative, it will use a default capacity of 7.
func NewRecordWithCapacity(initialCapacity int) *Record {
	if initialCapacity < 0 {
		initialCapacity = 7
	}
	return &Record{
		Fields: make([]Field, 0, initialCapacity),
	}
}

// Reset clears the record of all fields.
func (r *Record) Reset() {
	r.Fields = r.Fields[:0]
}

// Get returns the value for a given field.
// If the field is empty, or does not exist, an empty string is returned.
func (r *Record) Get(field adifield.Field) string {
	// O(n) Linear search leverages CPU cache line prefetching and predictable memory access patterns.
	// The contiguous array layout ensures minimal cache misses compared to pointer chasing in map structures.
	// Tested to perform 10% - 30% faster than a map with field counts ranging from 10 - 50.
	for i := 0; i < len(r.Fields); i++ {
		if r.Fields[i].Name == field {
			return r.Fields[i].Data
		}
	}
	return ""
}

// Set updates a field value or adds a new field if it does not exist.
// It expects the field name to be in UPPERCASE.
func (r *Record) Set(field adifield.Field, value string) *Record {
	// ensure the strings are interned if reasonably possible.
	// This makes future lookups faster and reduces overall memory use.
	if fieldDef, ok := adifield.FieldMap[field]; ok {
		field = fieldDef.ID
	}
	return r.setNoIntern(field, value)
}

// setNoIntern is a low-level method that does not perform any string interning.
// It is used internally to avoid duplicating the interning that is automatically performed by the adi reader.
// It expects the field name to be in UPPERCASE.
func (r *Record) setNoIntern(field adifield.Field, value string) *Record {
	// O(n) Linear search leverages CPU cache line prefetching and predictable memory access patterns.
	// The contiguous array layout ensures minimal cache misses compared to pointer chasing in map structures.
	// While this (somewhat surprisingly) gives us performance gains event without string interning,
	// it is particularly effective due to our use of string interning both in the adi reader and in Set() above.
	// Tested to perform 10% - 30% faster than a map with field counts ranging from 10 - 50.

	for i := 0; i < len(r.Fields); i++ {
		if r.Fields[i].Name == field {
			r.Fields[i].Data = value
			return r
		}
	}

	// If the value is empty, we don't need to add the field
	if value == "" {
		return r
	}
	r.Fields = append(r.Fields, Field{Name: field, Data: value})
	return r
}

// ReadFrom reads an ADIF formatted record from the provided io.Reader.
// It returns the number of bytes read and any error encountered.
// io.EOF is returned when no more records are available.
// Existing fields will be updated and add new fields added when necessary.
// Header records are SKIPPED.
//
// n.b. This method is best used when reading a single record.
// When reading multiple records, create an ADIFReader using NewadiReader().
// Use its Next() method to obtain maximum speed and memory efficiency.
func (r *Record) ReadFrom(src io.Reader) (int64, error) {
	p := NewADIReader(src, true)
	var n int64

	record, _, c, err := p.Next()
	n += c
	if err != nil {
		return n, err
	}

	r.Fields = record.Fields
	return n, nil
}

// WriteTo writes ADI formatted record data to the provided io.Writer.
// It returns the number of bytes written and any error encountered.
func (r *Record) WriteTo(dest io.Writer) (int64, error) {
	adiLength := r.appendAsADIPreCalculate()
	bufPtr := bufferPool.Get().(*[]byte)
	buf := *bufPtr

	if cap(buf) < adiLength {
		buf = make([]byte, 0, adiLength)
		*bufPtr = buf
	}
	buf = buf[:0]

	buf = r.appendAsADI(buf)
	n, err := dest.Write(buf)
	bufferPool.Put(bufPtr)
	return int64(n), err
}

// appendAsADI writes the ADI formatted QSO record to the provided buffer.
// The buffer should have sufficient capacity to avoid reallocations.
// You should use appendAsADIPreCalculate() to determine the required capacity
func (r *Record) appendAsADI(buf []byte) []byte {
	if len(r.Fields) == 0 {
		return buf
	}

	for i := 0; i < len(r.Fields); i++ {
		if len(r.Fields[i].Data) == 0 {
			continue
		}

		buf = append(buf, '<')
		buf = append(buf, r.Fields[i].Name...)
		buf = append(buf, ':')
		buf = strconv.AppendInt(buf, int64(len(r.Fields[i].Data)), 10)
		buf = append(buf, '>')
		buf = append(buf, []byte(r.Fields[i].Data)...)
	}

	return buf
}

// appendAsADIPreCalculate returns:
// 1) the length of the record in bytes when exported to ADI format by the AppendAsADI method.
// 2) a boolean indicating if the record is a header record.
func (r *Record) appendAsADIPreCalculate() (adiLength int) {
	if len(r.Fields) == 0 {
		return 0
	}

	for i := 0; i < len(r.Fields); i++ {
		valueLength := len(r.Fields[i].Data)
		if valueLength == 0 {
			continue
		}
		adiLength += 3 + valueLength + len(r.Fields[i].Name) // 3 for '<', ':', '>'

		// Avoid strconv.Itoa string allocation by calculating number of base 10 digits mathematically
		switch {
		case valueLength < 10:
			adiLength += 1
		case valueLength < 100:
			adiLength += 2
		case valueLength < 1_000:
			adiLength += 3
		default:
			adiLength += int(math.Log10(float64(valueLength))) + 1
		}
	}

	return adiLength
}

// Clean
// 1) trims whitespace in the field values
func (r *Record) Clean() {
	for i := 0; i < len(r.Fields); i++ {
		trimmed := strings.TrimSpace(r.Fields[i].Data)
		r.Fields[i].Data = trimmed
	}
}

// String returns the ADIF record as a string
func (r *Record) String() string {
	adiLength := r.appendAsADIPreCalculate()
	bufPtr := bufferPool.Get().(*[]byte)
	buf := *bufPtr

	if cap(buf) < adiLength {
		buf = make([]byte, 0, adiLength)
		*bufPtr = buf
	}
	buf = buf[:0]

	buf = r.appendAsADI(buf)
	s := string(buf)

	bufferPool.Put(bufPtr)
	return s
}
