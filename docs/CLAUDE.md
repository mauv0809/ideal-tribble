# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**ideal-tribble** is a Go-based backend service for managing a private Padel club through "Wally", a Slack bot that coordinates matches, tracks statistics, and manages responsibilities. The system integrates with Playtomic API for match data and uses Google Cloud Platform for infrastructure.

## Development Commands

### Local Development
```bash
# Hot reloading development server
air

# Run tests
go test -v -race ./...

# Build CLI tool
make build

# Run CLI tool
./tribble-cli
```

### Environment Setup
```bash
# Copy environment template
cp .env.example .env
# Edit .env with actual credentials before running locally
```

### Database
```bash
# Run migrations (handled automatically in main.go)
goose up

# Create new migration
goose create migration_name sql
```

## Architecture

### Clean Architecture Pattern
- **Internal packages**: Core business logic in `internal/` (Go convention)
- **Interfaces**: Each service defines interfaces for testability  
- **Dependency injection**: Services injected through constructors
- **Clear separation**: HTTP, business logic, and data layers are distinct

### Core Services
- **Club Store** (`internal/club/`): Player management with fuzzy name matching
- **Matchmaking Service** (`internal/matchmaking/`): Match requests and availability collection  
- **Processor** (`internal/processor/`): Asynchronous match processing pipeline
- **Notifier** (`internal/notifier/slack/`): Slack message formatting and delivery
- **Playtomic Client** (`internal/playtomic/`): Third-party API integration
- **Metrics Service** (`internal/metrics/`): Operational monitoring

### HTTP Route Organization
Routes are organized by trigger source in separate handler files:
- **Health/Operational**: `/health`, `/metrics`, `/clear` → `handlers/health.go`
- **REST API**: `/members`, `/matches`, `/leaderboard` → `handlers/api.go`  
- **Scheduled**: `/fetch`, `/process` → `handlers/scheduled.go` (Cloud Scheduler)
- **Pub/Sub**: Processing endpoints → `handlers/pubsub.go`
- **Slack Commands**: `/slack/command/*` → `handlers/slack_commands.go`
- **Slack Events**: `/slack/events` → `handlers/slack_events.go` (webhooks)

### Database Schema
SQLite with 6 migrations covering:
- **Players**: ID, name, level, ball-bringing count
- **Matches**: Complete match data with processing status
- **Player Stats**: Win/loss records, sets/games statistics
- **Weekly Stats**: Periodic snapshots  
- **Match Requests**: Matchmaking system data (new feature)
- **Slack Mapping**: User identity linking (new feature)

## Current Development Status

### Active Branch: `feature/matchmaking-system`
Major new feature in development that allows users to:
1. Request matches via `/match` command
2. Indicate availability with emoji reactions (1️⃣-7️⃣ for days)
3. Get automated match proposals with team assignments

### Completed Components
- ✅ Match request creation and storage
- ✅ Emoji reaction processing for availability collection
- ✅ Player mapping system (Slack user → Playtomic player)
- ✅ New member onboarding with welcome messages
- ✅ Database schema and CRUD operations

### Critical Missing Components
- ❌ **Match proposal algorithm** (HIGH PRIORITY): Analyze availability and suggest matches
- ❌ **Team assignment logic** (HIGH PRIORITY): Balance teams and assign booking responsibility  
- ❌ **Match confirmation workflow** (MEDIUM PRIORITY): Handle player confirmations/declines
- ❌ **Cleanup jobs** (LOW PRIORITY): Remove old data

## Key Implementation Patterns

### Error Handling
- Comprehensive error handling throughout with proper logging
- Idempotent operations prevent duplicate notifications and processing
- Graceful degradation when external services fail

### Player Mapping System
- Uses fuzzy search with Levenshtein distance for name matching
- Confidence scoring: >0.8 auto-maps, 0.5-0.8 requires confirmation
- Handles Slack display names vs real names vs Playtomic names

### Slack Integration
- Proper signature verification for security (`X-Slack-Signature`)
- Events API webhook handling with challenge verification
- Emoji reaction processing (1️⃣=Monday, 2️⃣=Tuesday, etc.)
- Direct message capability for user onboarding

### Asynchronous Processing
- Uses Google Cloud Pub/Sub for event-driven architecture
- Match processing pipeline with state machine pattern
- Scheduled operations via Cloud Scheduler

## Testing Approach

### Unit Tests
- Core business logic has comprehensive test coverage
- Uses testify framework for assertions and mocking
- Race condition detection with `-race` flag

### Key Test Files
- `internal/club/store_test.go` - Player management logic
- `internal/matchmaking/store_test.go` - Matchmaking CRUD operations
- `internal/processor/processor_test.go` - Match processing pipeline

## Infrastructure

### Google Cloud Platform
- **Cloud Run**: Container deployment with auto-scaling
- **Cloud Scheduler**: Periodic match fetching and processing
- **Pub/Sub**: Asynchronous event processing
- **Secret Manager**: Secure credential storage
- **Artifact Registry**: Container image storage

### Infrastructure as Code
- **Terraform**: Complete infrastructure definition in `terraform/`
- **GitHub Actions**: Automated CI/CD pipeline (`ci-cd.yml`)
- **Docker**: Multi-stage builds for production deployment

## Development Guidelines

### Adding New Features
1. Define interfaces in the appropriate service package
2. Implement business logic with proper error handling
3. Add HTTP handlers following the trigger-source organization
4. Write comprehensive tests for core logic
5. Update database schema with new migrations if needed

### Database Changes
- Always use migrations via goose
- Index frequently queried fields
- Use foreign keys for data integrity
- Keep backward compatibility in mind

### Slack Integration
- Verify webhook signatures for security
- Handle both slash commands and events API
- Use threaded messages for complex interactions
- Implement proper message formatting helpers

## External Dependencies

### Critical APIs
- **Playtomic API**: Match data source (go-playtomic-api)
- **Slack API**: Bot interactions (slack-go/slack)
- **Turso/LibSQL**: Distributed SQLite database

### Authentication Requirements
- Slack Bot Token (xoxb-*) 
- Slack Signing Secret for webhook verification
- Turso database credentials
- GCP service account for cloud services

## Monitoring and Metrics

### Operational Metrics
Exposed via `/metrics` endpoint:
- Number of match processing runs
- Playtomic API call counts  
- Slack notification counts
- Processing errors and success rates

### Logging
Uses charmbracelet/log for structured logging throughout the application.

## Performance Considerations

### Database
- SQLite with WAL mode for concurrent access
- Proper indexing on match queries and player lookups
- Connection pooling handled by database/sql

### API Rate Limits
- Respects Playtomic API rate limits
- Slack API rate limiting handled by slack-go library
- Implements backoff and retry logic where appropriate

### Memory Management
- Efficient JSON processing for large match datasets
- Streaming approach for bulk operations
- Proper resource cleanup in goroutines

## Security

### Webhook Security
- Slack signature verification on all webhook endpoints
- Proper secret management through environment variables
- No sensitive data in logs or error messages

### Infrastructure Security
- GCP IAM with least privilege principles
- Workload Identity Federation for GitHub Actions
- Secrets stored in Google Secret Manager

## Git Operations

### Commit Guidelines
- Remember that we need to use --no-gpg-sign when doing git commit