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

type ErrorResponse struct {
	Error string `json:"error"`
}

type APIEndpoint[RequestType, ResponseType any] func(context.Context, *RequestType) (*ResponseType, error)

var (
	ErrorInvalidRequest = errors.New("invalid request")
	ErrorNotFound       = errors.New("no data found")
)

func ToHandlerFunc[RequestType, ResponseType any](endpoint APIEndpoint[RequestType, ResponseType]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		// deserialize the body
		requestData := new(RequestType)
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(requestData); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				writeErrorJSON(w, err)
				return
			}
			
			// if the request type has a Validatable method defined
			if req, isValidatable := (any(requestData)).(Validatable); isValidatable {
				if err := req.Validate(ctx); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					writeErrorJSON(w, err)
					return
				}
			}
		} else {
			requestData = nil
		}
		responseData, err := endpoint(ctx, requestData)
		if err != nil {
			// try to set a relevant HTTP status code
			if errors.Is(err, ErrorInvalidRequest) {
				w.WriteHeader(http.StatusBadRequest)
			} else if errors.Is(err, ErrorNotFound) {
				w.WriteHeader(http.StatusNotFound)
			}
			writeErrorJSON(w, err)
			return
		}

		json.NewEncoder(w).Encode(responseData)
	}
}

// Most RequestTypes are going to require some kind of validation, and it would be annoying to need to
// do something like err := request.Validate(); err != nil { return "invalid request..." } in every
// handler the RequestType is used in. This could be useful for Get/List requests, where the RequestType
// could be T/[]T, as well as Add/Update operations when some fields are required or immutable, for example.
type Validatable interface {
	Validate(context.Context) error
}

// shorthand function to reduce code verbosity; writes err.Error to a JSON object on the response
func writeErrorJSON(w http.ResponseWriter, err error) {
	json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
}
