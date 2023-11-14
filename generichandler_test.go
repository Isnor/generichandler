package generichandler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Isnor/generichandler"
	"github.com/pkg/errors"
)

func TestToHandlerFunc(t *testing.T) {
	type testDef struct {
		name         string
		petToTest    *pet
		expectations func(test *testing.T, input *pet, output *pet, response *http.Response)
	}

	api := &petAPI{
		pets: make(map[string]*pet),
	}
	// "convert" the "regular function" of api.AddPet into an "endpoint function", i.e. a `HandlerFunc`
	convertedHandler := generichandler.DefaultJSONHandlerFunc(api.addPet)
	// this is just to satisfy httptest.NewRequest and doesn't really matter for these tests
	const apiPath = "/pet"

	// these are the simple "given this input, this function responds with this output" test definitions
	tests := []testDef{
		{
			name: "successful add",
			petToTest: &pet{
				Name:  "fido",
				Owner: "jeff",
				Age:   3,
			},
			expectations: func(test *testing.T, input, output *pet, response *http.Response) {
				if response.StatusCode != http.StatusOK {
					test.Error("should have returned 200")
				}

				if output.addedAt.After(time.Now()) {
					test.Error("invalid timestamp on added Pet")
				}
			},
		},
		{
			name: "no name",
			petToTest: &pet{
				Owner: "jeff",
				Age:   3,
			},
			expectations: func(test *testing.T, input, output *pet, response *http.Response) {
				if response.StatusCode != http.StatusBadRequest {
					test.Error("should have returned 400")
				}
			},
		},
		{
			name: "no owner",
			petToTest: &pet{
				Name: "fifi",
				Age:  3,
			},
			expectations: func(test *testing.T, input, output *pet, response *http.Response) {
				if response.StatusCode != http.StatusBadRequest {
					test.Error("should have returned 400")
				}
			},
		},
		{
			name: "no age",
			petToTest: &pet{
				Name:  "fifi",
				Owner: "jessica",
			},
			expectations: func(test *testing.T, input, output *pet, response *http.Response) {
				if response.StatusCode != http.StatusBadRequest {
					test.Error("should have returned 400")
				}
			},
		},
	}

	for _, test := range tests {
		// 16 of the 17 lines of code here are boilerplate, but this also only covers a very limited set of fairly simple endpoints.
		// Without generics though, we need to write this code for every RequestType and Endpoint in the service.
		// I do think this is how APIs should be tested so that we don't lose the ability to makes assertions about our HTTP response,
		// but I also wish this weren't so verbose.
		t.Run(test.name, func(t *testing.T) {

			// turn our request/test input into an HTTP request
			request := httptest.NewRequest(http.MethodPost, apiPath, nil)
			if test.petToTest != nil {
				body, err := json.Marshal(test.petToTest)
				if err != nil {
					t.Errorf("error writing post request body %v %v\n", err, test.petToTest)
					return
				}
				request = httptest.NewRequest(http.MethodPost, apiPath, bytes.NewBuffer(body))
			}

			// now we can execute our function as an endpoint:
			recorder := httptest.NewRecorder()
			convertedHandler(recorder, request)
			endpointResult := &pet{}
			httpResponse := recorder.Result()
			// unmarshal the response body - we're assuming that the (converted) handler has encoded a JSON body
			if err := json.NewDecoder(httpResponse.Body).Decode(endpointResult); err != nil {
				t.Errorf("failed deserializing response body %v\n", err)
			}

			// run the assertions about the output
			test.expectations(t, test.petToTest, endpointResult, httpResponse)
		})
	}
}

func TestHandlerDirectly(t *testing.T) {
	// in the above example, we were testing against an HTTP endpoint using httptest, but if ToHandlerFunc works properly and all we want
	// is to validate the behaviour of the endpoint, then we can just skip the unmarshal/marshal behaviour.
	// By sacrificing the HTTP context, we can remove what is often uninteresting code and just test the input and output of the function:
	type testDef struct {
		name         string
		petToTest    *pet
		expectations func(test *testing.T, input *pet, output *pet, err error)
	}

	api := &petAPI{
		pets: make(map[string]*pet),
	}

	tests := []testDef{
		{
			name: "successful add",
			petToTest: &pet{
				Name:  "fido",
				Owner: "jeff",
				Age:   3,
			},
			expectations: func(test *testing.T, input, output *pet, err error) {
				if err != nil {
					t.Error("did not expect an error: ", err)
				}
				if !output.addedAt.Before(time.Now()) {
					t.Error("added at time was bad")
				}
			},
		},
		{
			name: "no name",
			petToTest: &pet{
				Owner: "jeff",
				Age:   3,
			},
			expectations: func(test *testing.T, input, output *pet, err error) {
				if err != nil {
					test.Error("expected an error")
					return
				}
			},
		},
		{
			name: "no owner",
			petToTest: &pet{
				Name: "fifi",
				Age:  3,
			},
			expectations: func(test *testing.T, input, output *pet, err error) {
				if err != nil {
					test.Error("expected an error")
				}
			},
		},
		{
			name: "no age",
			petToTest: &pet{
				Name:  "fifi",
				Owner: "jessica",
			},
			expectations: func(test *testing.T, input, output *pet, err error) {
				if err != nil {
					test.Error("expected an error")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := api.addPet(context.Background(), test.petToTest)
			test.expectations(t, test.petToTest, result, err)
		})
	}
}

// this is just a simple "Pet" API; it doesn't really do much, it just gives us _something_ to test
type pet struct {
	Name    string
	Owner   string
	Age     uint
	addedAt time.Time
}

// Validate returns an error if Name, Owner, or Age is not set
func (p pet) Validate(context.Context) error {
	if len(p.Name) == 0 {
		return errors.WithMessage(generichandler.ErrorInvalidRequest, "the pet must have a name")
	}
	if len(p.Owner) == 0 {
		return errors.WithMessage(generichandler.ErrorInvalidRequest, "the pet must have an owner")
	}
	if p.Age == 0 {
		return errors.WithMessage(generichandler.ErrorInvalidRequest, "the pet must be at least 1 year old")
	}

	return nil
}

// our "pet API", that uses an in-memory datastore to keep track of people's pets
type petAPI struct {
	pets map[string]*pet
}

// addPet adds a pet to the API. This function assumes that the provided pet is valid
func (api *petAPI) addPet(ctx context.Context, pet *pet) (*pet, error) {
	addedPet := pet
	addedPet.addedAt = time.Now()
	api.pets[pet.Name] = addedPet
	return addedPet, nil
}
