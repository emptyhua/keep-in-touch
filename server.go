package kit

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type Server struct {
	HeartbeatInterval time.Duration
	SessionManager    *SessionManager
	Route             *Route
}

func (s *Server) setup() {
	if s.Route == nil {
		panic(errors.New("Serve.Route missed"))
	}

	if s.SessionManager == nil {
		s.SessionManager = NewSessionManager()
		go s.SessionManager.CheckExpire()
	}

	if s.HeartbeatInterval == 0 {
		s.HeartbeatInterval = 5 * time.Second
	}
}

func (s *Server) HandleWebSocket(mux *http.ServeMux, path string) {
	s.setup()

	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(_ *http.Request) bool { return true },
	}

	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			Logger.Errorf("websocket upgrade failure, URI=%s, Error=%v", r.RequestURI, err)
			return
		}

		c, err := newWSConn(conn)
		if err != nil {
			Logger.Errorf("newWSConn error %v", err)
			return
		}

		kitConn := NewKitConn(s, c)
		kitConn.Handle()
	})
}

func (s *Server) RunWebSocketServer(path string, port int) {
	s.setup()

	mux := http.NewServeMux()
	s.HandleWebSocket(mux, path)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
