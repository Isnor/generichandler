# generic handler

Using generics to reduce some boilerplate code from HTTP handlers.

This module doesn't have very much code implemented in it; it isn't a middleware or a mux, it isn't really a framework, and it isn't something you can quickly drop into an existing codebase. It's more like guidelines and restrictions that try to encourage clean APIs. The hope is to relieve some of the annoyances I'm used to experiencing when writing REST APIs and allow us to test the endpoint as if they were simply functions, irrespective of the HTTP context. This isn't always desirable, but can be useful if it's otherwise very difficult to simulate or mock the behaviour required to pass a particular state to or reach a certain code path in the handler.

## Design

The main idea is to remove the need to write endpoint handlers as `http.HandlerFunc`s:

```golang
func AddPet(w http.ResponseWriter, r *http.Request) {
  petToAdd := &Pet{}
  if err := json.NewDecoder(r.Body).Decode(petToAdd); err != nil {
    w.WriteHeader(http.StatusBadRequest)
    return
  }

  // use the pet
  w.Header().Set("Content-Type", "application/json")
  if addPet(petToAdd) != nil {
    w.WriteHeader(http.InternalServerError)
    return
  }
  w.WriteHeader(http.StatusOK)
}
```

The `httptest` library exists to test these functions and in general, I think that's how we should be testing APIs; however, the `AddPet` function itself is "weird" because the return type is void. The way the programmer communicates with the caller is via the `http.ResponseWriter` object, which makes sense when we think about what's actually happening to allow us to talk over HTTP, but it makes writing code for larger APIs very tedious and difficult to test.



### Simple Example

Most of the time when we write handler functions, we define a struct with JSON tags for the endpoint, unmarshal the `*http.Request`, and then use the struct, like in the above example. The most interesting part of the code is what we do with the `Pet`:

```golang
func addPet(p *Pet) error {
  // put the pet in the database, etc.
}
```

And because we know this will execute in an HTTP request, let's add a `context.Context` parameter and return a new `Pet` instance for cases when the database writes to the object (e.g. UUID or timestamp):

```golang
func addPet(ctx context.Context, p *Pet) (*Pet, error) {
  // put the pet in the database, etc.
}
```

So our `HandlerFunc` looks like:

```golang
func AddPet(w http.ResponseWriter, r *http.Request) {
  // deserialize the body of the request
  petToAdd := &Pet{}
  if err := json.NewDecoder(r.Body).Decode(petToAdd); err != nil {
    w.WriteHeader(http.StatusBadRequest)
    return
  }
  w.Header().Set("Content-Type", "application/json")

  // add the pet
  addedPet, err := addPet(petToAdd)
  
  // handle errors encountered
  if err != nil {
    w.WriteHeader(http.StatusInternalServerError)
    json.NewEncoder(w).Encode(Error{Text: "failed added pet"})
    return
  }

  // handle success
  json.NewEncoder(w).Encode(pet)
  w.WriteHeader(http.StatusOK)
}
```

Breaking that up, we have:
* the bit that unmarshals - boring
* the bit that actually does something - less boring 
* the code the handles its return values - fairly boring as well

It looks like if we make some assumptions about how the RequestType is modeled and how the API functions should work, we can generalize this into a fairly neat pattern, and since generics were added in `1.18`, we don't need to rely on type assertions any more. In a nutshell, that's all this module does; abstract away the HTTP components and focus on the functionality of the service / endpoint.

And those assumptions are:
* The client is sending JSON and expecting JSON in return;
* The endpoint has a need to have many varing input<->output verifications;
* `RequestType`s for each endpoint should implement `Validate(context.Context) error`
* all of the information that the handler requires can be passed to it via the (context, RequestType) it receives.
  * because of this, the programmer needs to be able to control how the request information is deserialized. If they want to decode auth information and store it in the `context`, fine; if they want to store it on the request object, fine as well