package common

import (
	"errors"
)

type FilePath string

type Env string

const (
	Development Env = "development"
	Production  Env = "production"
	Local       Env = "local"
)

const VersionString = "0.1"

type DBHandler interface {
	Set(req SetRequest) error
	Add(req SetRequest) error
	Replace(req SetRequest) error
	Append(req SetRequest) error
	Prepend(req SetRequest) error
	Delete(req DeleteRequest) error
	Touch(req TouchRequest) error
	Get(req GetRequest) error
	GetE(req GetRequest) error
	Gat(req GATRequest) error
	Noop(req NoopRequest) error
	Quit(req QuitRequest) error
	Version(req VersionRequest) error
	Unknown(req Request) error
	Error(req Request, reqType RequestType, err error)
}

// Common metrics used across packages
var (
	ErrBadRequest = errors.New("CLIENT_ERROR bad request")
	ErrBadLength  = errors.New("CLIENT_ERROR length is not a valid integer")
	ErrBadFlags   = errors.New("CLIENT_ERROR flags is not a valid integer")
	ErrBadExptime = errors.New("CLIENT_ERROR exptime is not a valid integer")

	ErrNoError        = errors.New("Success")
	ErrKeyNotFound    = errors.New("ERROR Key not found")
	ErrKeyExists      = errors.New("ERROR Key already exists")
	ErrValueTooBig    = errors.New("ERROR Value too big")
	ErrInvalidArgs    = errors.New("ERROR Invalid arguments")
	ErrItemNotStored  = errors.New("ERROR Item not stored")
	ErrBadIncDecValue = errors.New("ERROR Bad increment/decrement value")
	ErrAuth           = errors.New("ERROR Authentication error")
	ErrUnknownCmd     = errors.New("ERROR Unknown command")
	ErrNoMem          = errors.New("ERROR Out of memory")
	ErrNotSupported   = errors.New("ERROR Not supported")
	ErrInternal       = errors.New("ERROR Internal error")
	ErrBusy           = errors.New("ERROR Busy")
	ErrTempFailure    = errors.New("ERROR Temporary error")

	ErrCollision = errors.New("ERROR Hash collision")
)

// IsAppError differentiates between protocol-defined errors that are relatively benign and other
// fatal errors like an IO error because of some socket problem or network issue.
// Make sure to keep this list in sync with the one above. It should contain all Err* that could
// come back from memcached itself
func IsAppError(err error) bool {
	return errors.Is(err, ErrKeyNotFound) ||
		errors.Is(err, ErrKeyExists) ||
		errors.Is(err, ErrValueTooBig) ||
		errors.Is(err, ErrInvalidArgs) ||
		errors.Is(err, ErrItemNotStored) ||
		errors.Is(err, ErrBadIncDecValue) ||
		errors.Is(err, ErrAuth) ||
		errors.Is(err, ErrUnknownCmd) ||
		errors.Is(err, ErrNoMem) ||
		errors.Is(err, ErrNotSupported) ||
		errors.Is(err, ErrInternal) ||
		errors.Is(err, ErrBusy) ||
		errors.Is(err, ErrTempFailure) ||
		errors.Is(err, ErrCollision)
}

func IsWrongRequest(err error) bool {
	return errors.Is(err, ErrBadRequest) ||
		errors.Is(err, ErrBadLength) ||
		errors.Is(err, ErrBadFlags) ||
		errors.Is(err, ErrBadExptime)
}

func IsMiss(err error) bool {
	return errors.Is(err, ErrKeyNotFound) || errors.Is(err, ErrKeyExists) || errors.Is(err, ErrItemNotStored)
}

// RequestType is the protocol-agnostic identifier for the command
type RequestType int

const (
	// RequestUnknown means the parser doesn't know what the request represents. Valid protocol
	// parsing but invalid values.
	RequestUnknown RequestType = iota

	// RequestGet represents both a single get and a multi-get, which take differen forms in
	// different protocols. This means it can also be the accumulation of many GETQ commands.
	RequestGet

	// RequestSet is to insert a new piece of data unconditionally. What that means is different
	// depending on L1 / L2 handling.
	RequestSet

	// RequestAdd will perform the same operations as set, but only if the key does not exist
	RequestAdd

	// RequestReplace will perform the same operations as set, but only if the key already exists
	RequestReplace

	// RequestAppend appends data to the end of the already existing data for a given key. Does not
	// change the flags or TTL values even if they are given.
	RequestAppend

	// RequestPrepend appends data to the end of the already existing data for a given key. Does not
	// change the flags or TTL values even if they are given.
	RequestPrepend

	// RequestDelete deletes a piece of data from all levels of db
	RequestDelete

	// RequestTouch updates the TTL for the item specified to a new TTL
	RequestTouch

	// RequestNoop does nothing
	RequestNoop

	// RequestQuit closes the connection
	RequestQuit

	// RequestVersion replies with a string designating the current software version
	RequestVersion
)

type Request interface {
	GetOpaque() uint32
	IsQuiet() bool
}

// SetRequest corresponds to common.RequestSet. It contains all the information required to fulfill
// a set request.
type SetRequest struct {
	Key     []byte
	Data    []byte
	Flags   uint32
	Exptime uint32
	Opaque  uint32
	Quiet   bool
}

func (r SetRequest) GetOpaque() uint32 {
	return r.Opaque
}

func (r SetRequest) IsQuiet() bool {
	return r.Quiet
}

// GetRequest corresponds to common.RequestGet. It contains all the information required to fulfill
// a get requestGets are batch by default, so single gets and batch gets are both represented by the
// same type
type GetRequest struct {
	Keys       [][]byte
	Opaques    []uint32
	Quiet      []bool
	NoopOpaque uint32
	NoopEnd    bool
}

func (r GetRequest) GetOpaque() uint32 {
	// TODO: better implementation?
	// It's nonsensical but the best way to react in this case since it's a bad situation already.
	// Typically if this method was needed we're already in a fatal error sitation.
	return 0
}

func (r GetRequest) IsQuiet() bool {
	return false
}

// DeleteRequest corresponds to common.RequestDelete. It contains all the information required to
// fulfill a delete request.
type DeleteRequest struct {
	Key    []byte
	Opaque uint32
	Quiet  bool
}

func (r DeleteRequest) GetOpaque() uint32 {
	return r.Opaque
}

func (r DeleteRequest) IsQuiet() bool {
	return r.Quiet
}

// TouchRequest corresponds to common.RequestTouch. It contains all the information required to
// fulfill a touch request.
type TouchRequest struct {
	Key     []byte
	Exptime uint32
	Opaque  uint32
	Quiet   bool
}

func (r TouchRequest) GetOpaque() uint32 {
	return r.Opaque
}

func (r TouchRequest) IsQuiet() bool {
	return r.Quiet
}

// GATRequest corresponds to common.RequestGat. It contains all the information required to fulfill
// a get-and-touch request.
type GATRequest struct {
	Key     []byte
	Exptime uint32
	Opaque  uint32
	Quiet   bool
}

func (r GATRequest) GetOpaque() uint32 {
	return r.Opaque
}

func (r GATRequest) IsQuiet() bool {
	return r.Quiet
}

// QuitRequest corresponds to common.RequestQuit. It contains all the information required to
// fulfill a quit request.
type QuitRequest struct {
	Opaque uint32
	Quiet  bool
}

func (r QuitRequest) GetOpaque() uint32 {
	return r.Opaque
}

func (r QuitRequest) IsQuiet() bool {
	return r.Quiet
}

// NoopRequest corresponds to common.RequestNoop. It contains all the information required to
// fulfill a version request.
type NoopRequest struct {
	Opaque uint32
}

func (r NoopRequest) GetOpaque() uint32 {
	return r.Opaque
}

func (r NoopRequest) IsQuiet() bool {
	return false
}

// VersionRequest corresponds to common.RequestVersion. It contains all the information required to
// fulfill a version request.
type VersionRequest struct {
	Opaque uint32
}

func (r VersionRequest) GetOpaque() uint32 {
	return r.Opaque
}

func (r VersionRequest) IsQuiet() bool {
	return false
}

// GetResponse is used in both RequestGet and RequestGat handling. Both respond in the same manner
// but with different opcodes. It is binary-protocol specific, but is still a part of the interface
// of responder to make the handling code more protocol-agnostic.
type GetResponse struct {
	Key    []byte
	Data   []byte
	Opaque uint32
	Flags  uint32
	Miss   bool
	Quiet  bool
}

// GetEResponse is used in the GetE protocol extension
type GetEResponse struct {
	Key     []byte
	Data    []byte
	Opaque  uint32
	Flags   uint32
	Exptime uint32
	Miss    bool
	Quiet   bool
}
