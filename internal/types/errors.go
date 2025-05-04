package types

import "errors"

// Domain specific errors for authentication and authorization.
var (
	ErrNotFound        = errors.New("requested item not found")
	ErrConflict        = errors.New("item already exists or conflict")
	ErrUnauthenticated = errors.New("authentication required or invalid credentials")
	ErrForbidden       = errors.New("action forbidden")
	ErrBadRequest      = errors.New("bad request")
)
