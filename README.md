# generic-service

Using generics to reduce some boilerplate code from HTTP handlers.

This module doesn't have very much code implemented in it; it isn't a middleware or a mux, it isn't really a framework, and it isn't something you can quickly drop into an existing codebase. This is really just an experiment. The hope is to relieve some of the annoyances I'm used to experiencing when writing REST APIs and allow us to test the endpoint as if they were simply functions, irrespective of the HTTP context. I think in general that probably isn't a great idea because the functions _do_ run in an HTTP context, but it makes generating inputs and expected results easier.

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

The `httptest` library exists to test these functions and in general, I think that's how we should be testing APIs; however, the function itself is "weird" because the return type is void. The way the programmer communicates with the caller is via the `http.ResponseWriter` object, which makes sense when we think about what's actually happening to allow us to talk over HTTP, but it makes writing code for larger APIs very tedious.

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

To do that, we need to set some "sensible defaults":
* The client is sending JSON and expecting JSON in return
* `RequestType`s for each endpoint should implement `Validate(context.Context) error`
* Endpoints should use `errors.WithMessage(error, string)` with one of the ones defined in this package when they want to signal a specific error, e.g. NotFound, InvalidRequest