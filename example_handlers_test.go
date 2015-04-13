package httprequest_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/julienschmidt/httprouter"

	"github.com/juju/httprequest"
)

type arithHandler struct {
}

type number struct {
	N int
}

func (arithHandler) Add(arg *struct {
	httprequest.Route `httprequest:"GET /:A/add/:B"`
	A                 int `httprequest:",path"`
	B                 int `httprequest:",path"`
}) (number, error) {
	return number{
		N: arg.A + arg.B,
	}, nil
}

func ExampleErrorMapper_Handlers() {
	f := func(p httprequest.Params) (arithHandler, error) {
		fmt.Printf("handle %s %s\n", p.Request.Method, p.Request.URL)
		return arithHandler{}, nil
	}
	router := httprouter.New()
	for _, h := range exampleErrorMapper.Handlers(f) {
		router.Handle(h.Method, h.Path, h.Handle)
	}
	srv := httptest.NewServer(router)
	resp, err := http.Get(srv.URL + "/123/add/11")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		panic("status " + resp.Status)
	}
	fmt.Println("result:")
	io.Copy(os.Stdout, resp.Body)
	// Output: handle GET /123/add/11
	// result:
	// {"N":134}
}

type exampleErrorResponse struct {
	Message string
}

var exampleErrorMapper httprequest.ErrorMapper = func(err error) (int, interface{}) {
	return http.StatusInternalServerError, &exampleErrorResponse{
		Message: err.Error(),
	}
}
