package mcp

import "errors"

func IsRPCError(err error) bool {
	var e *RPCError
	return errors.As(err, &e)
}

func IsHTTPStatusError(err error) bool {
	var e *HTTPStatusError
	return errors.As(err, &e)
}

func IsInitError(err error) bool {
	var e *ClientError
	if errors.As(err, &e) {
		return e.Op == "initialize"
	}
	return false
}

func IsCallToolError(err error) bool {
	var e *CallToolError
	if errors.As(err, &e) {
		return true
	}
	var ce *ClientError
	return errors.As(err, &ce) && ce.Method == "tools/call"
}

func IsAuthError(err error) bool {
	var e *HTTPStatusError
	return errors.As(err, &e) && (e.StatusCode == 401 || e.StatusCode == 403)
}

func IsRateLimited(err error) bool {
	var e *HTTPStatusError
	return errors.As(err, &e) && e.StatusCode == 429
}

func IsServerError(err error) bool {
	var e *HTTPStatusError
	return errors.As(err, &e) && e.StatusCode >= 500 && e.StatusCode <= 599
}
