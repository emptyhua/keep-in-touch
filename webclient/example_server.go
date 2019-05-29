package main

import (
	"fmt"
	"net/http"

	"github.com/emptyhua/keep-in-touch"
)

type MyApp struct {
}

type HelloReq struct {
	kit.RequestHead
	Msg string `json:"msg"`
}

type PushData struct {
	Msg string `json:"msg"`
}

func (app *MyApp) Echo(s *kit.Session, req *HelloReq) {
	fmt.Println("recev echo req:", req)
	s.Push("chat", &PushData{Msg: "welcome"})
	s.Response(req, req)
}

func main() {
	kit.SetDebug(true)
	route := kit.NewRoute()

	route.Reg("m", &MyApp{})

	kitServer := kit.NewServer(route)

	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(http.Dir("./")))
	mux.Handle("/websocket", kitServer)

	port := "12345"
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	fmt.Println("http started at port " + port)

	err := httpServer.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
