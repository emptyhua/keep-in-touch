package kit

import (
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

func NewServer(route *Route) *Server {
	server := &Server{
		SessionManager:    NewSessionManager(),
		Route:             route,
		HeartbeatInterval: 5 * time.Second,
	}

	go server.SessionManager.CheckExpire()
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(_ *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), 500)
		Logger.Errorf("websocket upgrade failure, URI=%s, Error=%v", r.RequestURI, err)
		return
	}

	c, err := newWSConn(conn)
	if err != nil {
		http.Error(w, err.Error(), 500)
		Logger.Errorf("newWSConn error %v", err)
		return
	}

	kitConn := NewKitConn(s, c)
	kitConn.Handle()
}

func (s *Server) RunWebSocketServer(path string, port int) {
	mux := http.NewServeMux()
	mux.HandleFunc(path, s.ServeHTTP)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
