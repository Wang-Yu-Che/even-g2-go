package bridge

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wang-Yu-Che/even-g2-go/g2"
)

type fakeClient struct {
	text  string
	image image.Image
	left  g2.State
	right g2.State
}

func (c *fakeClient) DisplayText(_ context.Context, text string) error {
	c.text = text
	return nil
}

func (c *fakeClient) DisplayImage(_ context.Context, value image.Image) error {
	c.image = value
	return nil
}

func (c *fakeClient) States() (g2.State, g2.State) { return c.left, c.right }

func TestStatus(t *testing.T) {
	client := &fakeClient{left: g2.Ready, right: g2.Ready}
	server := &Server{client: client, ctx: t.Context(), maxBodyBytes: DefaultMaxBodyBytes}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/status", nil))
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"ready":true`)) {
		t.Fatalf("status = %d %s", response.Code, response.Body.String())
	}
}

func TestPostTextJSONAndRaw(t *testing.T) {
	client := &fakeClient{}
	server := &Server{client: client, ctx: t.Context(), maxBodyBytes: DefaultMaxBodyBytes}
	request := httptest.NewRequest(http.MethodPost, "/text", bytes.NewBufferString(`{"text":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK || client.text != "hello" {
		t.Fatalf("JSON text = %q, status=%d", client.text, response.Code)
	}

	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/text", bytes.NewBufferString("raw")))
	if response.Code != http.StatusOK || client.text != "raw" {
		t.Fatalf("raw text = %q, status=%d", client.text, response.Code)
	}
}

func TestPostImage(t *testing.T) {
	client := &fakeClient{}
	server := &Server{client: client, ctx: t.Context(), maxBodyBytes: DefaultMaxBodyBytes}
	source := image.NewGray(image.Rect(0, 0, 2, 1))
	source.SetGray(0, 0, color.Gray{Y: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/image", &encoded))
	if response.Code != http.StatusOK || client.image == nil {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
