# 🚀 kiro2api Quick Start

## Your Credentials

```bash
KIRO_CLIENT_TOKEN="RcHfY3zpMNXmL4hCSlP1QQZjlZTQ9+LnaVnSbt+xy9A="
```

## Start Service (Choose One)

### Local
```bash
cd /tmp/cc-agent/59364193/project
go build -o kiro2api main.go && ./kiro2api
```

### Docker Compose
```bash
cd /tmp/cc-agent/59364193/project
docker-compose up -d
```

### Docker
```bash
docker run -d --name kiro2api -p 8080:8080 --env-file .env ghcr.io/caidaoli/kiro2api:latest
```

## Quick Tests

### Health Check
```bash
curl http://localhost:8080/v1/models
```

### Test API
```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Authorization: Bearer RcHfY3zpMNXmL4hCSlP1QQZjlZTQ9+LnaVnSbt+xy9A=" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-20250514","max_tokens":50,"messages":[{"role":"user","content":"Hi"}]}'
```

## Claude Code Integration

```bash
export ANTHROPIC_BASE_URL="http://localhost:8080/v1"
export ANTHROPIC_API_KEY="RcHfY3zpMNXmL4hCSlP1QQZjlZTQ9+LnaVnSbt+xy9A="
claude-code "Your question here"
```

## Files Reference

- **`.env`** - Your active configuration
- **`.env.example`** - Configuration template with all options
- **`auth_config.json`** - Your authentication tokens
- **`DEPLOYMENT_CHECKLIST.md`** - Full deployment guide
- **`CONFIGURATION_SUMMARY.txt`** - Setup summary

## Need Help?

1. Check logs: `docker logs -f kiro2api` (Docker) or console output (local)
2. Token status: `curl http://localhost:8080/api/tokens`
3. Documentation: See `README.md` and `CLAUDE.md`

---
✅ Configuration complete and ready to use!
