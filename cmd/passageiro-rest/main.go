// passageiro-rest e um cliente de demonstracao do Problema 2. Ele se conecta
// somente ao servidor da propria empresa; esse servidor consulta o consorcio.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Leg struct {
	ServerURL   string `json:"server_url"`
	RideID      string `json:"ride_id"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
}
type Itinerary struct {
	Legs       []Leg   `json:"legs"`
	TotalPrice float64 `json:"total_price"`
}
type SearchResponse struct {
	Origin      string      `json:"origin"`
	Destination string      `json:"destination"`
	Itineraries []Itinerary `json:"itineraries"`
	Servers     int         `json:"servers_consulted"`
}

func main() {
	serverDefault := os.Getenv("VAIJUNTO_REST_URL")
	if serverDefault == "" {
		serverDefault = "http://localhost:8081"
	}
	server := flag.String("server", serverDefault, "API da empresa do passageiro")
	origin := flag.String("origin", "Salvador", "cidade de origem")
	destination := flag.String("destination", "Montes Claros", "cidade de destino")
	passenger := flag.String("passenger", "passageiro-demo", "identificador do passageiro")
	reserve := flag.Int("reserve", -1, "indice do itinerario a reservar (0-based); -1 apenas busca")
	flag.Parse()
	base := strings.TrimRight(*server, "/")
	client := &http.Client{Timeout: 10 * time.Second}
	q := url.Values{"origin": {*origin}, "destination": {*destination}}
	resp, err := client.Get(base + "/api/v1/federated/search?" + q.Encode())
	if err != nil {
		fail(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fail(fmt.Errorf("busca retornou HTTP %d", resp.StatusCode))
	}
	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fail(err)
	}
	if len(result.Itineraries) == 0 {
		fmt.Println("Nenhum itinerario encontrado.")
		return
	}
	fmt.Printf("Itinerarios %s -> %s (servidores consultados: %d)\n", result.Origin, result.Destination, result.Servers)
	for i, it := range result.Itineraries {
		fmt.Printf("[%d] R$ %.2f\n", i, it.TotalPrice)
		for _, l := range it.Legs {
			fmt.Printf("    %s: %s -> %s (carona %s)\n", l.ServerURL, l.Origin, l.Destination, l.RideID)
		}
	}
	if *reserve < 0 {
		return
	}
	if *reserve >= len(result.Itineraries) {
		fail(fmt.Errorf("indice de itinerario invalido"))
	}
	body, _ := json.Marshal(map[string]any{"passenger_id": *passenger, "legs": result.Itineraries[*reserve].Legs})
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/reservations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	book, err := client.Do(req)
	if err != nil {
		fail(err)
	}
	defer book.Body.Close()
	fmt.Printf("Reserva: HTTP %d\n", book.StatusCode)
	var out map[string]any
	if err := json.NewDecoder(book.Body).Decode(&out); err == nil {
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, "erro:", err); os.Exit(1) }
