# kiro2api Deployment Checklist

## ✅ Configuration Complete

All environment variables have been configured and validated. The application is ready to build and deploy.

## Pre-Deployment Verification

### 1. Configuration Files Status

- ✅ `.env` - Active environment configuration (994 bytes, 12 variables)
- ✅ `.env.example` - Configuration template (4.3KB, comprehensive documentation)
- ✅ `auth_config.json` - Valid JSON authentication config (365 bytes)
- ✅ `auth_config.json.example` - Authentication template (409 bytes)
- ✅ `.gitignore` - Properly excludes `.env` file

### 2. Environment Variables Configured

| Variable | Value | Status |
|----------|-------|--------|
| `KIRO_AUTH_TOKEN` | `/tmp/cc-agent/59364193/project/auth_config.json` | ✅ File-based |
| `KIRO_CLIENT_TOKEN` | `RcHfY3zpMNXmL4hCSlP1QQZjlZTQ9+LnaVnSbt+xy9A=` | ✅ Secure (32 bytes) |
| `PORT` | `8080` | ✅ Default |
| `GIN_MODE` | `release` | ✅ Production |
| `LOG_LEVEL` | `info` | ✅ Production |
| `LOG_FORMAT` | `json` | ✅ Structured |
| `LOG_CONSOLE` | `true` | ✅ Enabled |
| `LOG_FILE` | Empty | ✅ Console only |
| `MAX_TOOL_DESCRIPTION_LENGTH` | `10000` | ✅ Default |
| `DEBUG` | `false` | ✅ Production |

### 3. Authentication Details

**Social Authentication (Github OAuth)**
- Provider: Github
- Profile ARN: `arn:aws:codewhisperer:us-east-1:699475941385:profile/EHGA3GRVQMUK`
- Token Expiry: 2025-10-28T21:53:25.536Z
- Status: ✅ Active

## Deployment Methods

### Method 1: Local Build (Recommended for Development)

```bash
# Navigate to project
cd /tmp/cc-agent/59364193/project

# Build the application
go build -o kiro2api main.go

# Run the application
./kiro2api

# Expected output:
# {"level":"info","time":"...","message":"Starting kiro2api server..."}
# {"level":"info","time":"...","message":"Server listening on :8080"}
```

### Method 2: Docker Compose (Recommended for Production)

```bash
# Navigate to project
cd /tmp/cc-agent/59364193/project

# Start with docker-compose
docker-compose up -d

# Check logs
docker-compose logs -f

# Check health
docker-compose ps
```

### Method 3: Docker Build

```bash
# Build the image
docker build -t kiro2api:latest .

# Run the container
docker run -d \
  --name kiro2api \
  -p 8080:8080 \
  --env-file .env \
  kiro2api:latest

# Check logs
docker logs -f kiro2api
```

### Method 4: Pre-built Docker Image

```bash
# Pull and run the latest image
docker run -d \
  --name kiro2api \
  -p 8080:8080 \
  -v $(pwd)/auth_config.json:/app/auth_config.json:ro \
  -e KIRO_AUTH_TOKEN=/app/auth_config.json \
  -e KIRO_CLIENT_TOKEN=RcHfY3zpMNXmL4hCSlP1QQZjlZTQ9+LnaVnSbt+xy9A= \
  -e PORT=8080 \
  -e GIN_MODE=release \
  -e LOG_LEVEL=info \
  -e LOG_FORMAT=json \
  ghcr.io/caidaoli/kiro2api:latest
```

## Post-Deployment Testing

### 1. Health Check

```bash
# Check if service is running
curl -s http://localhost:8080/v1/models

# Expected: JSON response with available models
```

### 2. Authentication Test

```bash
# Test with client token
curl -H "Authorization: Bearer RcHfY3zpMNXmL4hCSlP1QQZjlZTQ9+LnaVnSbt+xy9A=" \
  http://localhost:8080/v1/models

# Expected: 200 OK with model list
```

### 3. API Functionality Test

```bash
# Test Anthropic-compatible endpoint
curl -X POST http://localhost:8080/v1/messages \
  -H "Authorization: Bearer RcHfY3zpMNXmL4hCSlP1QQZjlZTQ9+LnaVnSbt+xy9A=" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "max_tokens": 100,
    "messages": [
      {"role": "user", "content": "Hello, test message"}
    ]
  }'

# Expected: JSON response with AI completion
```

### 4. Streaming Test

```bash
# Test streaming response
curl -N -X POST http://localhost:8080/v1/messages \
  -H "Authorization: Bearer RcHfY3zpMNXmL4hCSlP1QQZjlZTQ9+LnaVnSbt+xy9A=" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-20250514",
    "max_tokens": 100,
    "stream": true,
    "messages": [
      {"role": "user", "content": "Count to 5"}
    ]
  }'

# Expected: SSE stream with incremental responses
```

### 5. Token Status Check

```bash
# Check token pool status (no auth required)
curl http://localhost:8080/api/tokens

# Expected: JSON with token usage information
```

## Integration with Claude Code

### Configuration

```bash
# Set environment variables for Claude Code
export ANTHROPIC_BASE_URL="http://localhost:8080/v1"
export ANTHROPIC_API_KEY="RcHfY3zpMNXmL4hCSlP1QQZjlZTQ9+LnaVnSbt+xy9A="

# Test Claude Code
claude-code --model claude-sonnet-4 "Hello, test"
```

### VS Code Settings

Add to `.vscode/settings.json`:

```json
{
  "anthropic.baseUrl": "http://localhost:8080/v1",
  "anthropic.apiKey": "RcHfY3zpMNXmL4hCSlP1QQZjlZTQ9+LnaVnSbt+xy9A="
}
```

## Monitoring

### Check Logs

```bash
# Docker
docker logs -f kiro2api

# Docker Compose
docker-compose logs -f

# Local (console output)
# Logs appear in stdout/stderr with structured JSON format
```

### Monitor Resources

```bash
# Docker stats
docker stats kiro2api

# System resources
top -p $(pgrep kiro2api)
```

## Troubleshooting

### Common Issues

| Issue | Solution |
|-------|----------|
| **Port already in use** | Change `PORT` in `.env` or stop conflicting service |
| **Authentication fails** | Verify `KIRO_CLIENT_TOKEN` matches in requests |
| **Token refresh errors** | Check `auth_config.json` has valid refresh token |
| **No response** | Check service is running: `curl localhost:8080/v1/models` |

### Debug Mode

To enable detailed debugging:

```bash
# Edit .env
LOG_LEVEL=debug
DEBUG=true
GIN_MODE=debug

# Restart service
```

## Security Reminders

⚠️ **CRITICAL SECURITY NOTES**

1. **Never commit `.env` file** - Already protected by `.gitignore`
2. **Rotate client token regularly** - Use: `openssl rand -base64 32`
3. **Keep refresh tokens secure** - Stored in `auth_config.json`
4. **Use HTTPS in production** - Add reverse proxy (nginx, caddy)
5. **Monitor token expiration** - Current token expires: 2025-10-28

## Multi-Account Setup (Optional)

If you need to add more accounts, edit `auth_config.json`:

```json
[
  {
    "auth": "Social",
    "refreshToken": "aorAAAAAGl3JfA...",
    "description": "Primary Github OAuth",
    "disabled": false
  },
  {
    "auth": "IdC",
    "refreshToken": "...",
    "clientId": "ty6HkZrNwxPuEyWU74VG6nVzLWVhc3QtMQ",
    "clientSecret": "eyJraWQiOiJrZXktMTU2NDAyODA5OSIsImFsZyI6IkhTMzg0In0...",
    "description": "Kiro IDE Enterprise",
    "disabled": false
  }
]
```

Restart the service after changes.

## Additional Resources

- **Configuration Guide**: `.env.example`
- **Developer Guide**: `CLAUDE.md`
- **User Documentation**: `README.md`
- **Configuration Summary**: `CONFIGURATION_SUMMARY.txt`

---

**Status**: ✅ Ready for deployment
**Last Updated**: 2025-10-28
**Configuration Version**: 1.0
