package middleware

import (
	webserver "github.com/mxmauro/go-webserver/v2"
)

// -----------------------------------------------------------------------------

// ConditionEvaluator defines a function that executes the wrapped middleware if it returns true.
type ConditionEvaluator func(req *webserver.RequestContext) (bool, error)

// -----------------------------------------------------------------------------

// NewConditional wraps a middleware to conditionally execute or skip it depending on the evaluator's return value.
func NewConditional(cond ConditionEvaluator, m webserver.HandlerFunc) webserver.HandlerFunc {
	// Set up the middleware function.
	return func(req *webserver.RequestContext) error {
		// Evaluate the condition.
		ok, err := cond(req)
		if err != nil {
			return err
		}
		if ok {
			return m(req)
		}

		// Go to the next middleware.
		return req.Next()
	}
}
