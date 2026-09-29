package server

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
)

// ErrorResponse is the single error shape every failing request returns.
type ErrorResponse struct {
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// errorHandler renders errors as JSON and logs anything that is a 5xx.
//
// Note the v5 signature: (c *echo.Context, err error) — the arguments are
// swapped relative to v4. It also resolves the status with echo.StatusCode
// instead of asserting to *echo.HTTPError, because v5's sentinels such as
// echo.ErrNotFound are no longer of that type and a type assertion would turn
// every 404 into a 500.
func errorHandler(log zerolog.Logger, exposeInternal bool) echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		if err == nil {
			return
		}

		status := echo.StatusCode(err)
		if status == 0 {
			status = http.StatusInternalServerError
		}

		message := http.StatusText(status)
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) && httpErr.Message != "" {
			message = httpErr.Message
		} else if status >= http.StatusInternalServerError && exposeInternal {
			message = err.Error()
		}

		requestID := c.Response().Header().Get(echo.HeaderXRequestID)

		if status >= http.StatusInternalServerError {
			log.Error().
				Err(err).
				Str("request_id", requestID).
				Str("path", c.Request().URL.Path).
				Msg("request failed")
		}

		// The response may already be partially written (e.g. a panic mid-stream).
		if resp, uErr := echo.UnwrapResponse(c.Response()); uErr == nil && resp.Committed {
			return
		}

		body := ErrorResponse{Message: message, RequestID: requestID}

		var sendErr error
		if c.Request().Method == http.MethodHead {
			sendErr = c.NoContent(status)
		} else {
			sendErr = c.JSON(status, body)
		}
		if sendErr != nil {
			log.Error().Err(sendErr).Msg("failed to write error response")
		}
	}
}
