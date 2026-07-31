package apperror

import "errors"

var ErrUnauthenticated = errors.New("authentication required")
var ErrPermissionDenied = errors.New("permission denied")
