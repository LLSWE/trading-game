# Arquitetura e Decisões de Design - Wagering Backend

Este documento descreve as escolhas arquiteturais, padrões de projeto e garantias de consistência implementadas no motor de processamento distribuído de apostas.

## 1. Tratamento e Representação de Dinheiro (`Money` Value Object)
* **Abordagem:** Utilização exclusiva de `int64` para representar valores monetários estritamente em **centavos** (unidades mínimas).
* **Justificativa:** Elimina por completo os erros de arredondamento inerentes ao uso de números de ponto flutuante (`float32`/`float64`), cumprindo rigorosamente os padrões financeiros e as exigências do edital.
* **Contrato Externo:** A serialização e desserialização de JSON ocorrem de forma transparente por meio das interfaces nativas `json.Marshaler` e `json.Unmarshaler`, respeitando estritamente o formato de contrato exigido (`{"amount": "25.00", "currency": "BRL"}`).
* **Validações de Domínio:** Rejeita valores negativos em operações externas de apostas, notação científica, `NaN`, `Infinity` e aplica checagens severas de overflow em `int64` durante o parsing e operações aritméticas.

## 2. Concorrência, Isolamento e Controle de Transações
* **Bloqueio Pessimista (`FOR UPDATE`):** Para garantir isolamento e prevenir perdas de atualizações (*lost updates*) em ambiente distribuído, o sistema utiliza bloqueio pessimista a nível de linha no PostgreSQL na tabela de carteiras (`wallets`) logo no início da transação do caso de uso.
* **Idempotência Persistente:** Cada requisição de aposta calcula um hash SHA-256 canônico dos campos de negócio. A tabela `wagering_transactions` valida a unicidade da chave de idempotency. Replicas exatas retornam o payload persistido com `idempotentReplay: true`, enquanto colisões de payload com a mesma chave retornam conflito HTTP 409.

## 3. Mensageria Confiável: Outbox e Inbox Patterns
* **Transactional Outbox:** Os eventos gerados (como `WagerTransactionProcessed`) são gravados na tabela `outbox_events` atomicamente na mesma transação de banco que altera o saldo e o ledger. Um worker em background lê os eventos pendentes utilizando `FOR UPDATE SKIP LOCKED` e os publica em uma fila SQS FIFO (`wager-transactions.fifo`), garantindo entrega durável.
* **Inbox Pattern:** Na ponta de consumo, a tabela `inbox_messages` restringe a unicidade do par `(consumer_name, message_id)`, assegurando que reentregas de mensagens via SQS (*at-least-once*) não causem efeitos colaterais duplicados.

## 4. Imutabilidade do Ledger
* O ledger financeiro (`wallet_ledger_entries`) é estritamente *append-only*. O banco de dados protege a integridade dos registros através de triggers em PL/pgSQL que bloqueiam qualquer tentativa de alteração (`UPDATE`) ou exclusão (`DELETE`).

## 5. Composição com Uber Fx e Ciclo de Vida
* **Injeção de Dependências:** O framework `go.uber.org/fx` organiza e injeta de forma modular o pool de conexões (`pgxpool`), repositórios, casos de uso, handlers HTTP e workers em background.
* **Graceful Shutdown:** Através da interface `fx.Lifecycle`, a aplicação gerencia o cancelamento limpo de contextos (`context.WithCancel`), permitindo que servidores e workers finalizem tarefas em andamento de forma segura antes do encerramento dos processos.
