package core

import "errors"

var ErrBadArguments = errors.New("arguments are not acceptable")
var ErrAlreadyExists = errors.New("resource or task already exists")
var ErrNotFound = errors.New("resource is not found")
var ErrDropUpdate = errors.New("cant drop during update")
var ErrFailedPublish = errors.New("failed to publish event")
