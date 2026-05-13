package utils

import "fmt"

type ErrCode int

const (
	ErrDockerClientInit ErrCode = iota + 1
	ErrDockerClientClose
	ErrContainerScan
	ErrContainerInspect
	ErrContainerNotFound
	ErrImageSave
	ErrImagePull
	ErrVolumeBackup
	ErrVolumeRestore
	ErrNetworkCreate
	ErrNetworkRemove
	ErrPackCreate
	ErrUnpackExtract
	ErrVerifyFailed
	ErrConfigMarshal
	ErrConfigUnmarshal
	ErrFileCreate
	ErrFileWrite
	ErrFileRead
	ErrDirCreate
	ErrInvalidInput
)

var errMessages = map[ErrCode]string{
	ErrDockerClientInit:    "failed to initialize Docker client",
	ErrDockerClientClose:   "failed to close Docker client",
	ErrContainerScan:       "failed to scan containers",
	ErrContainerInspect:   "failed to inspect container",
	ErrContainerNotFound:  "container not found",
	ErrImageSave:           "failed to save image",
	ErrImagePull:           "failed to pull image",
	ErrVolumeBackup:        "failed to backup volume",
	ErrVolumeRestore:       "failed to restore volume",
	ErrNetworkCreate:       "failed to create network",
	ErrNetworkRemove:       "failed to remove network",
	ErrPackCreate:         "failed to create migration package",
	ErrUnpackExtract:      "failed to extract migration package",
	ErrVerifyFailed:        "verification failed",
	ErrConfigMarshal:       "failed to marshal config",
	ErrConfigUnmarshal:    "failed to unmarshal config",
	ErrFileCreate:         "failed to create file",
	ErrFileWrite:          "failed to write file",
	ErrFileRead:           "failed to read file",
	ErrDirCreate:          "failed to create directory",
	ErrInvalidInput:       "invalid input",
}

type AppError struct {
	Code    ErrCode
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewError(code ErrCode, err error) *AppError {
	msg, ok := errMessages[code]
	if !ok {
		msg = "unknown error"
	}
	return &AppError{
		Code:    code,
		Message: msg,
		Err:     err,
	}
}

func (e *AppError) WithMessage(msg string) *AppError {
	e.Message = msg
	return e
}

func PrintErrMsg(code ErrCode, a ...any) {
	msg, ok := errMessages[code]
	if !ok {
		msg = "unknown error"
	}
	if len(a) > 0 {
		PrintE("%s: %v\n", msg, a[0])
	} else {
		PrintE("%s\n", msg)
	}
}
