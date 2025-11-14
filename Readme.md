```bash
curl -X POST http://localhost:8080/submit   -H "Content-Type: application/json"   -d '{
    "first_name": "John",
    "last_name": "Doe",
    "email": "john@example.com"
  }'
```

## Docker run

```bash
docker build -t virtudocks-back .
```

```bash
mkdir -p ./data
```

```bash
docker run --rm -p 8080:8080 \
  -e EXPORT_PASSWORD="supersecret" \
  -v "$(pwd)/data:/app" \
  virtudocks-back
```
