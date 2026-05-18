package update

import "errors"

// Sentinel errors returned by update operations.
var (
	ErrUnsafeArchivePath    = errors.New("unsafe archive entry path")
	ErrUnsupportedTarEntry  = errors.New("unsupported tar entry type")
	ErrNonHTTPS             = errors.New("refusing redirect to non-HTTPS URL")
	ErrTooManyRedirects     = errors.New("too many redirects")
	ErrDownloadExceeded     = errors.New("download exceeds size limit")
	ErrChecksumMismatch     = errors.New("checksum mismatch")
	ErrPayloadExceeded      = errors.New("payload exceeds size limit")
	ErrLockExists           = errors.New("lock file exists")
	ErrCheckingVersion      = errors.New("checking latest version")
	ErrInvalidVersion       = errors.New("invalid latest version")
	ErrDownloading          = errors.New("downloading")
	ErrDownloadingChecksums = errors.New("downloading checksums")
	ErrExtractingBinary     = errors.New("extracting binary")
	ErrOpeningBinary        = errors.New("opening new binary")
	ErrOpeningArchive       = errors.New("opening archive")
	ErrOpeningZip           = errors.New("opening zip")
	ErrComputingChecksum    = errors.New("computing checksum")
	ErrHTTPStatus           = errors.New("unexpected HTTP status")
)
