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
	convertedHandler := generichandler.ToHandlerFunc(api.AddPet)
	// this is just to satisfy httptest.NewRequest and doesn't really matter for these tests
	const apiPath = "/pet"

	tests := []testDef{
		{
			name: "successful add",
			petToTest: &pet{
				Name:  "fido",
				Owner: "jeff",
				Age:   3,
			},
			expectations: func(test *testing.T, input, output *pet, response *http.Response) {
				if response.StatusCode != 200 {
					t.Error("should have returned 200")
				}

				if output.addedAt.After(time.Now()) {
					t.Error("invalid timestamp on added Pet")
				}
			},
		},
	}

	for _, test := range tests {
		// 16 of the 17 lines of code here are boilerplate:
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
			if err := json.NewDecoder(httpResponse.Body).Decode(endpointResult); err != nil {
				t.Errorf("failed deserializing response body %v\n", err)
			}

			// run the assertions about the output
			test.expectations(t, test.petToTest, endpointResult, httpResponse)
		})
	}
}

func TestHandlerDirectly(t *testing.T) {
	// in the above example, we were testing against an HTTP endpoint using httptest, but if ToHandlerFunc works properly, we almost shouldn't need to
	// we can define the same tests as above, but change the way the test runs and the signature of the expectations function to simplify the tests and
	// remove boilerplate, uninteresting code:
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := api.AddPet(context.Background(), test.petToTest)
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

// our "pet API", that uses an in-memory datastore to keep track of people's pets
type petAPI struct {
	pets map[string]*pet
}

// AddPet adds a pet to the API
func (api *petAPI) AddPet(ctx context.Context, pet *pet) (*pet, error) {
	if len(pet.Name) == 0 {
		return nil, errors.WithMessage(generichandler.ErrorInvalidRequest, "the pet must have a name")
	}
	if len(pet.Owner) == 0 {
		return nil, errors.WithMessage(generichandler.ErrorInvalidRequest, "the pet must have an owner")
	}
	if pet.Age == 0 {
		return nil, errors.WithMessage(generichandler.ErrorInvalidRequest, "the pet must be at least 1 year old")
	}

	addedPet := pet
	addedPet.addedAt = time.Now()
	api.pets[pet.Name] = addedPet
	return addedPet, nil
}
