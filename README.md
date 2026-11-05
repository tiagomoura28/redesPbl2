# VAIJUNTO — Sistema de Caronas Compartilhadas

Projeto acadêmico desenvolvido em Go para estudar comunicação em rede, concorrência e coordenação de reservas. O repositório reúne duas etapas do trabalho, executadas separadamente:

- **Problema 1:** aplicação cliente-servidor centralizada, com comunicação por sockets TCP/IP.
- **Problema 2:** consórcio de empresas independentes, com API REST, coordenação distribuída por Two-Phase Commit (2PC) e relógios lógicos de Lamport.

O Problema 2 foi adicionado em paralelo para preservar a implementação e os clientes TCP do Problema 1.

## Sumário

- [Tecnologias](#tecnologias)
- [Estrutura do projeto](#estrutura-do-projeto)
- [Problema 1 — Sistema TCP](#problema-1--sistema-tcp)
- [Problema 2 — Consórcio distribuído](#problema-2--consórcio-distribuído)
- [Testes](#testes)
- [Documentação complementar](#documentação-complementar)
- [Limitações conhecidas](#limitações-conhecidas)

## Tecnologias

- Go (versão indicada em `go.mod`).
- TCP/IP e protocolo textual próprio no Problema 1.
- HTTP/REST e mensagens JSON no Problema 2.
- Docker e Docker Compose para empacotamento e execução local.
- Arquivos JSON locais para persistência das instâncias REST.
- Testes automatizados da linguagem Go.

Não são utilizados frameworks de mensageria ou RPC para a comunicação entre os servidores.

## Estrutura do projeto

```text
.
├── cmd/
│   ├── server/             # Servidor central TCP (Problema 1)
│   ├── motorista/          # Cliente motorista TCP
│   ├── passageiro/         # Cliente passageiro TCP
│   ├── webui/              # Interface web do Problema 1
│   ├── consortium/         # Servidor REST de cada empresa (Problema 2)
│   ├── passageiro-rest/    # Cliente passageiro REST
│   └── simulator/          # Simulador de reservas concorrentes
├── internal/
│   ├── model/              # Modelos do Problema 1
│   ├── proto/              # Protocolo textual TCP
│   ├── server/             # Atendimento das conexões TCP
│   ├── store/              # Estado e regras do Problema 1
│   ├── clientio/           # Comunicação dos clientes TCP
│   └── consortium/         # API, estado e coordenação 2PC
├── docs/
│   ├── API_DISTRIBUIDA.md
│   ├── SEQUENCIA_2PC.md
│   └── RELATORIO_TECNICO_PROBLEMA2.md
├── scripts/
│   └── demo_distribuido.sh
├── docker-compose.yml
└── go.mod
```

## Problema 1 — Sistema TCP

### Visão geral

O Problema 1 implementa um sistema de caronas compartilhadas com um servidor central. Os clientes motorista e passageiro comunicam-se com esse servidor por sockets TCP/IP, utilizando um protocolo de aplicação textual próprio.

O servidor mantém os dados de usuários, caronas e reservas em memória. A interface web é uma alternativa de acesso e encaminha as operações ao mesmo servidor central.

### Componentes

| Componente | Responsabilidade |
|---|---|
| `cmd/server` | Recebe conexões TCP e processa solicitações |
| `cmd/motorista` | Publica, consulta e cancela caronas |
| `cmd/passageiro` | Busca itinerários e gerencia reservas |
| `cmd/webui` | Disponibiliza uma interface web |

### Executar localmente sem Docker

É necessário ter Go instalado. Na raiz do projeto, execute cada comando em um terminal separado.

Servidor:

```bash
go run ./cmd/server -addr :9000
```

Cliente motorista:

```bash
go run ./cmd/motorista -addr localhost:9000
```

Cliente passageiro:

```bash
go run ./cmd/passageiro -addr localhost:9000
```

Interface web (com o servidor TCP em execução):

```bash
go run ./cmd/webui -addr localhost:9000 -listen :8080
```

Acesse `http://localhost:8080` no navegador.

### Executar com Docker Compose

Para iniciar o servidor TCP e a interface web:

```bash
docker compose up --build -d server webui
```

A interface fica disponível em `http://localhost:8080`.

Para abrir os clientes de terminal:

```bash
docker compose run --rm motorista
docker compose run --rm passageiro
```

O Compose deste repositório foi configurado para execução na mesma máquina. Para clientes em computadores diferentes, publique a porta do servidor e configure `VAIJUNTO_ADDR` com o IP e a porta acessíveis do computador que executa o servidor.

## Problema 2 — Consórcio distribuído

### Objetivo

No Problema 2, cada empresa mantém seu próprio servidor e seus próprios dados. O passageiro acessa o servidor de sua empresa, que consulta os demais participantes para localizar itinerários compostos por caronas de empresas diferentes.

Quando uma reserva envolve mais de uma empresa, os servidores participantes coordenam a operação utilizando **Two-Phase Commit (2PC)**. O objetivo é impedir que apenas parte de um itinerário seja confirmada.

### Arquitetura implementada

Cada instância REST possui:

- Identificador próprio (`SERVER_ID`).
- Endereço HTTP (`HTTP_ADDR`) e endereço anunciado aos participantes (`SELF_URL`).
- Arquivo JSON local definido por `DATA_FILE`.
- Lista de servidores conhecidos definida por `PEERS`.
- Estado das transações e contador lógico de Lamport.

O servidor que recebe a solicitação do passageiro coordena aquela transação específica. Não existe um coordenador central fixo no consórcio.

**Importante:** a configuração Docker Compose incluída sobe **três empresas na mesma máquina**. Ela não configura automaticamente cinco empresas nem cinco computadores físicos. A seção de execução distribuída explica como configurar instâncias em computadores diferentes usando seus endereços IP reais.

### Fluxo resumido da reserva

1. O passageiro solicita a busca de itinerários ao servidor de sua empresa.
2. O servidor consulta os endereços configurados em `PEERS` e reúne as opções encontradas.
3. O passageiro envia a solicitação de reserva ao servidor ao qual está conectado.
4. O coordenador envia mensagens `PREPARE` aos servidores proprietários das caronas envolvidas.
5. Cada participante verifica a disponibilidade, registra sua preparação ou vota pela recusa.
6. Se todos votarem pela confirmação, o coordenador registra a decisão `COMMIT` e a distribui aos participantes.
7. Se algum participante recusar ou ficar indisponível durante a preparação, o coordenador registra `ABORT` e comunica a decisão.

O servidor proprietário de cada carona é a autoridade sobre os assentos daquela carona. O relógio de Lamport acompanha a ordem lógica dos eventos e mensagens; ele não sincroniza relógios físicos nem determina sozinho a ordem real de chegada das solicitações.

### API REST

A documentação completa dos campos JSON e exemplos de requisições e respostas está em [`docs/API_DISTRIBUIDA.md`](docs/API_DISTRIBUIDA.md).

| Método | Endpoint | Finalidade |
|---|---|---|
| `GET` | `/api/v1/health` | Verificar a disponibilidade do servidor |
| `POST` | `/api/v1/rides` | Cadastrar uma carona na empresa |
| `GET` | `/api/v1/rides` | Listar caronas locais |
| `GET` | `/api/v1/federated/search` | Buscar itinerários entre empresas |
| `POST` | `/api/v1/reservations` | Solicitar reserva coordenada por 2PC |
| `POST` | `/api/v1/transactions/prepare` | Preparar participação em uma transação |
| `POST` | `/api/v1/transactions/commit` | Confirmar uma transação preparada |
| `POST` | `/api/v1/transactions/abort` | Abortar uma transação |
| `GET` | `/api/v1/transactions/{id}` | Consultar o estado de uma transação |
| `GET` | `/api/v1/reservations/{id}` | Consultar uma reserva |

### Executar três empresas com Docker Compose

Pré-requisitos: Docker e Docker Compose instalados.

Na raiz do projeto:

```bash
docker compose up --build -d empresa1 empresa2 empresa3
```

Verifique as três instâncias:

```bash
curl http://localhost:8081/api/v1/health
curl http://localhost:8082/api/v1/health
curl http://localhost:8083/api/v1/health
```

Mapeamento das portas no computador hospedeiro:

| Empresa | Endereço no host | Porta no container |
|---|---|---|
| Empresa 1 | `http://localhost:8081` | `8081` |
| Empresa 2 | `http://localhost:8082` | `8081` |
| Empresa 3 | `http://localhost:8083` | `8081` |

Dentro da rede do Compose, os serviços comunicam-se pelos nomes `empresa1`, `empresa2` e `empresa3`, todos na porta `8081`.

Para encerrar os serviços:

```bash
docker compose down
```

Os dados ficam em volumes Docker separados. Para apagar também os dados persistidos nos volumes, use `docker compose down -v`.

### Cadastrar uma carona de teste

Exemplo de cadastro na Empresa 1:

```bash
curl -X POST http://localhost:8081/api/v1/rides \
  -H "Content-Type: application/json" \
  -d '{
    "id": "C-SAL-FSA-01",
    "driver_id": "motorista-10",
    "route": ["Salvador", "Feira de Santana"],
    "date": "2026-11-10",
    "seats": 3,
    "price_per_segment": 35.0
  }'
```

Repita a operação nas outras empresas, alterando o endereço e os dados da carona.

### Buscar itinerários

Exemplo de busca federada a partir da Empresa 1:

```bash
curl "http://localhost:8081/api/v1/federated/search?origin=Salvador&destination=Montes%20Claros"
```

A busca federada apresenta as opções encontradas naquele momento. A confirmação definitiva dos assentos ocorre novamente durante a etapa `PREPARE` da reserva.

### Usar o cliente passageiro REST

Para consultar itinerários:

```bash
go run ./cmd/passageiro-rest \
  -server http://localhost:8081 \
  -origin Salvador \
  -destination "Montes Claros"
```

Para reservar o primeiro itinerário retornado:

```bash
go run ./cmd/passageiro-rest \
  -server http://localhost:8081 \
  -origin Salvador \
  -destination "Montes Claros" \
  -passenger passageiro-42 \
  -reserve 0
```

Também é possível executar o cliente pelo perfil opcional do Compose:

```bash
docker compose --profile clientes run --rm passageiro-rest \
  -origin Salvador \
  -destination "Montes Claros" \
  -reserve 0
```

### Simular concorrência

Com os servidores em execução e uma carona cadastrada, execute:

```bash
go run ./cmd/simulator \
  -coordinator http://localhost:8081 \
  -owner http://localhost:8081 \
  -ride C-SAL-FSA-01 \
  -origin Salvador \
  -destination "Feira de Santana" \
  -passengers 100
```

O simulador envia solicitações concorrentes e apresenta a quantidade de respostas confirmadas, pendentes, recusadas e outras falhas.

### Executar empresas em computadores diferentes

O `docker-compose.yml` utiliza nomes DNS internos do Compose e foi pensado para uma única máquina. Para distribuir as empresas por computadores diferentes, cada computador deve executar sua própria instância REST, com armazenamento local independente.

Em cada computador, construa a imagem:

```bash
docker build -f Dockerfile.rest -t vaijunto-rest .
```

Exemplo para a máquina da Empresa 1:

```bash
docker run -d --name empresa1 \
  -p 8081:8081 \
  -e SERVER_ID=empresa1 \
  -e HTTP_ADDR=:8081 \
  -e SELF_URL=http://IP_EMPRESA1:8081 \
  -e PEERS=http://IP_EMPRESA1:8081,http://IP_EMPRESA2:8081,http://IP_EMPRESA3:8081 \
  -e DATA_FILE=/data/empresa1.json \
  -v empresa1_data:/data \
  vaijunto-rest
```

Nas demais máquinas, altere `SERVER_ID`, `SELF_URL`, `DATA_FILE` e o volume local. A variável `PEERS` deve conter os endereços IP alcançáveis de todos os participantes. Substitua os nomes `IP_EMPRESA1`, `IP_EMPRESA2` e `IP_EMPRESA3` pelos IPs reais da rede e libere a porta `8081` no firewall de cada computador.

### Demonstração automatizada

Com as três empresas iniciadas, execute:

```bash
sh scripts/demo_distribuido.sh
```

O script executa o fluxo de demonstração definido no próprio arquivo.

## Testes

Na raiz do projeto:

```bash
go test ./... -v
```

Para detectar condições de corrida:

```bash
go test -race ./...
```

Para análise estática:

```bash
go vet ./...
```

Execute esses comandos no ambiente de desenvolvimento para verificar o estado atual do código após alterações.

## Documentação complementar

- [`docs/API_DISTRIBUIDA.md`](docs/API_DISTRIBUIDA.md) — endpoints REST, estruturas JSON, relógio lógico, timeouts e recuperação.
- [`docs/SEQUENCIA_2PC.md`](docs/SEQUENCIA_2PC.md) — diagramas de sequência do protocolo 2PC.
- [`docs/RELATORIO_TECNICO_PROBLEMA2.md`](docs/RELATORIO_TECNICO_PROBLEMA2.md) — descrição técnica da solução do Problema 2.

## Limitações conhecidas

- **Problema 1:** os dados do servidor TCP central são mantidos em memória; reiniciar o processo reinicia o estado.
- **Problema 2:** cada instância persiste seus dados em arquivo JSON local; não existe banco de dados compartilhado.
- **Topologia do Compose:** a configuração incluída demonstra três instâncias REST na mesma máquina. A execução em computadores separados exige configurar manualmente IPs, portas, firewall e `PEERS`.
- **Número de empresas:** a configuração atual demonstra três empresas. Uma topologia com cinco empresas exige adicionar e configurar duas instâncias e atualizar a lista de participantes em todas elas.
- **2PC:** se o coordenador ficar inacessível enquanto a decisão ainda for desconhecida, uma transação preparada pode permanecer bloqueada até que seja possível consultar a decisão. O timeout detecta indisponibilidade, mas não elimina a característica de bloqueio do 2PC clássico.
- **Segurança:** a versão acadêmica não implementa autenticação entre empresas nem TLS. Não deve ser exposta diretamente à Internet sem controles adicionais.
- **Busca:** a busca federada retorna itinerários de até três caronas e possui as limitações de compatibilidade de horários descritas na documentação da API.

---

Projeto acadêmico — VAIJUNTO.
