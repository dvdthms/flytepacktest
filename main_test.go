package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDoPost(t *testing.T) {
	body, _ := json.Marshal(`{"canned":"response"}`)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))

	headers := http.Header{"X-Random-Value": {"text/plain"}}
	input := Input{Headers: headers, Body: "The Body", URL: ts.URL}

	client := &http.Client{}

	output, _ := DoPost(client, input)

	expected := &Output{
		StatusCode: http.StatusOK,
		Body:       body,
		Header:     http.Header{"Content-Type": {"application/json"}},
	}

	assert.Equal(t, expected.StatusCode, output.StatusCode)
	assert.Equal(t, "application/json", output.Header.Get("Content-Type"))
	assert.Equal(t, expected.Body, output.Body)
}

func TestDoPostShouldConstructCorrectRequestFromInput(t *testing.T) {
	var req *http.Request
	var reqBody string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req = r
		reqBody = toString(r.Body)
	}))

	rand.Seed(time.Now().Unix())
	rv1 := fmt.Sprintf("%d", rand.Int())
	rv2 := fmt.Sprintf("%d", rand.Int())

	randomHeader := fmt.Sprintf("X-Random-%d", rand.Int())

	headers := http.Header{randomHeader: {rv1, rv2}, "X-Another-Header": {"Is Present"}}
	input := Input{Body: "The Body!", Headers: headers, URL: ts.URL + "/testuri"}

	client := &http.Client{}

	DoPost(client, input)

	assert.NotNil(t, req)

	assert.Equal(t, rv1, req.Header[randomHeader][0])
	assert.Equal(t, rv2, req.Header[randomHeader][1])
	assert.Equal(t, "Is Present", req.Header.Get("X-Another-Header"))

	assert.Equal(t, http.MethodPost, req.Method)
	assert.Equal(t, "/testuri", req.RequestURI)
	assert.Equal(t, "The Body!", reqBody)
}

func TestDoPostDoesPerformRequest(t *testing.T) {
	gotCalled := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCalled = true
	}))

	input := Input{URL: ts.URL}
	client := &http.Client{}

	DoPost(client, input)

	assert.True(t, gotCalled)
}

func TestDoPostShouldReturnErrorIfRequestConstructionFails(t *testing.T) {
	client := &http.Client{}
	_, err := DoPost(client, Input{URL: "://"})

	assert.Error(t, err)
	assert.Equal(t, "parse ://: missing protocol scheme", err.Error())
}

func TestShouldTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
	}))

	input := Input{Timeout: 5 * time.Millisecond, URL: ts.URL}
	client := &http.Client{Timeout: input.Timeout}
	_, err := DoPost(client, input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Timeout exceeded")
}

func TestShouldReturnResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("The Body"))
	}))

	input := Input{URL: ts.URL}
	client := &http.Client{}

	output, err := DoPost(client, input)

	assert.NoError(t, err)

	assert.Equal(t, http.StatusCreated, output.StatusCode)
	assert.Equal(t, "text/plain", output.Header.Get("Content-Type"))

	expectedBody := base64.URLEncoding.EncodeToString([]byte("The Body"))
	assert.Equal(t, expectedBody, output.Body)
}

func TestShouldReturnErrorIfReadingBodyReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	trc := &TestReadCloser{
		ReadFunc: func(p []byte) (n int, err error) {
			return 0, fmt.Errorf("could not read response body")
		},
	}

	input := Input{URL: ts.URL}
	client := &MockHttpClient{DoFunc: func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			Body: trc,
		}, nil
	}}

	_, err := DoPost(client, input)

	assert.Error(t, err)
	assert.Equal(t, "could not read response body", err.Error())
}

func TestShouldCallCloseReader(t *testing.T) {
	rc := ioutil.NopCloser(strings.NewReader("A string"))

	gotCalled := false
	trc := &TestReadCloser{
		rc: rc,
		CloseFunc: func() error {
			gotCalled = true
			return nil
		},
	}

	s := toString(trc)

	assert.Equal(t, "A string", s)
	assert.True(t, gotCalled)
}

func TestShouldReturnErrorIfClosingReaderReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	trc := &TestReadCloser{
		rc: ioutil.NopCloser(strings.NewReader("")),
		CloseFunc: func() error {
			return fmt.Errorf("could not close response body")
		},
	}

	input := Input{URL: ts.URL}
	client := &MockHttpClient{DoFunc: func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			Body: trc,
		}, nil
	}}

	_, err := DoPost(client, input)

	assert.Error(t, err)
	assert.Equal(t, "could not close response body", err.Error())
}

func TestShouldConvertResponseBodyToString(t *testing.T) {
	rc1 := ioutil.NopCloser(strings.NewReader("The Body"))
	rc2 := ioutil.NopCloser(strings.NewReader("A string"))

	s1 := toString(rc1)
	s2 := toString(rc2)

	assert.Equal(t, "The Body", s1)
	assert.Equal(t, "A string", s2)
}

func TestShouldReturnJsonInOutputIfResponseContentTypeIsJson(t *testing.T) {
	expectedBody, _ := json.Marshal(`{"canned":"response"}`)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	}))

	input := Input{URL: ts.URL}
	client := &http.Client{}

	output, _ := DoPost(client, input)

	assert.Equal(t, "application/json", output.Header.Get("Content-Type"))
	assert.Equal(t, expectedBody, output.Body)
}

func TestShouldReturnBase64EncodedStringInOutputIfResponseContentTypeIsNotJson(t *testing.T) {
	body := []byte("The Body")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))

	input := Input{URL: ts.URL}
	client := &http.Client{}

	output, _ := DoPost(client, input)

	expectedBody := base64.URLEncoding.EncodeToString(body)

	assert.Equal(t, "text/plain", output.Header.Get("Content-Type"))
	assert.Equal(t, expectedBody, output.Body)
}

func TestShouldReturnEmptyStringInOutputIfResponseBodyIsEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	input := Input{URL: ts.URL}
	client := &http.Client{}

	output, _ := DoPost(client, input)
	assert.Equal(t, "", output.Body)
}

type MockHttpClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (c *MockHttpClient) Do(req *http.Request) (*http.Response, error) {
	if c.DoFunc != nil {
		return c.DoFunc(req)
	}
	return nil, nil
}

type TestReadCloser struct {
	ReadFunc  func(p []byte) (n int, err error)
	CloseFunc func() error
	rc        io.ReadCloser
}

func (r *TestReadCloser) Read(p []byte) (n int, err error) {
	if r.ReadFunc != nil {
		return r.ReadFunc(p)
	}
	return r.rc.Read(p)
}

func (r *TestReadCloser) Close() error {
	if r.CloseFunc != nil {
		return r.CloseFunc()
	}
	return r.rc.Close()
}

func toString(closer io.ReadCloser) string {
	var buf bytes.Buffer
	buf.ReadFrom(closer)
	closer.Close()
	return buf.String()
}
