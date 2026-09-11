package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/Wang-Yu-Che/even-g2-go/g2"
)

const DefaultMaxBodyBytes int64 = 12 << 20

type displayClient interface {
	DisplayText(context.Context, string) error
	DisplayImage(context.Context, image.Image) error
	States() (g2.State, g2.State)
}

// Server exposes a small HTTP bridge for one G2 client.
type Server struct {
	client       displayClient
	ctx          context.Context
	maxBodyBytes int64
	operations   sync.Mutex
}

// NewServer creates an HTTP bridge handler.
func NewServer(client *g2.Client) *Server {
	return NewServerWithContext(context.Background(), client)
}

// NewServerWithContext binds display work to the server lifecycle.
func NewServerWithContext(ctx context.Context, client *g2.Client) *Server {
	return &Server{client: client, ctx: ctx, maxBodyBytes: DefaultMaxBodyBytes}
}

// Handler returns the HTTP bridge routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.status)
	mux.HandleFunc("/text", s.text)
	mux.HandleFunc("/image", s.image)
	return mux
}

func (s *Server) status(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		methodNotAllowed(response, http.MethodGet)
		return
	}
	left, right := s.client.States()
	writeJSON(response, http.StatusOK, map[string]any{
		"left":  left.String(),
		"right": right.String(),
		"ready": left == g2.Ready && right == g2.Ready,
	})
}

func (s *Server) text(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(response, http.MethodPost)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, s.maxBodyBytes)
	var content string
	if strings.Contains(request.Header.Get("Content-Type"), "application/json") {
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			writeError(response, http.StatusBadRequest, err)
			return
		}
		content = body.Text
	} else {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			writeError(response, http.StatusBadRequest, err)
			return
		}
		content = string(body)
	}
	if content == "" {
		writeError(response, http.StatusBadRequest, fmt.Errorf("text is empty"))
		return
	}
	s.operations.Lock()
	err := s.client.DisplayText(s.ctx, content)
	s.operations.Unlock()
	if err != nil {
		writeError(response, http.StatusBadGateway, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) image(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		methodNotAllowed(response, http.MethodPost)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, s.maxBodyBytes)
	decoded, _, err := image.Decode(request.Body)
	if err != nil {
		writeError(response, http.StatusBadRequest, err)
		return
	}
	s.operations.Lock()
	err = s.client.DisplayImage(s.ctx, decoded)
	s.operations.Unlock()
	if err != nil {
		writeError(response, http.StatusBadGateway, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]bool{"ok": true})
}

func methodNotAllowed(response http.ResponseWriter, allow string) {
	response.Header().Set("Allow", allow)
	writeError(response, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
}

func writeError(response http.ResponseWriter, status int, err error) {
	writeJSON(response, status, map[string]string{"error": err.Error()})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
