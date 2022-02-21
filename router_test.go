package locust

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/labstack/gommon/log"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func init() {
	log.SetLevel(log.DEBUG)
}

func TestRouter_GETOnlyEcho(t *testing.T) {
	r := New()

	r.GET("/testOnlyEcho", func(a Context) error {
		fmt.Println("Result Only Echo:", a.QueryParam("Query"))
		return nil
	})

	request, err := http.NewRequest("GET", "http://localhost/testOnlyEcho?Query=foo+bar", nil)
	if err != nil {
		t.Errorf("failed to make request: %v", err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != 200 {
		t.Fatal("got non 200 code", w.Code)
	}
}

func TestRouter_GET(t *testing.T) {
	r := New()
	r.GET("/test", func(a struct {
		Query string `bind:"query,json" as:"q,required"`
		Count int
		Ctx   Context
		_     string `return:"200"`
		_     error  `return:"400"`
	}) error {
		fmt.Println("Searching for...", a.Query)
		return a.Ctx.JSON(200, "wow")
	})

	request, err := http.NewRequest("GET", "http://localhost/test?q=hello+world&Count=69", nil)
	if err != nil {
		t.Errorf("failed to make request: %v", err)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != 200 {
		t.Fatal("got non 200 code", w.Code)
	}
	fmt.Printf("[%v] %v\n", w.Code, w.Body.String())
}

func TestRouter_GETValidationFailed(t *testing.T) {
	r := New()
	r.GET("/test", func(a struct {
		Query string `bind:"query,json" json:"query,omitempty"`
		Count int
		Ctx   Context
		_     string `return:"200"`
		_     error  `return:"400"`
	}) error {
		fmt.Println("Searching for...", a.Query)
		return a.Ctx.JSON(200, "wow")
	})

	request, err := http.NewRequest("GET", "http://localhost/test?Query=hello+world&Count=wow", nil)
	if err != nil {
		t.Errorf("failed to make request: %v", err)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, request)
	assert.Equal(t, w.Code, http.StatusBadRequest)
	fmt.Printf("[%v] %v\n", w.Code, w.Body.String())
}

func TestRouter_TagAs(t *testing.T) {
	r := New()
	r.GET("/test/:id", func(params struct {
		ID    uint
		Query string
		Count float64
		Token string `bind:"header" as:"X-Security-Token,sensitive"`
	}, ctx Context) error {
		assert.Equal(t, params.Token, "my humps")
		return ctx.JSON(200, map[string]interface{}{
			"request": params,
			"result":  fmt.Sprintf("Searched %v on %v", params.Query, params.ID),
		})
	})

	request, err := http.NewRequest("GET", "http://localhost/test/123?query=hello+world&count=0.51", nil)
	if err != nil {
		t.Errorf("failed to make request: %v", err)
	}
	request.Header.Add("X-Security-Token", "my humps")

	w := httptest.NewRecorder()
	x := time.Now()
	r.ServeHTTP(w, request)
	fmt.Println("request took", time.Now().Sub(x).Milliseconds(), "ms")
	assert.Equal(t, w.Code, 200)
	fmt.Printf("[%v] %v\n", w.Code, w.Body.String())
}

func TestHandler_DumpJson(t *testing.T) {
	r := New()
	x := time.Now()
	r.POST("/test", func(params struct {
		FromJson
		FirstName string `json:"first_name,omitempty"`
		LastName  string `json:"last_name,omitempty"`
		Age       int    `json:"age,omitempty"`
	}, ctx Context) error {
		assert.Equal(t, params.FirstName, "joe")
		return ctx.JSON(200, map[string]interface{}{
			"request": params,
			"result":  fmt.Sprintf("Body dump: %v %v", params.FirstName, params.LastName),
		})
	})

	marshal, err := json.Marshal(map[string]interface{}{"first_name": "joe", "last_name": "mama"})
	assert.NoError(t, err)
	request, err := http.NewRequest("POST", "http://localhost/test", bytes.NewBuffer(marshal))
	if err != nil {
		t.Errorf("failed to make request: %v", err)
	}

	w := httptest.NewRecorder()
	x = time.Now()
	r.ServeHTTP(w, request)
	fmt.Println("request took", time.Now().Sub(x).Microseconds(), "us")
	if w.Code != 200 {
		t.Errorf("got non 200 code: %v", w.Code)
	}
	fmt.Printf("[%v] %v\n", w.Code, w.Body.String())

}

func TestHandler_BodyDump(t *testing.T) {
	expect := "hello world!"
	r := New()
	r.GET("/test", func(params struct {
		Body io.Reader
		Ctx  Context `json:"-"`
	}) error {
		all, err := ioutil.ReadAll(params.Body)
		if err != nil {
			return err
		}

		assert.Equal(t, string(all), expect)
		return params.Ctx.JSON(200, map[string]interface{}{
			"request": params,
			"result":  fmt.Sprintf("Body dump: %v", string(all)),
		})
	})

	request, err := http.NewRequest("GET", "http://localhost/test", bytes.NewBufferString(expect))
	if err != nil {
		t.Errorf("failed to make request: %v", err)
	}

	w := httptest.NewRecorder()
	x := time.Now()
	r.ServeHTTP(w, request)
	fmt.Println("request took", time.Now().Sub(x).Milliseconds(), "ms")
	if w.Code != 200 {
		t.Errorf("got non 200 code: %v", w.Code)
	}
	fmt.Printf("[%v] %v\n", w.Code, w.Body.String())

}

func TestHandler_FormFileDump(t *testing.T) {
	file, err := os.OpenFile("wow.txt", os.O_RDWR, os.ModePerm)
	assert.NoError(t, err)
	defer file.Close()
	expect := "hello world!"
	id := "furry.png"
	r := New()

	r.POST("/test", func(params struct {
		ID     string                `bind:"form"`
		Avatar *multipart.FileHeader `as:"avatar"`
		Ctx    Context               `json:"-"`
	}) error {
		if params.Avatar == nil {
			assert.Fail(t, "avatar is nil")
			return nil
		}
		open, err := params.Avatar.Open()
		if err != nil {
			return err
		}
		defer open.Close()
		all, err := ioutil.ReadAll(open)
		if err != nil {
			return err
		}

		assert.Equal(t, id, params.ID)
		assert.Equal(t, string(all), expect)
		return params.Ctx.JSON(200, map[string]interface{}{
			"request": params,
			"result":  fmt.Sprintf("Body dump: %v", string(all)),
		})
	})

	// Prepare a form that you will submit to that URL.
	values := map[string]io.Reader{
		"avatar": file,
		"id":     bytes.NewBufferString(id),
	}
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for key, r := range values {
		var fw io.Writer
		var err error
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		// Add an image file
		if x, ok := r.(*os.File); ok {
			if fw, err = w.CreateFormFile(key, x.Name()); err != nil {
				t.Errorf("CreateFormFile: %w", err)
			}
		} else {
			// Add other fields
			if fw, err = w.CreateFormField(key); err != nil {
				t.Errorf("CreateFormField: %w", err)

			}
		}
		if _, err = io.Copy(fw, r); err != nil {
			t.Errorf("failed to copy: %w", err)
		}

	}

	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	// Now that you have a form, you can submit it to your handler.
	req, err := http.NewRequest("POST", "http://localhost/test", &b)
	if err != nil {
		return
	}
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())

	rec := httptest.NewRecorder()
	x := time.Now()
	r.ServeHTTP(rec, req)
	fmt.Println("request took", time.Now().Sub(x).Milliseconds(), "ms")
	if rec.Code != 200 {
		t.Errorf("got non 200 code: %v", rec.Code)
	}

	fmt.Printf("[%v] %v\n", rec.Code, rec.Body.String())

}
