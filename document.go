package adif

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

var (
	_ io.WriterTo   = &Document{}
	_ io.ReaderFrom = &Document{}
	_ fmt.Stringer  = &Document{}
)

// NewDocument creates a new Document with default initial capacity.
// If preamble is empty, we will use the default header preamble.
func NewDocument(headerPreamble string) *Document {
	return &Document{
		Records:        make([]Record, 0, 16),
		headerPreamble: headerPreamble,
	}
}

// Reset clears the document and prepares it for reuse.
func (f *Document) Reset() {
	f.Header = nil
	f.Records = f.Records[:0]
}

// WriteTo writes the document in ADI format to the given writer.
func (f *Document) WriteTo(w io.Writer) (n int64, err error) {
	bw, ok := w.(*bufio.Writer)
	if !ok {
		bw = bufio.NewWriter(w)
	}

	if f.Header != nil {
		var err error

		if len(f.headerPreamble) == 0 {
			f.headerPreamble = fmt.Sprintf("K9CTS AM✠DG ADIF Library v%d.%d.%d%s / go\n",
				versionMajor,
				versionMinor,
				versionPatch,
				versionPreRelease)
		}

		var ci int
		ci, err = bw.WriteString(f.headerPreamble)
		n += int64(ci)
		if err != nil {
			return handleFlush(bw, n, err)
		}

		var c64 int64
		c64, err = f.Header.WriteTo(bw)
		n += c64
		if err != nil {
			return handleFlush(bw, n, err)
		}

		ci, err = bw.WriteString("<EOH>\n")
		n += int64(ci)
		if err != nil {
			return handleFlush(bw, n, err)
		}
	}

	for _, record := range f.Records {
		c, err := record.WriteTo(bw)
		n += c
		if err != nil {
			return handleFlush(bw, n, err)
		}

		cc, err := bw.WriteString("<EOR>\n")
		n += int64(cc)
		if err != nil {
			return handleFlush(bw, n, err)
		}
	}

	return handleFlush(bw, n, err)
}

func handleFlush(bw *bufio.Writer, n int64, err error) (int64, error) {
	if bwerr := bw.Flush(); bwerr != nil {
		return n, fmt.Errorf("error flushing writer: %w", bwerr)
	}
	return n, err
}

// ReadFrom reads an ADI document from the given reader.
// You should call Reset() if you do not want add/update the existing document.
func (f *Document) ReadFrom(r io.Reader) (n int64, err error) {
	p := NewADIReader(r, false)

	firstRecord, isHeader, n, err := p.Next()
	if err != nil {
		return n, err
	}

	if isHeader {
		f.Header = firstRecord
	} else {
		f.Records = append(f.Records, *firstRecord)
	}

	for {
		record, _, c, err := p.Next()
		n += c
		if err != nil {
			if err == io.EOF {
				break
			}
			return n, err
		}
		f.Records = append(f.Records, *record)

		// prevent memory exhaustion attacks
		if n > DocumentMaxSizeInBytes {
			return n, ErrDocumentTooLarge
		}
	}

	return n, nil
}

// String returns the document as an ADI string.
// Returns an empty string if the receiver is nil.
func (f *Document) String() string {
	if f == nil || (len(f.Records) == 0 && f.Header == nil) {
		return ""
	}

	sb := strings.Builder{}

	_, err := f.WriteTo(&sb)
	if err != nil {
		return fmt.Sprintf("error while building adi string: %v", err)
	}

	return sb.String()
}
