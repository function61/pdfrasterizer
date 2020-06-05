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
func (c *Client) RasterizeToJpeg(
	ctx context.Context,
	pdfBytes io.Reader,
) (io.ReadCloser, error) {
	return c.rasterizeToFormat(ctx, pdfBytes, "image/jpeg")
}

// returns PNG bytes
func (c *Client) RasterizeToPng(
	ctx context.Context,
	pdfBytes io.Reader,
) (io.ReadCloser, error) {
	return c.rasterizeToFormat(ctx, pdfBytes, "image/png")
}

func (c *Client) rasterizeToFormat(
	ctx context.Context,
	pdfBytes io.Reader,
	format string,
) (io.ReadCloser, error) {
	resp, err := ezhttp.Post(
		ctx,
		c.baseUrl+"/rasterize",
		ezhttp.AuthBearer(c.bearerToken),
		ezhttp.Header("Accept", format),
		ezhttp.SendBody(pdfBytes, "application/pdf"))
	if err != nil {
		return nil, fmt.Errorf("PDF rasterizer: %w", err)
	}

	return resp.Body, nil
}
