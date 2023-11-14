package generichandler

// The goal of this package is to help programmers reason about "simple" HTTP endpoints like regular functions.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type APIEndpoint[RequestType, ResponseType any] func(context.Context, *RequestType) (*ResponseType, error)

var (
	ErrorInvalidRequest = errors.New("invalid request")
	ErrorNotFound       = errors.New("no data found")
)

// an HTTPDecoder is a function that transforms an HTTP request into a concrete type
type HTTPDecoder[RequestType any] func(*http.Request) (*RequestType, error)

// an HTTPEncoder is a function that transforms a
type HTTPEncoder[ResponseType any] func(http.ResponseWriter, *ResponseType) error

// DefaultHTTPDecoder unmarshals JSON from the request body
func DefaultHTTPDecoder[RequestType any](request *http.Request) (*RequestType, error) {
	// deserialize the body
	requestData := new(RequestType)
	if request.Body != nil {
		if err := json.NewDecoder(request.Body).Decode(requestData); err != nil {
			return nil, err
		}
	} else {
		// if the request didn't contain a body, return nil instead of an empty RequestType
		requestData = nil
	}

	return requestData, nil
}

func DefaultHTTPEncoder[ResponseType any](w http.ResponseWriter, data *ResponseType) error {
	if data != nil {
		return json.NewEncoder(w).Encode(data)
	}
	return nil
}

// ToHandlerFunc returns an http.HandlerFunc that attempts to unmarshal JSON from the request body and use it
// as an argument to the provided `endpoint` function, along with the request context. It then tries to handle
// any errors encountered (x)or marshal the output of the function as JSON to the response body.
// There isn't a ton of reason to export this function, and it won't be clear to anybody why this is a function
// with 3 function arguments that returns a function with a function argument that also returns a function - actually,
// there's almost certainly no reason to do it like that. Originally, I wanted to extend the previous `ToHandlerFunc`
// to allow the caller control over how the request is serialized/deserialized, but still provide a clean shorthand
// to wrap the common `APIEndpoint`s.
func ToHandlerFunc[RequestType, ResponseType any](
	decoder HTTPDecoder[RequestType],
	endpoint APIEndpoint[RequestType, ResponseType],
	encoder HTTPEncoder[ResponseType],
) func(APIEndpoint[RequestType, ResponseType]) http.HandlerFunc {

	return func(a APIEndpoint[RequestType, ResponseType]) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			// deserialize the body
			requestData := new(RequestType)
			if r.Body != nil {
				var err error
				requestData, err = decoder(r)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					writeErrorJSON(w, err)
				}

				// if the request type has a Validate method defined
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
				writeErrorJSON(w, err)
				return
			}

			if err := encoder(w, responseData); err != nil {
				writeErrorJSON(w, err)
				return
			}
		}
	}
}

// DefaultJSONHandlerFunc uses the default decoder and encoder to wrap the `endpoint` in an opaque `http.HandlerFunc` that somewhat resembles
// `encoder(handler(decoder()))`
func DefaultJSONHandlerFunc[RequestType, ResponseType any](endpoint APIEndpoint[RequestType, ResponseType]) http.HandlerFunc {
	return ToHandlerFunc(DefaultHTTPDecoder[RequestType], endpoint, DefaultHTTPEncoder[ResponseType])(endpoint)
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
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
}
