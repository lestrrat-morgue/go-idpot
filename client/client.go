package client

import (
  "fmt"
  "io/ioutil"
  "net/http"
  "net/url"
  "strconv"
)

// Client talks to the idpot server to fetch IDs
type Client struct {
  baseURL string
}

// New creates a new Client. You must pass a valid idpot server URL
func New(base string) *Client {
  return &Client { base }
}

// CreatePot takes a pot name, and a minimum ID value
func (c *Client) CreatePot(pot string, min uint64) error {
  uri := fmt.Sprintf("%s/pot/create", c.baseURL)
  res, err := http.PostForm(uri, url.Values { "name": {pot}, "min": {fmt.Sprintf("%d", min)} })
  if err != nil {
    return err
  }

  if res.StatusCode != 201 {
    return fmt.Errorf("expected status code 201, got %d", res.StatusCode)
  }

  return nil
}

// CurrentID queries the server for the current ID for pot named `pot`
func (c *Client) CurrentID(pot string) (uint64, error) {
  uri := fmt.Sprintf("%s/id/%s", c.baseURL, pot)
  res, err := http.Get(uri)
  if err != nil {
    return 0, err
  }

  return c.processIDResponse(res)
}

// NextID fetches the next ID for pot `pot` from the server
func (c *Client) NextID(pot string) (uint64, error) {
  uri := fmt.Sprintf("%s/id/%s", c.baseURL, pot)
  res, err := http.Post(uri, "text/plain", nil)
  if err != nil {
    return 0, err
  }

  return c.processIDResponse(res)
}

func (c *Client) processIDResponse(res *http.Response) (uint64, error) {
  if res.StatusCode != 200 {
    return 0, fmt.Errorf("expected status code 200, got %d", res.StatusCode)
  }

  // Grab the response
  content, err := ioutil.ReadAll(res.Body)
  if err != nil {
    return 0, err
  }

  // parse the response
  id, err := strconv.ParseUint(string(content), 10, 64)
  if err != nil {
    return 0, err
  }

  return id, nil
}

