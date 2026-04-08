# Trek Demo App

A simple HTTP API that demonstrates Trek SDK integration.

## What This Demo Shows

1. **Trek middleware** automatically extracts request context
2. **Debug logs** only appear when a matching session is active
3. **Session expiration** works automatically

## Running the Demo

### 1. Start the Trek control plane

```bash
cd ../..
docker-compose up -d
```

### 2. Start the demo app

```bash
go run main.go
```

### 3. Make a request (no debug logs)

```bash
curl -H "X-User-ID: user123" http://localhost:3000/api/orders
```

You'll see only INFO level logs.

### 4. Create a debug session

```bash
trek start --user user123 --ttl 5m --level debug --reason "demo"
```

### 5. Make the same request (debug logs appear!)

```bash
curl -H "X-User-ID: user123" http://localhost:3000/api/orders
```

Now you'll see DEBUG level logs for this user's requests.

### 6. Wait or revoke (debug logs stop)

Either wait 5 minutes for TTL expiry, or:

```bash
trek stop --session <session_id>
```

Debug logs will stop appearing for user123.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TREK_SERVICE_NAME` | `demo-api` | Service name for session scoping |
| `TREK_ORG_ID` | `org_demo` | Organization ID |
| `TREK_ENV` | `dev` | Environment |
| `TREK_API_ENDPOINT` | `http://localhost:8080` | Trek server URL |
| `TREK_API_TOKEN` | `demo-token` | Service token |
