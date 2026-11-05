# API REST distribuida – VaiJunto Problema 2

Cada empresa executa `cmd/consortium` em um container independente. O armazenamento JSON indicado por `DATA_FILE` é exclusivo da instância. Não existe banco compartilhado nem coordenador fixo: a empresa que recebe `POST /reservations` coordena somente aquela transação.

Base URL: `http://<empresa>:8081/api/v1`

## Recursos

| Método | Endpoint | Finalidade | Respostas principais |
|---|---|---|---|
| GET | `/health` | Verificar servidor | 200 |
| POST | `/rides` | Cadastrar carona local | 201, 400, 409 |
| GET | `/rides` | Listar caronas locais | 200 |
| GET | `/federated/search?origin=A&destination=B` | Buscar itinerários no consórcio (até 3 caronas) | 200, 400 |
| POST | `/reservations` | Iniciar 2PC | 201 confirmado, 202 commit pendente, 409 sem assento/recusa, 503 participante indisponível, 400 inválido |
| POST | `/transactions/prepare` | Voto do participante | 200 com `VOTE_COMMIT` ou `VOTE_ABORT` |
| POST | `/transactions/commit` | Efetivar preparação | 200, 404, 409 |
| POST | `/transactions/abort` | Desfazer preparação | 200, 409 se já committed |
| GET | `/transactions/{id}` | Consultar decisão/estado | 200, 404 |
| GET | `/reservations/{id}` | Consultar reserva local | 200, 404 |

## Cadastrar carona

```http
POST /api/v1/rides
Content-Type: application/json
```

```json
{
  "id": "C-SAL-FSA-01",
  "driver_id": "motorista-10",
  "route": ["Salvador", "Feira de Santana"],
  "date": "2026-11-10",
  "seats": 3,
  "price_per_segment": 35.0
}
```

## Busca federada de itinerários

O cliente passageiro conecta-se apenas ao servidor da própria empresa. Esse servidor consulta os endereços definidos em `PEERS`, agrega as caronas e procura caminhos de até três trechos, respeitando a sequência de cidades.

```http
GET /api/v1/federated/search?origin=Salvador&destination=Montes%20Claros
```

Resposta simplificada:

```json
{
  "origin": "Salvador",
  "destination": "Montes Claros",
  "servers_consulted": 3,
  "itineraries": [
    {
      "total_price": 135.0,
      "legs": [
        {"server_url":"http://empresa1:8081","ride_id":"C1","origin":"Salvador","destination":"Feira de Santana"},
        {"server_url":"http://empresa2:8081","ride_id":"C7","origin":"Feira de Santana","destination":"Vitoria da Conquista"},
        {"server_url":"http://empresa3:8081","ride_id":"C9","origin":"Vitoria da Conquista","destination":"Montes Claros"}
      ]
    }
  ]
}
```

A busca é consultiva: a disponibilidade pode mudar entre a busca e a reserva. A garantia definitiva é feita novamente no PREPARE pelo servidor proprietário de cada carona.

## Solicitar itinerário com três empresas

O passageiro chama somente o servidor de sua empresa. Cada perna identifica o servidor que é dono da carona. O servidor de entrada agrupa as pernas por empresa e executa PREPARE/COMMIT.

```http
POST /api/v1/reservations
Content-Type: application/json
```

```json
{
  "passenger_id": "passageiro-42",
  "legs": [
    {"server_url":"http://empresa1:8081", "ride_id":"C1", "origin":"Salvador", "destination":"Feira de Santana"},
    {"server_url":"http://empresa2:8081", "ride_id":"C7", "origin":"Feira de Santana", "destination":"Vitoria da Conquista"},
    {"server_url":"http://empresa3:8081", "ride_id":"C9", "origin":"Vitoria da Conquista", "destination":"Montes Claros"}
  ]
}
```

Resposta de sucesso: `201 Created` com `state: COMMITTED`. Se a decisão COMMIT já estiver persistida, mas algum participante estiver temporariamente inacessível, a resposta é `202 Accepted` com `state: COMMIT_PENDING`; o coordenador repete a entrega da decisão em background. Falha no PREPARE resulta em `409 Conflict` (recusa/sem assento) ou `503 Service Unavailable` (participante inacessível), sempre com decisão ABORT persistida.

## Mensagens internas 2PC

PREPARE:

```json
{
  "transaction_id":"id-unico",
  "passenger_id":"passageiro-42",
  "coordinator_url":"http://empresa1:8081",
  "lamport":17,
  "legs":[{"server_url":"http://empresa2:8081","ride_id":"C7","origin":"Feira de Santana","destination":"Vitoria da Conquista"}]
}
```

Resposta do participante: `{"vote":"VOTE_COMMIT","lamport":18}` ou `{"vote":"VOTE_ABORT","lamport":18,"reason":"sem assento disponivel"}`.

COMMIT e ABORT recebem `{"transaction_id":"id-unico"}`.

## Relógio lógico e precedência

Cada mensagem PREPARE, COMMIT e ABORT transporta o contador de Lamport. Ao receber uma mensagem, o participante aplica `L = max(L_local, L_recebido) + 1` e devolve o valor atualizado nas respostas de PREPARE. O estado do contador é persistido junto ao estado local.

O critério de precedência para o último assento é: **vence a solicitação cujo PREPARE for primeiro aceito e registrado na seção crítica do servidor proprietário da carona**. Esse servidor é a autoridade exclusiva sobre seus assentos. A operação de validação e bloqueio ocorre sob mutex, portanto duas requisições concorrentes são serializadas; a primeira grava `PREPARED`, e a segunda encontra o bloqueio e recebe `VOTE_ABORT`. A ordem de chegada é definida no ponto de recebimento do proprietário, não pelo relógio físico do cliente/coordenador, que pode estar dessincronizado ou sofrer atraso de rede. Lamport registra a causalidade e a ordem lógica observada, mas não mede tempo físico nem permite reconstruir a ordem de envio no mundo real.

## Timeouts e recuperação

O cliente HTTP tem timeout de 4 segundos. A preparação grava o estado `PREPARED` e a expiração local antes de votar. Ao expirar, o participante consulta o estado do coordenador. **Não libera unilateralmente** uma preparação cujo resultado seja desconhecido: só aplica COMMIT/ABORT após consultar a decisão. O coordenador persiste COMMIT antes de enviar qualquer mensagem COMMIT; se reiniciar com transação ainda em PREPARING, registra ABORT e retransmite a decisão. Isso evita que um timeout local contradiga uma decisão COMMIT.

O 2PC clássico pode bloquear enquanto o coordenador estiver inacessível e a decisão for desconhecida. A implementação prioriza segurança: mantém o assento preparado até conseguir consultar a decisão. Ela não promete disponibilidade de reserva durante uma partição de rede.

## Execução local

```bash
docker compose up --build -d empresa1 empresa2 empresa3
curl http://localhost:8081/api/v1/health
curl http://localhost:8082/api/v1/health
curl http://localhost:8083/api/v1/health
```

Para cadastrar caronas, use `curl -X POST` em cada empresa conforme o JSON acima. No Docker Compose, as URLs entre containers são `http://empresa1:8081`, `http://empresa2:8081` e `http://empresa3:8081`. Do host, as portas são 8081, 8082 e 8083.

## Limitações desta versão didática

- O estado é persistido em arquivo JSON local, sem banco distribuído. Para laboratório, isso mantém o armazenamento independente; para produção, deve-se usar armazenamento transacional local com `fsync`/WAL e backups.
- Não há autenticação/autorização entre empresas, TLS, descoberta dinâmica de membros nem assinatura das mensagens. Esses itens devem ser adicionados antes de exposição pública.
- A busca federada consulta os membros configurados em `PEERS`, combina até três caronas e não considera ainda compatibilidade de horários de baldeação nem filtros avançados de data. A disponibilidade apresentada é um snapshot; o PREPARE é a validação definitiva.
- O cliente `cmd/passageiro-rest` conecta-se somente à API da empresa do passageiro, busca itinerários e pode reservar um deles. Os clientes TCP do Problema 1 permanecem disponíveis sem alteração.


## Simulador de carga

Depois de iniciar os três servidores e cadastrar caronas (por exemplo, com `sh scripts/demo_distribuido.sh`), execute no host:

```bash
go run ./cmd/simulator -coordinator http://localhost:8081 -owner http://empresa1:8081 -ride C1 -passengers 100
```

O simulador dispara reservas concorrentes e contabiliza HTTP 201/202, HTTP 409 e demais falhas. No host, use URLs publicadas em `localhost` para coordenador e proprietário. Entre containers, use os nomes DNS `empresa1`, `empresa2` e `empresa3`.

## Cliente passageiro REST

Busca sem reservar:

```bash
go run ./cmd/passageiro-rest -server http://localhost:8081 -origin Salvador -destination "Montes Claros"
```

Reserva o primeiro itinerário retornado (índice zero):

```bash
go run ./cmd/passageiro-rest -server http://localhost:8081 -origin Salvador -destination "Montes Claros" -passenger passageiro-42 -reserve 0
```

Dentro do Docker Compose, o cliente opcional pode ser executado com:

```bash
docker compose --profile clientes run --rm passageiro-rest -origin Salvador -destination "Montes Claros" -reserve 0
```

## Execução em computadores distintos no laboratório

No laboratório, cada computador deve executar apenas a instância da empresa que representa. Primeiro, em cada máquina, construa a imagem:

```bash
docker build -f Dockerfile.rest -t vaijunto-rest .
```

Em seguida, execute uma instância por computador, substituindo `IP_EMPRESA1`, `IP_EMPRESA2` e `IP_EMPRESA3` pelos endereços IP alcançáveis na rede do laboratório. Exemplo para a máquina da Empresa 1:

```bash
docker run -d --name empresa1 -p 8081:8081 \
  -e SERVER_ID=empresa1 -e HTTP_ADDR=:8081 \
  -e SELF_URL=http://IP_EMPRESA1:8081 \
  -e PEERS=http://IP_EMPRESA1:8081,http://IP_EMPRESA2:8081,http://IP_EMPRESA3:8081 \
  -e DATA_FILE=/data/empresa1.json \
  -v empresa1_data:/data vaijunto-rest
```

Nas outras máquinas, use `SERVER_ID=empresa2` ou `empresa3` e um volume local próprio. Libere a porta TCP 8081 no firewall de cada máquina. Os endereços `SELF_URL` e `PEERS` devem ser endereços alcançáveis pelos demais containers, não nomes DNS exclusivos de uma máquina.
