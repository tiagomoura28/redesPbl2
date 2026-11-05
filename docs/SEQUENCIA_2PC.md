# Diagramas de sequência – VaiJunto

## Reserva confirmada

```mermaid
sequenceDiagram
    autonumber
    actor P as Passageiro
    participant S1 as Empresa 1 / Coordenador
    participant S2 as Empresa 2
    participant S3 as Empresa 3
    P->>S1: POST /api/v1/reservations
    S1->>S1: Cria transaction_id e persiste PREPARING
    Note over S1,S3: Fase 1 – PREPARE
    S1->>S2: POST /transactions/prepare (Lamport)
    S2->>S2: Atualiza Lamport, valida e bloqueia assento
    S2-->>S1: VOTE_COMMIT
    S1->>S3: POST /transactions/prepare (Lamport)
    S3->>S3: Atualiza Lamport, valida e bloqueia assento
    S3-->>S1: VOTE_COMMIT
    S1->>S1: Persiste decisão COMMIT
    Note over S1,S3: Fase 2 – COMMIT
    S1->>S2: POST /transactions/commit
    S2->>S2: Grava ocupação confirmada
    S2-->>S1: COMMITTED
    S1->>S3: POST /transactions/commit
    S3->>S3: Grava ocupação confirmada
    S3-->>S1: COMMITTED
    S1-->>P: HTTP 201 – itinerário confirmado
```

## Falha durante PREPARE

```mermaid
sequenceDiagram
    autonumber
    actor P as Passageiro
    participant S1 as Empresa 1 / Coordenador
    participant S2 as Empresa 2
    participant S3 as Empresa 3
    P->>S1: POST /reservations
    S1->>S2: PREPARE
    S2-->>S1: VOTE_COMMIT
    S1->>S3: PREPARE
    S3-->>S1: VOTE_ABORT (sem assento)
    S1->>S1: Persiste ABORT
    S1->>S2: POST /transactions/abort
    S2->>S2: Marca ABORTED e libera bloqueio
    S2-->>S1: ABORTED
    S1-->>P: HTTP 409 – reserva não realizada
```

## Timeout e recuperação

```mermaid
sequenceDiagram
    autonumber
    participant S1 as Coordenador
    participant S2 as Participante
    S1->>S2: PREPARE
    S2->>S2: Persiste PREPARED com prazo
    S2-->>S1: VOTE_COMMIT
    S1--xS2: COMMIT/ABORT sofre timeout ou conexão cai
    Note over S2: Prazo expirou; não libera por conta própria
    S2->>S1: GET /transactions/{id}
    alt Coordenador decidiu COMMIT
        S1-->>S2: COMMIT
        S2->>S2: Efetiva reserva
    else Coordenador decidiu ABORT
        S1-->>S2: ABORT
        S2->>S2: Libera bloqueio
    else Coordenador indisponível
        S1--xS2: Sem resposta
        Note over S2: Mantém PREPARED até recuperar decisão
    end
```

## Busca federada antes da reserva

```mermaid
sequenceDiagram
    autonumber
    actor P as Passageiro
    participant S1 as Servidor da empresa do passageiro
    participant S2 as Empresa 2
    participant S3 as Empresa 3
    P->>S1: GET /api/v1/federated/search?origin=A&destination=D
    S1->>S1: Consulta caronas locais
    S1->>S2: GET /api/v1/rides
    S2-->>S1: Caronas e ocupacao local
    S1->>S3: GET /api/v1/rides
    S3-->>S1: Caronas e ocupacao local
    S1->>S1: Monta itinerarios de ate 3 trechos
    S1-->>P: 200 - lista de itinerarios e pernas
    Note over P,S3: A disponibilidade e revalidada no PREPARE da reserva
```
