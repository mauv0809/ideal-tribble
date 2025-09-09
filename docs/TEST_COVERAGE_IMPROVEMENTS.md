# Test Coverage Improvements

## Issue Identified
The job handler bug (where handlers were no-op functions) wasn't caught by existing tests because of gaps in test coverage architecture.

## Current Test Coverage Gaps

### 1. No Integration Testing
- **Missing**: `main_test.go` or integration test package
- **Problem**: Tests only mock components, never test real interactions
- **Impact**: Job queue → worker → processor flow is never tested end-to-end

### 2. Unit Tests Stop at Interface Level
- **Current**: Tests verify jobs are enqueued (`jobqueue.EnqueueCalls`)
- **Missing**: Tests that verify job execution calls correct processor methods
- **Example**: Test checks `JobTypeAssignBallBoy` is enqueued but not that it calls `processor.AssignBallBringer()`

### 3. Job Handler Logic Untested
- **Location**: `main.go:92-129` job handler registration
- **Missing**: Direct tests for each job handler function
- **Risk**: Handler implementation bugs (like no-ops) go undetected

### 4. State Machine Flow Untested
- **Current**: Tests mock status updates
- **Missing**: Verification that complete state transitions work (job completion → status update → next processing cycle)

## Recommended Test Improvements

### Phase 1: Job Handler Unit Tests
Create `main_test.go` with tests for each job handler:

```go
func TestJobHandlers(t *testing.T) {
    t.Run("AssignBallBoy handler calls processor method", func(t *testing.T) {
        // Setup: mock processor, real job handler
        // Execute: call handler with match payload
        // Verify: processor.AssignBallBringer was called with correct params
    })
    
    t.Run("NotifyBooking handler calls processor method", func(t *testing.T) {
        // Similar pattern for each handler
    })
    // ... etc for all job types
}
```

### Phase 2: Integration Tests
Create `integration_test.go`:

```go
func TestJobWorkerIntegration(t *testing.T) {
    t.Run("complete ball bringer assignment flow", func(t *testing.T) {
        // Setup: real database, real job queue, real worker
        // Execute: 
        //   1. Insert match with StatusNew
        //   2. Run ProcessMatches() 
        //   3. Let worker process job
        //   4. Run ProcessMatches() again
        // Verify: Match progresses from StatusNew → StatusAssigningBallBringer → StatusBallBoyAssigned
    })
}
```

### Phase 3: State Machine Tests
Enhance existing processor tests:

```go
func TestCompleteMatchLifecycle(t *testing.T) {
    // Test complete flow with real job queue
    // Verify each status transition happens correctly
    // Include job execution, not just job enqueuing
}
```

### Phase 4: End-to-End Tests
Create tests that exercise the full application:

```go
func TestE2EMatchProcessing(t *testing.T) {
    // Setup: real database with test data
    // Execute: HTTP requests to trigger processing
    // Verify: Database shows correct final state
}
```

## Implementation Strategy

### Step 1: Add Job Handler Tests (Immediate)
- Create `main_test.go` 
- Test each job handler function individually
- Mock processor, verify correct method calls

### Step 2: Refactor for Testability (Medium term)
- Extract job handler registration into separate function
- Make job handlers injectable/configurable
- Create test helpers for common setup

### Step 3: Add Integration Tests (Medium term)
- Test job queue + worker + processor together
- Use real SQLite database with test data
- Verify complete state transitions

### Step 4: Add E2E Tests (Long term)
- Test via HTTP API endpoints
- Use real database, verify final state
- Include error scenarios and edge cases

## Success Criteria

### Immediate Goals
- [ ] Job handler functions are directly tested
- [ ] Bugs like no-op handlers are caught by tests
- [ ] Each processor method is verified to be called correctly

### Medium-term Goals  
- [ ] Complete state machine flows are tested
- [ ] Integration between job queue and processor is tested
- [ ] Database state changes are verified

### Long-term Goals
- [ ] E2E testing covers all user scenarios
- [ ] Performance testing for job processing
- [ ] Error handling and retry logic is tested

## Notes
- Current tests are good for individual component behavior
- Gap is in testing **interactions between components**
- Priority should be on job handler tests (quick win) then integration tests
- Consider using testcontainers for integration tests with real database