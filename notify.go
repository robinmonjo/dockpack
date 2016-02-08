package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func put(hook string, payload interface{}, w io.Writer) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", hook, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	u, err := url.Parse(hook)
	if err != nil {
		return err
	}

	var client *http.Client

	if u.Scheme == "https" {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{Transport: tr}
	} else {
		client = &http.Client{}
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	io.Copy(w, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("bad status code expected 200 .. 299 got %d", resp.Status)
	}
	return nil
}
