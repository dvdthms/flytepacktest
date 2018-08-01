package main

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"time"
)

type Input struct {
	URL     string        `json:"string"`
	Timeout time.Duration `json:"timeout"`
	Headers http.Header   `json:"headers"`
	Body    string        `json:"body"`
}

type Output struct {
	StatusCode int         `json:"statusCode"`
	Header     http.Header `json:"header"`
	Body       interface{} `json:"body"`
}

type HttpClient interface {
	Do(*http.Request) (*http.Response, error)
}

func DoPost(client HttpClient, input Input) (*Output, error) {
	req, err := http.NewRequest(http.MethodPost, input.URL, bytes.NewReader([]byte(input.Body)))
	if err != nil {
		return nil, err
	}
	req.Header = input.Headers

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	bodyContent, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}

	return &Output{
		StatusCode: resp.StatusCode,
		Body:       contentTypeHandler(resp.Header.Get("Content-Type"), bodyContent),
		Header:     resp.Header,
	}, nil
}

func contentTypeHandler(contentType string, body []byte) interface{} {
	switch contentType {
	case "application/json":
		return body
	default:
		return base64.URLEncoding.EncodeToString(body)
	}
}
