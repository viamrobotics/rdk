package web

import (
	"encoding/json"
	"net/http"

	"go.viam.com/robotcore/api"
)

type Client struct {
	root string // e.g. http://localhost:8080
}

func (c *Client) Status() (api.Status, error) {
	s := api.Status{}

	res, err := http.Get(c.root + "/api/status")
	if err != nil {
		return s, err
	}
	defer res.Body.Close()
	d := json.NewDecoder(res.Body)
	err = d.Decode(&s)
	return s, err
}
