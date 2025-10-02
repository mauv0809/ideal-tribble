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

### 11. Automatic Match Proposal Workflow 
- **Files**: `internal/http/handlers/slack_events.go`, `internal/matchmaking/store.go`, `internal/notifier/slack/slack.go`
- **What**: Complete automatic match proposal system that triggers after availability updates
- **Features**: 
  - Integrates `CanProposeMatch` into reaction processing workflow
  - Automatically calls `ProposeMatch` when 4+ players available for same date
  - Sends match proposal notifications as threaded Slack replies
  - Includes team assignments and booking responsibility
  - Fixed deadlock issue in ProposeMatch method
  - Added comprehensive logging and error handling
  - Uses default match times (06:00-07:30) for proposals

### 12. Deadlock Resolution & Debugging System
- **Files**: `internal/matchmaking/store.go`, `internal/notifier/slack/slack.go`
- **What**: Resolved critical deadlock in match proposal system and added debugging tools
- **Features**:
  - Fixed nested mutex lock deadlock in ProposeMatch method
  - Added detailed logging throughout matchmaking pipeline
  - Enhanced thread timestamp validation for Slack messages
  - Improved error handling and validation in notification system

## 🚧 PENDING TASKS

### 13. Match Detection & Completion System → ✅ COMPLETE
- **What**: Detect when proposed matches are booked on Playtomic and mark requests as completed
- **Implementation**:
  - ✅ Enhanced `/fetch` endpoint to detect matches matching proposed requests
  - ✅ Matches by date, time range (30min tolerance), booking responsible player, and team composition (all 4 players)
  - ✅ Updates match request status to `StatusCompleted` when detected
  - ✅ Handles edge cases with time tolerance and player verification
  - ✅ Comprehensive test coverage in `internal/matchmaking/store_test.go`
- **Files modified**:
  - `internal/http/handlers/scheduled.go` - Added detection logic to fetch workflow
  - `internal/matchmaking/store.go` - Already had `DetectMatchedRequests()` method
  - `internal/http/server.go` - Updated handler dependency injection
  - `internal/http/handlers_test.go` - Updated test to pass matchmaking service

### 14. Dynamic Team Reassignment → DEFERRED FOR V2 🚀
- **Decision**: Ship current system first, iterate based on real usage feedback
- **Current State**: System works well without dynamic reassignment - users can create new `/match` requests if availability changes significantly after proposal
- **V2 Implementation Planned**: Will implement based on actual user behavior and feedback

#### **Future Enhancements for Dynamic Team Reassignment** (MEDIUM PRIORITY):
- **Advanced Team Balancing**: Use player skill levels from database for team assignment
- **Booking Responsibility Intelligence**: 
  - Prefer players with lower ball_bringer_count
  - Consider player booking history/reliability
- **Smart Notification Strategy**: 
  - Update original proposal message instead of new thread
  - Add "Updated proposal" indicators
- **Rapid Change Handling**: 
  - Debouncing for multiple quick availability changes
  - Rate limiting for proposal updates
- **Advanced Edge Cases**:
  - Handle partial team changes (only 1-2 players change)
  - Consider player preferences/partnerships
  - Handle timezone considerations for proposal timing
- **User Experience**:
  - Allow manual team override by admin users  
  - Player preference system for partnerships
  - Notification preferences (some users may not want constant updates)

### 15. Team Assignment Enhancement (MEDIUM PRIORITY)
- **What**: Improve team balancing algorithm in ProposeMatch
- **Requirements**:
  - Balance teams by skill level (use existing player levels from database)
  - Consider player preferences/partnerships (future enhancement)
  - Handle odd number of players (>4 available)
- **Files to modify**: `internal/matchmaking/store.go` (enhance ProposeMatch method)

### 16. Cleanup Job (LOW PRIORITY)
- **What**: Remove old match request data after completion
- **Requirements**:
  - Clean up player availability records for completed matches
  - Archive or remove old match requests
  - Prevent database bloat
  - Run periodically via scheduled endpoint
- **Files to create**: New cleanup service or enhance existing processor

### 17. End-to-End Testing (LOW PRIORITY)
- **What**: Test complete matchmaking flow
- **Requirements**:
  - Test `/match` command → availability collection → match proposal → booking detection
  - Test dynamic team reassignment scenarios
  - Test edge cases (not enough players, no availability overlap)
  - Test new member onboarding flow
  - Verify Slack webhook event handling

## 🔄 CURRENT WORKFLOW STATUS

### ✅ Complete Flow:
1. ✅ User runs `/match` command
2. ✅ System creates match request and sends availability message
3. ✅ Players react with day emojis (1️⃣-7️⃣)
4. ✅ System processes reactions and stores availability
5. ✅ System automatically analyzes availability and proposes matches
6. ✅ System sends match proposal with team assignments and booking responsibility
7. ✅ Player books match on Playtomic (external action)
8. ✅ System detects booked match via fetch endpoint and marks request as completed

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

### ✅ RECENTLY COMPLETED:
- **Task #11**: Automatic Match Proposal Workflow - ✅ COMPLETE
- **Task #12**: Deadlock Resolution & Debugging System - ✅ COMPLETE
- **Task #13**: Match Detection & Completion System - ✅ COMPLETE

### IMMEDIATE PRIORITY: Task #15 - Team Assignment Enhancement (MEDIUM PRIORITY)

1. **Improve team balancing algorithm in ProposeMatch**
   - Balance teams by skill level (use existing player levels from database)
   - Consider player preferences/partnerships (future enhancement)
   - Handle odd number of players (>4 available)

2. **Implementation guidance**
   - Modify `internal/matchmaking/store.go` (enhance ProposeMatch method)
   - Fetch player levels from club store
   - Implement balancing algorithm (e.g., sort by level, alternate assignment)
   - Test with various player level combinations

### TERTIARY PRIORITIES:
- **Task #16**: Cleanup jobs for old data
- **Task #17**: End-to-end testing

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