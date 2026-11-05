// simulator gera requisicoes concorrentes de passageiros para medir disputa
// por assentos nos servidores REST do consorcio.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type leg struct {
	ServerURL   string `json:"server_url"`
	RideID      string `json:"ride_id"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
}
type request struct {
	PassengerID string `json:"passenger_id"`
	Legs        []leg  `json:"legs"`
}

func main() {
	coordinator := flag.String("coordinator", "http://localhost:8081", "URL do servidor ao qual os passageiros se conectam")
	owner := flag.String("owner", "http://localhost:8081", "URL do servidor dono da carona, visivel ao coordenador")
	ride := flag.String("ride", "C1", "ID da carona")
	origin := flag.String("origin", "Salvador", "origem")
	destination := flag.String("destination", "Feira de Santana", "destino")
	passengers := flag.Int("passengers", 100, "quantidade de passageiros concorrentes")
	flag.Parse()
	client := &http.Client{Timeout: 8 * time.Second}
	var wg sync.WaitGroup
	var committed, pending, conflict, other int64
	start := time.Now()
	for i := 0; i < *passengers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body, _ := json.Marshal(request{PassengerID: fmt.Sprintf("sim-passageiro-%06d", i), Legs: []leg{{ServerURL: *owner, RideID: *ride, Origin: *origin, Destination: *destination}}})
			resp, err := client.Post(*coordinator+"/api/v1/reservations", "application/json", bytes.NewReader(body))
			if err != nil {
				atomic.AddInt64(&other, 1)
				return
			}
			defer resp.Body.Close()
			switch resp.StatusCode {
			case 201:
				atomic.AddInt64(&committed, 1)
			case 202:
				atomic.AddInt64(&pending, 1)
			case 409:
				atomic.AddInt64(&conflict, 1)
			default:
				atomic.AddInt64(&other, 1)
			}
		}(i)
	}
	wg.Wait()
	fmt.Printf("passageiros=%d confirmadas=%d commit_pendente=%d recusadas=%d outras_falhas=%d duracao=%s\n", *passengers, committed, pending, conflict, other, time.Since(start))
}
