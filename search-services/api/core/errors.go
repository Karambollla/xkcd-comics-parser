package core

import "errors"

var ErrBadArguments = errors.New("arguments are not acceptable")
var ErrAlreadyExists = errors.New("resource or task already exists")
var ErrNotFound = errors.New("resource is not found")
var ErrInvalidToken = errors.New("invalid token")
var ErrUnknownRole = errors.New("unknown role in token")
var ErrTokenExpired = errors.New("token expired")
var ErrInvalidCredentials = errors.New("invalid credentials")
