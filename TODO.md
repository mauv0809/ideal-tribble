# Matchmaking System TODO

## Overview
This branch (`feature/matchmaking-system`) implements a matchmaking feature for the Padel club Slack bot. Users can request matches via `/match` command, indicate availability with emoji reactions, and receive match proposals with team assignments.

## Architecture
- **Database**: SQLite with migrations for match requests and player availability
- **Slack Integration**: Events API for reactions, slash commands, new member welcome
- **Matchmaking Service**: Handles match creation, availability collection, and proposals
- **Player Mapping**: Fuzzy search system to link Slack users to Playtomic profiles

## ✅ COMPLETED TASKS

### 1. Database Schema & Core Structure
- **Files**: `migrations/000004_create_match_requests.sql`, `migrations/000005_add_slack_mapping.sql`
- **What**: Created database tables for match requests, player availability, and Slack user mapping
- **Commit**: Initial schema implementation

### 2. Matchmaking Service Interface & Store
- **Files**: `internal/matchmaking/interface.go`, `internal/matchmaking/store.go`, `internal/matchmaking/types.go`
- **What**: Complete CRUD operations for match requests and availability analysis
- **Methods**: CreateMatchRequest, GetPlayerAvailability, AnalyzeAvailability, AddPlayerAvailability, RemovePlayerAvailability

### 3. Slack Notifier Extensions
- **Files**: `internal/notifier/slack/slack.go`
- **What**: Added matchmaking message formatting and direct message capability
- **Methods**: SendMatchAvailabilityRequest, SendMatchProposal, SendMatchConfirmation, SendDirectMessage

### 4. Player Mapping System
- **Files**: `internal/club/mapper.go`, `internal/club/store.go`
- **What**: Fuzzy search to map Slack users to Playtomic players using Levenshtein distance
- **Features**: Confidence scoring, auto-mapping for high confidence (>0.8), manual confirmation for medium confidence

### 5. Slack Integration
- **Files**: `internal/http/handlers/slack_commands.go`, `internal/http/handlers/slack_events.go`
- **What**: 
  - `/match` slash command handler with player mapping
  - Slack Events API webhook with challenge verification
  - Emoji reaction processing (1️⃣-7️⃣ for Monday-Sunday)
  - New member welcome system with Playtomic profile setup instructions

### 6. Handler Restructuring
- **Files**: `internal/http/handlers/` directory with organized structure
- **What**: Split monolithic handlers.go into focused files by trigger/source:
  - `health.go`: Basic operational endpoints
  - `scheduled.go`: Cloud Scheduler triggered endpoints  
  - `pubsub.go`: Pub/Sub push subscription endpoints
  - `api.go`: REST API endpoints
  - `slack_commands.go`: Slack slash command endpoints
  - `slack_events.go`: Slack webhook events
  - `helpers.go`: Shared helper functions

### 7. Availability Collection System
- **What**: Complete emoji reaction processing workflow
- **Flow**: User reacts with day emoji → Check if message is active match request → Map Slack user to Playtomic player → Store availability in database
- **Features**: Handles both reaction additions and removals, filters by channel and active requests

### 8. New Member Onboarding
- **What**: Automatic welcome message when new members join the Slack channel
- **Flow**: New member joins → Send DM with instructions → User shares Playtomic profile URL → Manual linking process

### 9. Booking Responsibility Assignment
- **Files**: `internal/club/store.go`, `internal/matchmaking/store.go`, `migrations/000007_add_booking_counts.sql`
- **What**: Added atomic booking responsibility assignment with fairness tracking
- **Features**: 
  - Database fields for tracking booking counts per player
  - Atomic assignment method that selects player with lowest booking count
  - Integration with match proposal system
  - Proper fallback handling when assignment fails

### 10. Availability Checking System 
- **Files**: `internal/matchmaking/interface.go`, `internal/matchmaking/store.go`, `internal/matchmaking/store_test.go`
- **What**: Added method to check if enough players are available to propose a match
- **Features**:
  - `CanProposeMatch` method returns true when 4+ players available for any date
  - Returns best available date even when insufficient players (for debugging)
  - Comprehensive test suite covering all scenarios
  - Ready for integration with automatic match proposal workflow

## 🚧 PENDING TASKS

### 11. Automatic Match Proposal Workflow (HIGH PRIORITY)
- **What**: Integrate availability checking with automatic match proposals
- **Requirements**: 
  - Trigger `CanProposeMatch` after each availability update 
  - Automatically call `ProposeMatch` when enough players available
  - Send match proposal notification via Slack
  - Handle timing (wait for sufficient responses vs immediate proposals)
- **Files to modify**: `internal/http/handlers/slack_events.go` (reaction processing), `internal/matchmaking/store.go`

### 12. Team Assignment Enhancement (MEDIUM PRIORITY)
- **What**: Improve team balancing algorithm in ProposeMatch
- **Requirements**:
  - Balance teams by skill level (use existing player levels from database)
  - Consider player preferences/partnerships (future enhancement)
  - Handle odd number of players (>4 available)
- **Files to modify**: `internal/matchmaking/store.go` (enhance ProposeMatch method)

### 13. Match Confirmation & Notification System (MEDIUM PRIORITY)
- **What**: Send match proposals and handle confirmations
- **Requirements**:
  - Send threaded message with match proposal
  - Include team assignments and booking responsibility
  - Handle player confirmations/declines
  - Send final confirmation with court booking details
- **Files to modify**: `internal/notifier/slack/slack.go`, `internal/http/handlers/slack_events.go`

### 14. Cleanup Job (LOW PRIORITY)
- **What**: Remove old match request data after completion
- **Requirements**:
  - Clean up player availability records for completed matches
  - Archive or remove old match requests
  - Prevent database bloat
  - Run periodically via scheduled endpoint
- **Files to create**: New cleanup service or enhance existing processor

### 15. End-to-End Testing (LOW PRIORITY)
- **What**: Test complete matchmaking flow
- **Requirements**:
  - Test `/match` command → availability collection → match proposal → confirmation
  - Test edge cases (not enough players, no availability overlap)
  - Test new member onboarding flow
  - Verify Slack webhook event handling

## 🔄 CURRENT WORKFLOW STATUS

### Working Flow:
1. ✅ User runs `/match` command
2. ✅ System creates match request and sends availability message
3. ✅ Players react with day emojis (1️⃣-7️⃣)
4. ✅ System processes reactions and stores availability

### Missing Flow:
5. ❌ System analyzes availability and proposes matches
6. ❌ System sends match proposal with team assignments
7. ❌ Players confirm/decline participation
8. ❌ System sends final confirmation with booking details

## 📁 KEY FILES & DIRECTORIES

### Core Business Logic:
- `internal/matchmaking/` - Main matchmaking service
- `internal/club/mapper.go` - Player mapping system
- `internal/notifier/slack/` - Slack message formatting

### HTTP Handlers:
- `internal/http/handlers/slack_events.go` - Reaction processing
- `internal/http/handlers/slack_commands.go` - /match command
- `internal/http/server.go` - Route configuration

### Database:
- `migrations/000004_create_match_requests.sql` - Match request schema
- `migrations/000005_add_slack_mapping.sql` - Player mapping schema

## 🚀 NEXT STEPS TO RESUME WORK

### IMMEDIATE PRIORITY: Task #11 - Automatic Match Proposal Workflow

1. **Integrate CanProposeMatch into reaction processing** 
   - Modify `internal/http/handlers/slack_events.go` 
   - Call `CanProposeMatch` after each `AddPlayerAvailability`/`RemovePlayerAvailability`
   - When `canPropose == true`, automatically trigger match proposal

2. **Add automatic match proposal logic**
   - Define default match times (e.g., 06:00-07:30)
   - Call `ProposeMatch` with best available date and default times
   - Update match request status to `StatusProposingMatch`

3. **Send match proposal notification**
   - Use existing `SendMatchProposal` method in notifier
   - Include team assignments and booking responsibility
   - Send as threaded reply to original availability request

4. **Add configuration/timing logic**
   - Consider waiting period vs immediate proposals
   - Handle multiple requests for same day
   - Add logging for proposal decisions

### SECONDARY PRIORITIES:
- **Task #12**: Enhance team balancing with skill levels
- **Task #13**: Add player confirmation workflow

## 💡 IMPLEMENTATION NOTES

### Player Mapping:
- Uses fuzzy search with confidence scoring
- Auto-maps high confidence matches (>0.8)
- Requires manual confirmation for medium confidence (0.5-0.8)
- New members get welcome message with Playtomic profile setup

### Availability Collection:
- Emoji reactions: 1️⃣=Monday, 2️⃣=Tuesday, etc.
- Only processes reactions on active match request messages
- Handles both addition and removal of reactions
- Filters by club channel ID to ignore other channels

### Database Design:
- `match_requests` table tracks overall match requests
- `match_request_availability` table tracks individual player availability
- Foreign key relationships ensure data integrity
- Indexes on frequently queried fields for performance

## 🎯 SUCCESS CRITERIA

The matchmaking system will be complete when:
- [ ] A user can run `/match` and get a fully organized match with teams
- [ ] All players receive clear booking instructions
- [ ] The system handles edge cases gracefully
- [ ] New members can easily onboard and participate
- [ ] The system scales to handle multiple simultaneous match requests