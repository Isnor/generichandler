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

// APIEndpoint represents the function that is called when an endpoint is invoked
type APIEndpoint[RequestType, ResponseType any] func(context.Context, *RequestType) (*ResponseType, error)

var (
	ErrorInvalidRequest = errors.New("invalid request")
	ErrorNotFound       = errors.New("no data found")
)

// an HTTPDecoder is a function that transforms an HTTP request into a concrete type
type HTTPDecoder[RequestType any] func(*http.Request) (*RequestType, error)

// an HTTPEncoder is a function that writes data to the HTTP response
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

// DefaultHTTPEncoder writes data as JSON to an HTTP response
func DefaultHTTPEncoder[ResponseType any](w http.ResponseWriter, data *ResponseType) error {
	if data != nil {
		return json.NewEncoder(w).Encode(data)
	}
	return nil
}

// ToHandlerFunc returns an http.HandlerFunc composed of decoder, handler, and encoder that somewhat
// resembles encoder(handler(decoder(request))). It can be used to create `http.Handler`s for endpoints
// that require a decoder or encoder that isn't provided by this package.  Most common endpoints that
// expect JSON on the request and response body can be wrapped by this function.
func ToHandlerFunc[RequestType, ResponseType any](
	decoder HTTPDecoder[RequestType],
	endpoint APIEndpoint[RequestType, ResponseType],
	encoder HTTPEncoder[ResponseType],
) http.HandlerFunc {

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
				return
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

// DefaultJSONHandlerFunc uses the default decoder and encoder to wrap the `endpoint` returns an http.HandlerFunc
// that attempts to unmarshal JSON from the request body, use it and the request context as arguments to the provided `endpoint`
// function, and then write the response of that function as JSON
func DefaultJSONHandlerFunc[RequestType, ResponseType any](endpoint APIEndpoint[RequestType, ResponseType]) http.HandlerFunc {
	return ToHandlerFunc(DefaultHTTPDecoder[RequestType], endpoint, DefaultHTTPEncoder[ResponseType])
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
	json.NewEncoder(w).Encode(&ErrorResponse{Error: err.Error()})
}
