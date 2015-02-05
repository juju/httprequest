# httprequest
--
    import "github.com/juju/httprequest"

Package httprequest provides functionality for unmarshaling HTTP request
parameters into a struct type.

## Usage

#### func  Unmarshal

```go
func Unmarshal(p Params, x interface{}) error
```
Unmarshal takes values from given parameters and fills out fields in x, which
must be a pointer to a struct.

Tags on the struct's fields determine where each field is filled in from.
Similar to encoding/json and other encoding packages, the tag holds a
comma-separated list. The first item in the list is an alternative name for the
field (the field name itself will be used if this is empty). The next item
specifies where the field is filled in from. It may be:

    "path" - the field is taken from a parameter in p.PathVar
    	with a matching field name.

    "form" - the field is taken from the given name in p.Form
    	(note that this covers both URL query parameters and
    	POST form parameters)

    "body" - the field is filled in by parsing the request body
    	as JSON.

For path and form parameters, the field will be filled out from the field in
p.PathVar or p.Form using one of the following methods (in descending order of
preference):

- if the type is string, it will be set from the first value.

- if the type is []string, it will be filled out using all values for that field

    (allowed only for form)

- if the type implements encoding.TextUnmarshaler, its UnmarshalText method will
be used

- otherwise fmt.Sscan will be used to set the value.

#### type Params

```go
type Params struct {
	*http.Request
	PathVar httprouter.Params
}
```

Params holds request parameters that can be unmarshaled into a struct.
