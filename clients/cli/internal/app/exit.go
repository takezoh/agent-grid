package app

import "fmt"

// exit.go is the one place a failure class becomes a process exit code
// (adr-20260814-cli-failure-class-exit-codes). Every command returns a class;
// nothing else in the module names a numeric code, so automation can branch on
// the outcome and two classes can never collapse onto one number by accident.

// FailureClass is the closed set of ways an invocation can end. It is a
// classification of the *outcome*, not of the error that produced it: two
// unrelated internal faults share a class, and one transport error can be a
// transport failure here and an authentication failure there depending on what
// the server said.
type FailureClass int

const (
	// ClassSuccess is the only class that exits zero.
	ClassSuccess FailureClass = iota
	// ClassInternal is a fault in this program. The user cannot act on it.
	ClassInternal
	// ClassUsage is an invocation this program will not act on: an unknown
	// verb, a missing required flag, a selector naming only unusable entries.
	ClassUsage
	// ClassNotFound is a well-formed request for something that is not there,
	// on servers that were all readable.
	ClassNotFound
	// ClassAmbiguous is a selector matching more than one usable candidate.
	// Nothing is selected.
	ClassAmbiguous
	// ClassAuth is a rejected credential or an incompatible protocol version.
	// Retrying unchanged never succeeds.
	ClassAuth
	// ClassTransport is a connection that could not be established, was
	// refused for capacity, or was lost. Retrying later can succeed.
	ClassTransport
)

// exitCodes is the fixed assignment. It is a table rather than a switch so the
// test can assert it value by value and so a new class cannot silently reuse a
// number: a class absent from this table has no code at all.
var exitCodes = map[FailureClass]int{
	ClassSuccess:   0,
	ClassInternal:  1,
	ClassUsage:     2,
	ClassNotFound:  3,
	ClassAmbiguous: 4,
	ClassAuth:      5,
	ClassTransport: 6,
}

// ExitCode returns the process exit code for a class. An unknown class is an
// internal fault by construction — the program reached a state it does not
// define — and is reported as one rather than as success.
func ExitCode(class FailureClass) int {
	code, ok := exitCodes[class]
	if !ok {
		return exitCodes[ClassInternal]
	}
	return code
}

// String names the class for diagnostics. It is not part of any wire contract.
func (class FailureClass) String() string {
	switch class {
	case ClassSuccess:
		return "success"
	case ClassInternal:
		return "internal"
	case ClassUsage:
		return "usage"
	case ClassNotFound:
		return "not-found"
	case ClassAmbiguous:
		return "ambiguous"
	case ClassAuth:
		return "auth"
	case ClassTransport:
		return "transport"
	default:
		return "unknown"
	}
}

// Failure is what a command returns when it did not succeed. Carrying the class
// on the error is what keeps the mapping in one place: a command decides what
// kind of failure happened and never decides what number that is.
type Failure struct {
	Class  FailureClass
	Reason string
}

func (failure *Failure) Error() string { return failure.Reason }

// failf builds a Failure. The reason is presented to the user verbatim, so call
// sites are responsible for keeping credentials out of it — see redaction in
// the gateway package, which is where transport errors become reasons.
func failf(class FailureClass, format string, args ...any) *Failure {
	return &Failure{Class: class, Reason: fmt.Sprintf(format, args...)}
}
