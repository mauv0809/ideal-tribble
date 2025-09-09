# Zero-Downtime Deployment Strategy

This document outlines options for implementing zero-downtime deployment for ideal-tribble while maintaining our current Hetzner + Terraform infrastructure.

## Current State

- **Single server** deployment on Hetzner Cloud
- **Systemd service** managing the Go binary
- **Nginx reverse proxy** for HTTPS termination
- **Brief downtime** during binary replacement (~5-10 seconds)

## Zero-Downtime Deployment Options

### Option 1: Single Server with Process Management

Keep single server but improve the deployment process to avoid stopping the service.

#### Approach 1A: Graceful Binary Replacement
```bash
# 1. Copy new binary with different name
scp new-binary server:/opt/ideal-tribble/ideal-tribble.new

# 2. Send SIGTERM to current process (graceful shutdown)
kill -TERM $(pidof ideal-tribble)

# 3. Atomic rename when process exits
mv /opt/ideal-tribble/ideal-tribble.new /opt/ideal-tribble/ideal-tribble

# 4. Start new process
systemctl start ideal-tribble
```

**Pros:**
- Minimal infrastructure changes
- Low cost (single server)
- Reduces downtime to ~1-2 seconds
- Simple to implement

**Cons:**
- Still has brief downtime during process restart
- Single point of failure
- No rollback capability during deployment
- Database connections may be lost

#### Approach 1B: Hot Binary Swapping with symlinks
```bash
# Use symlinks for atomic binary swapping
/opt/ideal-tribble/
├── current -> v1.0.1/
├── v1.0.0/
│   └── ideal-tribble
└── v1.0.1/
    └── ideal-tribble
```

**Pros:**
- Atomic binary replacement
- Easy rollback (change symlink)
- Keeps multiple versions

**Cons:**
- Still requires process restart
- More complex file management
- Not true zero-downtime

### Option 2: Multi-Instance Single Server (Rolling Deployment)

Run multiple instances of the application on the same server behind a load balancer.

#### Implementation:
```yaml
# nginx.conf - Round robin between instances
upstream ideal_tribble {
    server 127.0.0.1:8081;
    server 127.0.0.1:8082;
}

# systemd services
# ideal-tribble@8081.service
# ideal-tribble@8082.service
```

#### Deployment Process:
1. Deploy to instance 1, remove from load balancer
2. Wait for health check to pass
3. Add back to load balancer
4. Repeat for instance 2

**Pros:**
- True zero-downtime
- Uses existing single server
- Rolling deployment capability
- Built-in redundancy

**Cons:**
- Doubled memory usage (~60MB → 120MB)
- More complex configuration
- Shared database could be bottleneck
- Port management complexity

### Option 3: Blue/Green Deployment

Maintain two identical production environments and switch traffic between them.

#### Option 3A: Blue/Green with Multiple Servers
```
Blue Environment:  Server 1 (currently serving traffic)
Green Environment: Server 2 (new version deployed here)

Switch: Load balancer routes all traffic from Blue → Green
```

#### Implementation:
```hcl
# terraform/hetzner/main.tf
resource "hcloud_server" "blue" {
  name = "tribble-blue"
  # ... configuration
}

resource "hcloud_server" "green" {
  name = "tribble-green"
  # ... configuration
}

resource "hcloud_load_balancer" "main" {
  name = "tribble-lb"
  target {
    type = "server"
    server_id = var.active_environment == "blue" ? 
                hcloud_server.blue.id : 
                hcloud_server.green.id
  }
}
```

#### Deployment Process:
1. Deploy new version to inactive environment (Green)
2. Run smoke tests on Green environment
3. Switch load balancer to point to Green
4. Monitor for issues
5. Keep Blue as rollback option

**Pros:**
- **Instant rollback** (switch LB back)
- **Full testing** on production-like environment
- **Zero downtime** during switch
- **Complete isolation** between versions
- **Easy rollback** if issues detected

**Cons:**
- **Double infrastructure cost** (€6.58/month)
- **Complex state management** (database migrations)
- **Storage synchronization** challenges
- **Resource waste** (one environment always idle)

#### Option 3B: Blue/Green with Single Server + Containers
```bash
# Two containers on same server
docker run -d --name tribble-blue -p 8081:8080 ideal-tribble:v1.0
docker run -d --name tribble-green -p 8082:8080 ideal-tribble:v1.1

# Switch nginx upstream
# Blue active:  upstream points to 127.0.0.1:8081
# Green active: upstream points to 127.0.0.1:8082
```

**Pros:**
- Lower cost than dual servers
- Quick switching via nginx config
- Container isolation benefits

**Cons:**
- Still shares server resources
- Manual nginx configuration switching
- Container orchestration complexity

### Option 4: Canary Deployment

Gradually roll out new version to subset of traffic/users.

#### Option 4A: Traffic-Based Canary
```nginx
# nginx.conf - Weighted routing
upstream tribble_stable {
    server 127.0.0.1:8081 weight=90;  # 90% traffic
}

upstream tribble_canary {
    server 127.0.0.1:8082 weight=10;  # 10% traffic
}

server {
    location / {
        # Route traffic based on weight
        proxy_pass http://tribble_stable;
        
        # Or use random/hash routing for canary
        if ($request_id ~ "^.{0,1}") {  # ~10% of requests
            proxy_pass http://tribble_canary;
        }
    }
}
```

#### Option 4B: User-Based Canary (Feature Flags)
```go
// In application code
func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
    userID := extractUserID(r)
    
    if s.featureFlags.IsCanaryUser(userID) {
        // Route to new logic/version
        s.handleWithNewFeatures(w, r)
    } else {
        // Route to stable logic
        s.handleWithStableFeatures(w, r)
    }
}
```

#### Deployment Process:
1. Deploy canary version alongside stable
2. Route small percentage of traffic to canary (5-10%)
3. Monitor metrics (error rates, response times, business metrics)
4. Gradually increase canary traffic (10% → 25% → 50% → 100%)
5. Remove stable version once canary is fully validated

**Pros:**
- **Gradual risk mitigation**
- **Real user feedback** on small scale
- **Data-driven rollout** based on metrics
- **Early detection** of issues
- **Flexible rollback** at any percentage

**Cons:**
- **Complex routing logic**
- **Mixed version state** can be confusing
- **Monitoring complexity** (separate metrics)
- **Feature flag management** overhead
- **Database schema challenges** with mixed versions

### Option 5: Multi-Server Setup

Scale horizontally with multiple Hetzner servers for rolling deployments.

#### Option 5A: Load Balancer + Multiple App Servers
```
Internet → Hetzner LB → App Server 1 (v1.0)
                    → App Server 2 (v1.0)
                    → App Server 3 (v1.1) ← Deploy here first
```

#### Rolling Deployment Process:
1. Remove Server 1 from load balancer
2. Deploy new version to Server 1
3. Health check Server 1
4. Add Server 1 back to load balancer
5. Repeat for Server 2, then Server 3

**Pros:**
- True zero-downtime
- High availability
- Independent failure domains
- Can handle traffic spikes
- Easy horizontal scaling
- **Gradual rollout** with immediate rollback

**Cons:**
- **Higher cost** (€9.87+ per month for 3 servers)
- More complex infrastructure
- Database becomes shared bottleneck
- Increased monitoring complexity

### Option 6: Container-Based Deployment

Migrate to Docker containers for better deployment control.

#### Option 6A: Docker with Blue/Green
```bash
# Blue/Green with containers
docker run -d --name tribble-blue -p 8081:8080 ideal-tribble:v1.0
docker run -d --name tribble-green -p 8082:8080 ideal-tribble:v1.1

# Health check green container
curl http://localhost:8082/health

# Switch nginx upstream atomically
nginx -s reload  # Points to green container

# Remove blue container after successful deployment
docker stop tribble-blue && docker rm tribble-blue
```

#### Option 6B: Kubernetes (Advanced)
```yaml
# k8s deployment with rolling update
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ideal-tribble
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
```

**Pros (Docker):**
- Better isolation
- Consistent environments
- Built-in health checks
- Easy rollback (keep old container)

**Pros (Kubernetes):**
- Automated rolling updates
- Built-in health checks
- Automatic rollback on failure
- Horizontal pod autoscaling

**Cons:**
- Container orchestration complexity
- Additional monitoring needed
- Kubernetes: Significantly more complex infrastructure

## Advanced Deployment Strategies Comparison

| Strategy | Downtime | Risk Level | Rollback Speed | Infrastructure Cost | Complexity |
|----------|----------|------------|----------------|-------------------|------------|
| Current | 10s | Medium | Manual (5min) | €3.29 | Low |
| Rolling | 0s | Low | Fast (30s) | €3.29-9.87 | Medium |
| Blue/Green | 0s | Very Low | Instant (5s) | €6.58 | Medium-High |
| Canary | 0s | Very Low | Fast (1min) | €3.29-9.87 | High |
| Multi-Server Rolling | 0s | Low | Fast (2min) | €9.87+ | High |

## Database Considerations for Zero-Downtime

### Database Migration Strategies

#### Option A: Backward Compatible Migrations
```sql
-- Phase 1: Add new column (backward compatible)
ALTER TABLE matches ADD COLUMN new_field TEXT DEFAULT NULL;

-- Phase 2: Populate new column
UPDATE matches SET new_field = calculate_value(old_field);

-- Phase 3: Remove old column (after all instances updated)
ALTER TABLE matches DROP COLUMN old_field;
```

#### Option B: Database Versioning with Feature Flags
```go
type DatabaseVersion struct {
    Version int
    Features []string
}

func (s *Store) SupportsNewSchema() bool {
    return s.version >= 2
}
```

#### Option C: Read Replicas for Blue/Green
```
Blue Environment:  App → Primary DB
Green Environment: App → Read Replica → Promote to Primary
```

**Migration Strategies:**
- **Additive changes**: Add columns/tables, keep old ones
- **Feature toggles**: Support both old and new schema
- **Database versioning**: Track schema version in app
- **Rollback scripts**: Always prepare reverse migrations

## Recommended Implementation Phases

### Phase 1: Improved Single Server (Quick Win)
**Timeline:** 1-2 days
**Cost:** No additional cost
**Strategy:** Graceful shutdown + faster restarts

1. Implement graceful shutdown in Go application
2. Improve deployment script for faster restarts
3. Add binary health checks before switching

**Expected downtime reduction:** 10 seconds → 2-3 seconds

### Phase 2: Multi-Instance Single Server (Rolling)
**Timeline:** 3-5 days  
**Cost:** No additional cost
**Strategy:** Rolling deployment with multiple instances

1. Configure nginx upstream with multiple ports
2. Create systemd template services
3. Implement rolling deployment script
4. Add health check endpoints

**Expected result:** True zero-downtime

### Phase 3: Blue/Green Single Server (Enhanced Safety)
**Timeline:** 1 week
**Cost:** No additional cost
**Strategy:** Container-based blue/green

1. Containerize application
2. Implement blue/green switching with containers
3. Add automated health checks and rollback
4. Enhanced monitoring and alerting

**Expected result:** Zero-downtime + instant rollback

### Phase 4: Multi-Server High Availability
**Timeline:** 2-3 weeks
**Cost:** €6-12/month additional
**Strategy:** True blue/green or canary with multiple servers

1. Update Terraform for multiple servers
2. Add Hetzner Load Balancer
3. Implement blue/green or canary deployment
4. Comprehensive monitoring and alerting

**Expected result:** Zero-downtime + high availability + advanced deployment strategies

## Decision Matrix

| Solution | Complexity | Cost | Downtime | Risk Mitigation | Rollback Speed | Time to Implement |
|----------|------------|------|----------|----------------|----------------|-------------------|
| Current | Low | €3.29 | 10s | Low | 5min | - |
| Rolling (Single) | Medium | €3.29 | 0s | Medium | 30s | 5 days |
| Blue/Green (Single) | Medium | €3.29 | 0s | High | 5s | 1 week |
| Canary (Single) | High | €3.29 | 0s | Very High | 1min | 2 weeks |
| Blue/Green (Multi) | High | €6.58 | 0s | Very High | 5s | 3 weeks |
| Multi-Server Rolling | High | €9.87+ | 0s | High | 2min | 3 weeks |

## Monitoring Requirements

### Health Check Endpoints
```go
// Enhanced health check with dependencies
func (s *Server) setupHealthChecks() {
    s.router.Get("/health", s.healthCheck)           // Basic health
    s.router.Get("/health/ready", s.readinessCheck)  # Load balancer ready
    s.router.Get("/health/live", s.livenessCheck)    # Process alive
    s.router.Get("/version", s.versionCheck)         # Deployment tracking
}

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
    checks := map[string]bool{
        "database": s.checkDatabase(),
        "slack_api": s.checkSlackAPI(),
        "playtomic_api": s.checkPlaytomicAPI(),
    }
    
    status := 200
    for service, healthy := range checks {
        if !healthy {
            status = 503
            break
        }
    }
    
    response := map[string]interface{}{
        "status": status,
        "version": s.version,
        "checks": checks,
        "timestamp": time.Now().Unix(),
    }
    
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(response)
}
```

### Deployment Monitoring
- **Pre-deployment**: Health check current version
- **During deployment**: Monitor health checks of new version
- **Post-deployment**: Compare metrics (response time, error rate, throughput)
- **Alerting**: Automatic rollback triggers on failure thresholds

### Key Metrics to Monitor
```go
type DeploymentMetrics struct {
    ResponseTime    time.Duration
    ErrorRate       float64
    RequestsPerSec  int64
    DatabaseErrors  int64
    SlackAPIErrors  int64
    MemoryUsage     int64
    ActiveUsers     int64
}
```

## Application Code Changes Needed

### Graceful Shutdown
```go
func (s *Server) gracefulShutdown() {
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-c
        log.Info("Shutting down gracefully...")
        
        // Stop accepting new requests
        s.server.SetKeepAlivesEnabled(false)
        
        // Finish processing current requests
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        
        if err := s.server.Shutdown(ctx); err != nil {
            log.Fatal("Forced shutdown:", err)
        }
        
        // Close database connections
        s.store.Close()
        
        log.Info("Server shutdown complete")
        os.Exit(0)
    }()
}
```

### Version Management
```go
type Server struct {
    version     string
    deployedAt  time.Time
    gitCommit   string
    environment string
}

func (s *Server) versionHandler(w http.ResponseWriter, r *http.Request) {
    json.NewEncoder(w).Encode(map[string]interface{}{
        "version":     s.version,
        "deployed_at": s.deployedAt,
        "git_commit":  s.gitCommit,
        "environment": s.environment,
    })
}
```

## Conclusion

**Recommended progression:**

1. **Start with Phase 2** (Multi-instance rolling) for immediate zero-downtime benefit
2. **Move to Phase 3** (Blue/Green) for enhanced safety and instant rollback
3. **Consider Phase 4** (Multi-server) when traffic or availability requirements increase

**Key decision factors:**
- **Budget constraints**: Phases 1-3 have no additional infrastructure cost
- **Risk tolerance**: Blue/Green and Canary provide the safest deployments
- **Team complexity**: Rolling deployments are simpler than canary deployments
- **Traffic patterns**: High-traffic applications benefit more from canary deployments

The **blue/green approach** offers the best balance of safety (instant rollback) and simplicity for most applications, while **canary deployments** provide the highest level of risk mitigation for critical systems.