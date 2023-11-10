package generichandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// The goal of this package is to help programmers reason about "simple" HTTP endpoints like regular functions.

// when we get a request, the server needs to determine whether it should have a body, how to create a request
// type for the endpoint, and what error code if any it should respond with. To do this, the plan is to
// - try to unmarshal the body, if there is one
// - wrap EndpointRequest[T] in a HandlerFunc
// - check the error returned by EndpointRequest

// Most RequestTypes are going to require some kind of validation, and it would be annoying to need to
// do something like err := request.Validate(); err != nil { return "invalid request..." } in every
// handler the RequestType is used in. This could be useful for Get/List requests, where the RequestType
// could be T/[]T, as well as Add/Update operations when some fields are required or immutable, for example.
type Validatable interface {
	Validate(context.Context) error
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type APIEndpoint[RequestType any] func(context.Context, *RequestType) (*RequestType, error)

var (
	ErrorInvalidRequest = errors.New("invalid request")
	ErrorNotFound       = errors.New("no data found")
)

func ToHandlerFunc[RequestType any](endpoint APIEndpoint[RequestType]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// deserialize the body
		requestData := new(RequestType)
		if err := json.NewDecoder(r.Body).Decode(requestData); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		}

		ctx := r.Context()
		responseData, err := endpoint(ctx, requestData)
		if err != nil {
			// try to set a relevant HTTP status code
			if errors.Is(err, ErrorInvalidRequest) {
				w.WriteHeader(http.StatusBadRequest)
			} else if errors.Is(err, ErrorNotFound) {
				w.WriteHeader(http.StatusNotFound)
			}
			json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
			return
		}

		json.NewEncoder(w).Encode(responseData)
	}
}
