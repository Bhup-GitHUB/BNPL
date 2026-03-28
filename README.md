# BNPL Instant Approval Prototype Backend

## Run

Set `DATABASE_URL` and start the service:

```bash
go run ./cmd/server
```

The gRPC server listens on `APP_PORT`, default `9090`.

## Environment

- `DATABASE_URL`
- `APP_PORT`
- `REQUEST_TIMEOUT_MS`
- `BUREAU_TIMEOUT_MS`
- `AA_TIMEOUT_MS`
- `FRAUD_TIMEOUT_MS`
- `FEATURE_LATENCY_MS`
- `BUREAU_LATENCY_MS`
- `AA_LATENCY_MS`
- `FRAUD_LATENCY_MS`
- `APPROVE_SCORE`
- `REJECT_SCORE`

## Tests

```bash
go test ./...
```

Set `TEST_DATABASE_URL` to run the Postgres integration test.
