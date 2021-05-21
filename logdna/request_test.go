package logdna

import (
	"encoding/json"
	// "errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"fmt"
	"strings"
	"io"
	"io/ioutil"
	"errors"

	"github.com/stretchr/testify/assert"
)

const SERVICE_KEY = "abc123"

type badClient struct{}
func (fc *badClient) Do(*http.Request) (*http.Response, error) {
	return nil, errors.New("FAKE ERROR calling HTTPClient.Do")
}

func setHttpRequest(customReq HttpRequest) func(*requestConfig) {
	return func(req *requestConfig) {
		req.HttpRequest = customReq
	}
}

func setBodyReader(customReader BodyReader) func(*requestConfig) {
	return func(req *requestConfig) {
		req.BodyReader = customReader
	}
}

func setJSONMarshal(customMarshaller jsonMarshal) func(*requestConfig) {
	return func(req *requestConfig) {
		req.jsonMarshal = customMarshaller
	}
}

func TestRequest_MakeRequest(t *testing.T) {
  assert := assert.New(t)
	pc := providerConfig{ServiceKey: SERVICE_KEY}
  resourceId := "test123456"

	t.Run("Server receives proper method, URL, and headers", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	    assert.Equal("GET", r.Method,  "Method is correct")
	    assert.Equal(fmt.Sprintf("/someapi/%s", resourceId), r.URL.String(), "URL is correct")
			key, ok := r.Header["Servicekey"]
	    assert.Equal(true, ok, "servicekey header exists")
	    assert.Equal(1, len(key), "servicekey header is correct")
			key = r.Header["Content-Type"]
	    assert.Equal("application/json", key[0], "content-type header is correct")
	  }))
	  defer ts.Close()

		pc.Host = ts.URL

		req := NewRequestConfig(
			&pc,
			"GET",
			fmt.Sprintf("someapi/%s", resourceId),
			nil,
		)

	  _, err := req.MakeRequest()
		assert.Nil(err, "No errors")
	})

	t.Run("Reads and decodes response from the server", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ViewResponse{ViewID: "test123456"})
		}))
		defer ts.Close()

		pc.Host = ts.URL

		req := NewRequestConfig(
			&pc,
			"GET",
			fmt.Sprintf("someapi/%s", resourceId),
			nil,
		)

		body, err := req.MakeRequest()
		assert.Nil(err, "No errors")
		assert.Equal(
			`{"viewID":"test123456"}`,
			strings.TrimSpace(string(body)),
			"Returned body is correct",
		)
	})

	t.Run("Successfully marshals a provided body", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			postedBody, _ := ioutil.ReadAll(r.Body)
			assert.Equal(
				`{"name":"Test View"}`,
				strings.TrimSpace(string(postedBody)),
				"Body got marshalled and sent correctly",
			)
	  }))
	  defer ts.Close()

		pc.Host = ts.URL

		req := NewRequestConfig(
			&pc,
			"POST",
			"someapi",
			ViewRequest{
				Name: "Test View",
			},
		)

	  _, err := req.MakeRequest()
		assert.Nil(err, "No errors")
	})

	t.Run("Handles errors when marshalling JSON", func(t *testing.T) {
		const ERROR = "FAKE ERROR during json.Marshal"
		req := NewRequestConfig(
			&pc,
			"POST",
			"will/not/work",
			ViewRequest{Name: "NOPE"},
			setJSONMarshal(func(interface{}) ([]byte, error) {
				return nil, errors.New(ERROR)
			}),
		)
		body, err := req.MakeRequest()
		assert.Nil(body, "No body due to error")
		assert.Error(err, "Expected error")
		assert.Equal(
			ERROR,
			err.Error(),
			"Expected error message",
		)
	})

	t.Run("Handles errors when creating a new HTTP request", func(t *testing.T) {
		const ERROR = "FAKE ERROR for http.NewRequest"
		req := NewRequestConfig(
			&pc,
			"GET",
			"will/not/work",
			nil,
			setHttpRequest(func(string, string, io.Reader) (*http.Request, error) {
				return nil, errors.New(ERROR)
			}),
		)
		body, err := req.MakeRequest()
		assert.Nil(body, "No body due to error")
		assert.Error(err, "Expected error")
		assert.Equal(
			ERROR,
			err.Error(),
			"Expected error message",
		)
	})

	t.Run("Handles errors during the HTTP request", func(t *testing.T) {
		req := NewRequestConfig(
			&pc,
			"GET",
			"will/not/work",
			nil,
			func(req *requestConfig) {
				req.HTTPClient = &badClient{}
			},
		)

		body, err := req.MakeRequest()
		assert.Nil(body, "No body due to error")
		assert.Error(err, "Expected error")
		assert.Equal(
			true,
			strings.Contains(err.Error(), "Error during HTTP request: FAKE ERROR calling HTTPClient.Do"),
			"Expected error message",
		)
	})

	t.Run("Throws non-200 errors returned by the server", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(400)
		}))
		defer ts.Close()

		pc.Host = ts.URL

		req := NewRequestConfig(
			&pc,
			"POST",
			fmt.Sprintf("someapi/%s", resourceId),
			nil,
		)

		_, err := req.MakeRequest()
		assert.Error(err, "Expected error")
		assert.Equal(
			true,
			strings.Contains(err.Error(), "status NOT OK: 400"),
			"Expected error message",
		)
	})

	t.Run("Handles errors when creating a new HTTP request", func(t *testing.T) {
		const ERROR = "FAKE ERROR for body reader"
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ViewResponse{ViewID: "test123456"})
		}))
		defer ts.Close()

		pc.Host = ts.URL
		req := NewRequestConfig(
			&pc,
			"GET",
			fmt.Sprintf("someapi/%s", resourceId),
			nil,
			setBodyReader(func(io.Reader) ([]byte, error) {
				return nil, errors.New(ERROR)
			}),
		)
		body, err := req.MakeRequest()
		assert.Nil(body, "No body due to error")
		assert.Error(err, "Expected error")
		assert.Equal(
			true,
			strings.Contains(err.Error(), "Error parsing HTTP response: " + ERROR),
			"Expected error message",
		)
	})
}