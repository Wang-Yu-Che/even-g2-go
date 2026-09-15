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
	text       string
	image      image.Image
	left       g2.State
	right      g2.State
	brightness g2.BrightnessOptions
	headUp     struct {
		enabled bool
		angle   int
	}
	position       g2.ScreenPosition
	dashboardCalls int
}

func (c *fakeClient) DisplayText(_ context.Context, text string) error {
	c.text = text
	return nil
}

func (c *fakeClient) DisplayImage(_ context.Context, value image.Image) error {
	c.image = value
	return nil
}

func (c *fakeClient) Status() g2.Status {
	return g2.Status{Left: c.left, Right: c.right, Ready: c.left == g2.Ready && c.right == g2.Ready}
}
func (c *fakeClient) RequestDeviceSettings(context.Context) (g2.DeviceSettings, error) {
	return g2.DeviceSettings{BatteryPercent: 87}, nil
}
func (c *fakeClient) SetBrightness(_ context.Context, options g2.BrightnessOptions) error {
	c.brightness = options
	return nil
}
func (c *fakeClient) SetHeadUp(_ context.Context, enabled bool, angle int) error {
	c.headUp.enabled = enabled
	c.headUp.angle = angle
	return nil
}
func (c *fakeClient) SetScreenPosition(_ context.Context, position g2.ScreenPosition) error {
	c.position = position
	return nil
}
func (c *fakeClient) ShowDashboard(context.Context) error {
	c.dashboardCalls++
	return nil
}

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

func TestSettingsAndControls(t *testing.T) {
	client := &fakeClient{}
	server := &Server{client: client, ctx: t.Context(), maxBodyBytes: DefaultMaxBodyBytes}
	tests := []struct {
		path string
		body string
	}{
		{path: "/brightness", body: `{"level":68,"auto":true}`},
		{path: "/head-up", body: `{"enabled":true,"angle":35}`},
		{path: "/screen-position", body: `{"height":4,"depth":1}`},
		{path: "/dashboard", body: `{}`},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, test.path, bytes.NewBufferString(test.body))
		server.Handler().ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("POST %s = %d %s", test.path, response.Code, response.Body.String())
		}
	}
	if client.brightness != (g2.BrightnessOptions{Level: 68, Auto: true}) {
		t.Fatalf("brightness = %+v", client.brightness)
	}
	if !client.headUp.enabled || client.headUp.angle != 35 {
		t.Fatalf("head-up = %+v", client.headUp)
	}
	if client.position != (g2.ScreenPosition{Height: 4, Depth: 1}) {
		t.Fatalf("position = %+v", client.position)
	}
	if client.dashboardCalls != 1 {
		t.Fatalf("dashboard calls = %d", client.dashboardCalls)
	}

	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/settings", nil))
	if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"batteryPercent":87`)) {
		t.Fatalf("GET /settings = %d %s", response.Code, response.Body.String())
	}
}
