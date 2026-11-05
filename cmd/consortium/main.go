package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"vaijunto/internal/consortium"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func main() {
	id := env("SERVER_ID", "empresa1")
	addr := env("HTTP_ADDR", ":8081")
	self := env("SELF_URL", "http://localhost"+addr)
	data := env("DATA_FILE", "data/"+id+".json")
	peerText := env("PEERS", self)
	peers := []string{}
	for _, p := range strings.Split(peerText, ",") {
		if p = strings.TrimSpace(p); p != "" {
			peers = append(peers, p)
		}
	}
	s, err := consortium.New(consortium.Config{ServerID: id, SelfURL: self, DataFile: data, LockTTL: 30 * time.Second, Peers: peers})
	if err != nil {
		log.Fatal(err)
	}
	stop := make(chan struct{})
	go s.Recover(stop)
	log.Printf("Servidor REST %s ouvindo em %s; URL=%s", id, addr, self)
	srv := &http.Server{Addr: addr, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(srv.ListenAndServe())
}
