# VaiJunto – Problema 2: Consórcio de Caronas Compartilhadas

## 1. Introdução

O Problema 2 amplia o protótipo centralizado do VaiJunto para um consórcio de empresas independentes. Cada organização mantém suas caronas, passageiros e armazenamento, mas precisa permitir que um passageiro monte um itinerário com trechos oferecidos por empresas diferentes. O principal desafio é confirmar todos os trechos como uma única operação lógica, impedindo reservas duplicadas e recuperando o estado após falhas.

## 2. Arquitetura

A solução mantém uma instância REST por empresa. Cada instância executa em container Docker, possui um arquivo JSON persistente próprio e expõe endpoints HTTP/JSON. Não há banco compartilhado. O servidor ao qual o passageiro se conecta coordena somente a transação em questão; em outra reserva, qualquer outra empresa pode atuar como coordenadora.

A busca federada consulta os membros definidos pela variável `PEERS`, agrega caronas e monta itinerários com até três trechos. O resultado da busca é apenas uma sugestão: no momento da reserva, cada servidor proprietário verifica novamente a disponibilidade.

## 3. Atomicidade com Two-Phase Commit

Na primeira fase, PREPARE, o coordenador envia a solicitação a cada servidor proprietário. Cada participante valida o trecho, seleciona assento, registra a transação como `PREPARED` em seu armazenamento local e bloqueia o assento. Se todos votarem `VOTE_COMMIT`, o coordenador grava a decisão COMMIT de forma persistente antes de enviar a segunda fase. Se algum participante votar ABORT ou ficar inacessível, o coordenador persiste ABORT e envia a decisão aos participantes.

Na segunda fase, COMMIT ou ABORT, os participantes efetivam ou desfazem a preparação. As operações são idempotentes: mensagens repetidas não criam reservas adicionais. O coordenador repete a entrega das decisões persistidas durante a recuperação.

O coordenador grava COMMIT antes de enviar qualquer COMMIT. Portanto, depois dessa gravação, ele nunca pode decidir ABORT. Essa regra evita que um participante confirme uma reserva enquanto outro recebe uma decisão contrária.

## 4. Exclusão mútua e precedência

O servidor proprietário é a autoridade sobre os assentos de suas caronas. A verificação de disponibilidade e a gravação de `PREPARED` são realizadas dentro de uma seção crítica protegida por mutex. Assim, duas solicitações simultâneas para o último assento são serializadas no servidor proprietário: vence a solicitação cujo PREPARE for primeiro aceito e registrado nessa seção crítica. A segunda solicitação encontra o assento ocupado ou bloqueado e recebe `VOTE_ABORT`.

Esse critério não compara relógios físicos dos clientes, pois eles podem estar dessincronizados e as mensagens podem atrasar. O ponto de linearização é o registro local no servidor que controla o assento.

## 5. Relógio lógico de Lamport

Cada servidor mantém um contador lógico. Antes de um evento local, incrementa o contador. Ao receber uma mensagem com contador remoto, aplica `L = max(L_local, L_remoto) + 1`. Mensagens de protocolo carregam o contador e as respostas de PREPARE devolvem o valor atualizado. O contador é persistido com o estado do servidor.

Lamport permite registrar causalidade e ordenar eventos relacionados, mas não sincroniza os relógios físicos nem mede o instante real em que o passageiro clicou para reservar.

## 6. Timeout e recuperação

A preparação possui prazo local e o cliente HTTP tem timeout. Quando o prazo de uma preparação expira, o participante consulta a decisão do coordenador. Se o coordenador reiniciar com uma transação ainda em `PREPARING`, converte-a em ABORT de forma persistente. O participante, ao consultar essa decisão, libera o bloqueio. Mensagens ABORT recebidas antes de PREPARE criam um tombstone, impedindo que uma mensagem PREPARE atrasada crie um bloqueio depois da decisão de aborto.

O participante não libera sozinho um assento preparado apenas porque seu timer expirou. Isso seria inseguro: uma decisão COMMIT poderia já estar persistida e a mensagem estar atrasada. Essa é uma limitação conhecida do 2PC clássico: enquanto o coordenador estiver inacessível e a decisão for desconhecida, o participante pode permanecer bloqueado. A prioridade é preservar a segurança; após a recuperação do coordenador, a decisão persistida permite concluir ou desfazer a operação.

## 7. Comparação com o Problema 1

| Aspecto | Problema 1 | Problema 2 |
|---|---|---|
| Organização | Servidor central | Servidores independentes por empresa |
| Comunicação | Protocolo TCP do protótipo | API REST entre empresas |
| Dados | Estado concentrado | Estado local por empresa |
| Reserva multi-trecho | Trava local | Two-Phase Commit |
| Exclusão mútua | Mutex no servidor central | Mutex no servidor proprietário de cada carona |
| Ordenação | Ordem local | Relógio lógico de Lamport e serialização no proprietário |
| Falhas | Falha do servidor central afeta o sistema | Falha afeta transações que dependem daquele participante |

## 8. Testes

A suíte automatizada inclui reserva de três servidores, disputa concorrente pelo último assento, falha de participante durante PREPARE, rejeição de PREPARE atrasado após ABORT, busca federada de itinerário com três trechos, rollback em falha de persistência e recuperação de participante após decisão ABORT do coordenador.

Comandos de validação:

```bash
go test ./...
go test -race ./...
go vet ./...
```

## 9. Limitações e trabalhos futuros

A implementação usa arquivos JSON locais para manter o protótipo simples e sem banco distribuído. Em produção, cada empresa deveria usar um armazenamento transacional local com WAL/fsync, autenticação mútua, TLS, assinatura das mensagens, política de descoberta de membros e observabilidade. A busca atual limita-se a três trechos e não considera duração de viagem ou compatibilidade de horários nas conexões. A garantia de disponibilidade durante uma partição de rede não é prometida pelo 2PC clássico.

## 10. Conclusão

A arquitetura proposta permite que empresas independentes cooperem sem compartilhar seus bancos de dados nem depender de um coordenador permanente. A combinação de REST, 2PC, exclusão mútua local, relógios lógicos, logs persistentes e recuperação oferece uma base demonstrável para reservar itinerários interempresas com decisão atômica e proteção contra dupla ocupação.
