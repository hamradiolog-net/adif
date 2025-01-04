// package adif provides
// 1) Types, Structs and Methods for managing ADIF QSOs.
// 2) ADIF parser for ADI formatted data.
// 3) Export ADI formatted data.
package adif

import (
	"fmt"

	"github.com/hamradiolog-net/adif-spec/src/pkg/adifield"
)

// The version of this library.
// See https://semver.org/
//
//	MAJOR version      == Incompatible API changes.
//	MINOR version      == Added functionality in a backward compatible manner
//	PATCH version      == Backward compatible bug fixes.
//	PRERELEASE version == Pre-release version (optional). This should be empty ("") or start with a dash (e.g. "-rc1").
//
// Additional labels for pre-release and build metadata are available as extensions to the MAJOR.MINOR.PATCH format.
const (
	versionMajor      = 0
	versionMinor      = 0
	versionPatch      = 1
	versionPreRelease = "-alpha"
)

const (
	tagEOH = string("<" + adifield.EOH + ">") // TagEOH is the end of header ADIF tag: <EOH>
	tagEOR = string("<" + adifield.EOR + ">") // TagEOR is the end of record ADIF tag: <EOR>
)

// AdifHeaderPreamble is always printed immediately before the header record.
//
// The ADIF specification states:
// "A Header begins with any character other than < and terminates with a case-insensitive End-Of-Header tag:"
//
// You may set your own custom ADI header preamble by changing this variable.
// If you provide your own preamble, you are responsible for ensuring it fulfils the ADIF specification.
var AdifHeaderPreamble = fmt.Sprintf("K9CTS AM✠DG ADIF Library v%d.%d.%d%s / go\n", versionMajor, versionMinor, versionPatch, versionPreRelease)

// DocumentMaxSizeInBytes controls the maximum size of data read into an Document struct in bytes.
// This variable helps prevent memory exhaustion attacks.
// You may adjust this value to suit your needs.
//
// For large documents, consider using the ADI parser to stream the records.
// The default limit is 256MB.
var DocumentMaxSizeInBytes int64 = 1024 * 1024 * 256
