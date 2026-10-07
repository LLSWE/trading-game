# Distributed Wagering Processing Backend

Distributed Go backend for processing financial transactions in the gambling job market

## Tech Stack

* **Go** (v1.27)
* **LocalStack SQS**
* **Keycloak** 
* **Docker** & **Compose**

## Environment Variables 

Copy .env.example for quick deployment:
```bash
mv .env.example .env
```

## Running Via Docker 

```bash
docker compose up -d --build 
```

Then apply the migrations on ./migrations

## Testing
go test -race -v ./tests/...

