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

## Logging & Tracing

- Logs estruturados com Logrus. Quando um span OTel estiver ativo, os logs incluem a tag `[SPAN]` e os campos `trace_id` e `span_id`.

- Para habilitar tracing configure `OTEL_COLLECTOR_URL`.

## Exemplo `.env` / `.envrc`

```env
LOG_FORMAT=json
LOG_LEVEL=debug
OTEL_COLLECTOR_URL=
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

