package pdfrasterizerclient

import (
	"context"
	"fmt"
	"io"

	"github.com/function61/gokit/envvar"
	"github.com/function61/gokit/ezhttp"
)

const (
	Function61 = "https://function61.com/api/pdfrasterizer"
	Localhost  = "http://localhost"
)

type TokenFn func() (string, error)

func TokenFromEnv() (string, error) {
	return envvar.Required("PDFRASTERIZER_TOKEN")
}

func NoToken() (string, error) {
	return "", nil
}

type Client struct {
	baseUrl     string
	bearerToken string
}

func New(baseUrl string, getToken TokenFn) (*Client, error) {
	bearerToken, err := getToken()
	if err != nil {
		return nil, fmt.Errorf("getToken: %w", err)
	}

	return &Client{baseUrl, bearerToken}, nil
}

// returns JPEG bytes
func (c *Client) Rasterize(
	ctx context.Context,
	pdfBytes io.Reader,
) (io.ReadCloser, error) {
	resp, err := ezhttp.Post(
		ctx,
		c.baseUrl+"/rasterize",
		ezhttp.AuthBearer(c.bearerToken),
		ezhttp.Header("Accept", "image/jpeg"),
		ezhttp.SendBody(pdfBytes, "application/pdf"))
	if err != nil {
		return nil, fmt.Errorf("PDF rasterizer: %w", err)
	}

	return resp.Body, nil
}
