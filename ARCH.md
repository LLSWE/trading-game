├── cmd/
│   └── server/
│       └── main.go              # Ponto de entrada e montagem do Uber Fx
├── internal/
│   ├── domain/                  # Entidades puras, Value Objects (Money) e Regras de Negócio
│   ├── usecase/                 # Casos de uso de transações, carteiras e reconciliação
│   ├── repository/              # Camada de persistência (pgx + SQL explícito)
│   └── delivery/                # Adaptadores de entrada (HTTP REST e SQS Consumer)
├── pkg/                         # Utilitários genéricos
├── db/migrations/               # Migrations versionadas (SQL puro)
├── docker-compose.yml           # Postgres, Keycloak, LocalStack
└── Dockerfile

ALERTA !!!! :

Zero Pontos Flutuantes: Em hipótese alguma use float64 para o saldo ou valores das apostas. Use int64 (centavos) em tudo no banco e no código, convertendo para string ("25.00") apenas nos limites da API JSON.

O Teste de Concorrência dos R$ 100: Saldo de 100 BRL recebendo duas apostas simultâneas de 80 BRL. Se passar as duas, você reprova na hora. Garanta que o banco faça lock na linha da carteira (SELECT ... FOR UPDATE) no início da transação de débito. A segunda transação deve falhar e ser registrada como rejeitada por saldo insuficiente.

Ledger Imutável (Append-Only): Crie restrições no banco (CHECK constraints) impedindo UPDATE ou DELETE na tabela de ledger. Cada movimentação gera um registro novo.

Outbox Atômico: O evento de outbox tem que ser gravado no banco na mesma transação SQL que altera o saldo da carteira e salva a transação. Nunca publique no SQS antes de dar COMMIT no banco.

Idempotência Real: Se o avaliador mandar a mesma requisição HTTP 50 vezes seguidas em paralelo (teste de race condition de idempotência), o sistema não pode criar 50 débitos. Tem que devolver o resultado cacheado com idempotentReplay: true.

Não use mocks no teste de integração: O desafio exige expressamente que os testes de integração rodem Postgres, Keycloak e LocalStack de verdade via Docker/Testcontainers ou Compose. Não substitua por mocks ou você perde os pontos de testes.
