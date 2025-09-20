# go-exchange

Motor simples de consulta de cotações em Go.

Este repositório contém uma API HTTP para conversão de moedas. Principais características:

- Cache em Redis
- Suporte a fee (percentual configurável via variável de ambiente ou serviço externo)
- Logs com Logrus e integração opcional com OpenTelemetry (OTLP HTTP)

## Quick start

1. Configure o ambiente usando o arquivo `.envrc` de exemplo.
2. Inicie os serviços:

```sh
docker compose up --build
```

## Endpoints

- GET `/convert?from=USD&to=BRL&amount=1000`
  - `amount` em centavos (1000 => 10.00)

- GET `/convert?from=USD&to=BRL&amount=10.00`
  - `amount` em unidades decimais (10.00)

## Environment variables

As variáveis de ambiente podem ser carregadas com direnv (veja `.envrc`). Principais variáveis:

- `HTTP_ADDR` (default `:8080`)
- `REDIS_ADDR` (default `localhost:6379`)
- `REDIS_DB` (default `0`)
- `CACHE_TTL` (default `5m`)
- `EXCHANGE_PROVIDER` (default `exchangerate.host`)
- `EXCHANGE_FEE_PERCENT` (default `0.0`) — ex.: `0.005` = 0.5%
- `FEE_API_URL` (opcional: URL que retorna JSON `{ "percent": 0.005 }`)
- `LOG_FORMAT` (`text` ou `json`, default: `text`)
- `LOG_LEVEL` (`info`, `debug`, `warn`, `error`)
- `OTEL_COLLECTOR_URL` (opcional: endpoint OTLP HTTP)
- `OTEL_COLLECTOR_URL` (opcional: endpoint OTLP HTTP or gRPC)
- `APP_NAME` (opcional: nome da aplicação, default: go-exchange)
- `APP_VERSION` (opcional: versão do serviço, ex.: 1.2.3)
- `APP_ENV` (opcional: ambiente, ex.: development, staging, production)
- `OTEL_COLLECTOR_URL` (opcional: endpoint OTLP HTTP or gRPC)
- `OTLP_ENDPOINT` (opcional: endpoint OTLP explícito que sobrescreve `OTEL_COLLECTOR_URL`)
- `OTLP_HEADERS` (opcional: cabeçalhos enviados aos exporters OTLP, formato: `KEY=VALUE,Other=Value`)
- `OTLP_USE_TLS` (opcional: `true`/`false` para usar TLS; quando `false` será usado modo inseguro)
- `OTLP_TLS_CA_PATH`, `OTLP_TLS_CERT_PATH`, `OTLP_TLS_KEY_PATH` (opcional: caminhos para CA e client cert/key para TLS/mTLS)
- `OTLP_INSECURE_SKIP_VERIFY` (opcional: `true` para pular verificação do certificado TLS do collector — use com cautela)

## Logging & Tracing

- Logs estruturados com Logrus. Quando um span OTel estiver ativo, os logs incluem a tag `[SPAN]` e os campos `trace_id` e `span_id`.

- Para habilitar tracing configure `OTEL_COLLECTOR_URL`.

Recomendação de inicialização:

- Primeiro crie o logger (por exemplo `logger.New(...)`).
- Em seguida chame `logger.SetupTelemetry(ctx, cfg)` para registrar hooks/formatters — isso garante que os hooks que adicionam eventos ou badges aos logs já estejam presentes antes de qualquer log de startup. `cfg` é o `*config.Config` carregado pela sua aplicação; o logger preferirá `AppVersion`/`AppEnv` vindos do `cfg`.
- Por fim, se for usar OTLP, chame `logger.SetupOTel(ctx, cfg)` após `SetupTelemetry` para configurar tracer/meter/logger providers com metadados do serviço. A função retorna um `shutdown` e uma lista com informações sobre os exporters criados (útil para diagnóstico); chame o `shutdown` em encerramento.

Notas OTLP e TLS:

- `OTLP_HEADERS` deve ser uma lista separada por vírgulas de pares `KEY=VALUE`. Exemplo: `OTLP_HEADERS=Authorization=Bearer abc123,X-Api-Key=secret`.
- Se `OTLP_USE_TLS=true`, o código tentará carregar `OTLP_TLS_CA_PATH` para validar o servidor; se `OTLP_TLS_CERT_PATH` e `OTLP_TLS_KEY_PATH` estiverem setados, também será carregado client cert/key para mTLS.
- Em alguns ambientes é desejável pular a verificação TLS (dev), use `OTLP_INSECURE_SKIP_VERIFY=true`, mas evite em produção.

Nota sobre esquema vs configuração TLS:

- Se o `OTLP_ENDPOINT` incluir um esquema explícito (`http://` ou `https://`) e houver inconsistência com `OTLP_USE_TLS`, o aplicativo irá registrar um WARNING indicando o mismatch. A configuração `OTLP_USE_TLS` (boolean) é a que determina o comportamento final (ou seja, ela prevalece sobre o esquema do endpoint).

## Exemplo `.env` / `.envrc`

```env
LOG_FORMAT=json
LOG_LEVEL=debug
OTEL_COLLECTOR_URL=
APP_NAME=go-exchange
APP_VERSION=0.0.0
APP_ENV=development

# OTLP examples
# OTLP HTTP collector (commonly used):
# OTEL_COLLECTOR_URL=http://collector:4318
# OTLP gRPC collector (use host:port or grpc://host:4317):
# OTEL_COLLECTOR_URL=collector:4317
# or
# OTEL_COLLECTOR_URL=grpc://collector:4317
```

## Exemplo de resposta JSON

```json
{
  "from": "USD",
  "to": "BRL",
  "amount_cents": 1000,
  "result_cents": 50325,
  "result": 503.25,
  "fee_percent": 0.005,
  "fee_amount_cents": 252,
  "net_result_cents": 50073,
  "net_result": 500.73
}
```

## Extras

- Para carregar variáveis de ambiente automaticamente: instale [direnv](https://direnv.net/) e execute `direnv allow`.
- Documentação OpenTelemetry: [OpenTelemetry](https://opentelemetry.io/)

