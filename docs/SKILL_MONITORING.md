# Production Monitoring Guide

## Overview

This guide describes the production monitoring setup for the refactored skill system.

## Key Metrics

### Performance Metrics

| Metric | Target | Alert Threshold | Description |
|--------|--------|-----------------|-------------|
| Render Time | < 50ms | > 100ms | Time to render skill component |
| Message Count | < 500 | > 1000 | Number of messages in session |
| Memory Usage | < 30MB | > 50MB | JS heap size per component |
| Bundle Size | < 150KB | > 200KB | Skill renderer bundle size |

### User Experience Metrics

| Metric | Target | Description |
|--------|--------|-------------|
| TTFT (Time To First Token) | < 2s | Time until first AI response |
| Progressive Generation | ✓ | Quiz/Presentation show progress |
| Error Rate | < 0.1% | Skill renderer crash rate |

## Monitoring Components

### 1. Performance Monitor

**Location:** `frontend/src/lib/monitoring/performanceMonitor.ts`

**Usage:**
```typescript
import { performanceMonitor } from '@/lib/monitoring';

// Track render performance
performanceMonitor.track('QuizRenderer', renderTime, messageCount);

// Get metrics
const avgTime = performanceMonitor.getAverageRenderTime('QuizRenderer');
```

**Features:**
- Automatic threshold checking
- Console warnings for performance issues
- Per-component metric history (last 100 renders)
- Memory usage tracking (Chrome only)

### 2. Analytics Queue

**Location:** `frontend/src/lib/monitoring/analytics.ts`

**Usage:**
```typescript
import { analyticsQueue } from '@/lib/monitoring';

// Track events
analyticsQueue.push({
  type: 'interaction',
  component: 'QuizRenderer',
  data: { questionId: 'q1', correct: true },
  timestamp: Date.now(),
});
```

**Features:**
- 10% sampling rate (configurable)
- Automatic batching (flush every 60s or 50 events)
- Graceful degradation on network errors
- Auto-flush on page unload

### 3. Error Boundary

**Location:** `frontend/src/components/SkillErrorBoundary.tsx`

**Usage:**
```tsx
<SkillErrorBoundary onError={(error, info) => {
  // Report to backend
  reportError(error, info);
}}>
  <QuizRenderer {...props} />
</SkillErrorBoundary>
```

**Features:**
- Catches React rendering errors
- Shows user-friendly fallback UI
- Logs error details for debugging
- Optional custom error handler

## Backend Integration

### Analytics Endpoint

**Endpoint:** `POST /api/v1/analytics/performance`

**Request Body:**
```json
{
  "events": [
    {
      "type": "render",
      "component": "QuizRenderer",
      "data": {
        "renderTime": 45,
        "messageCount": 12
      },
      "timestamp": 1709913600000
    }
  ]
}
```

**Implementation:** (To be added)
```go
// internal/delivery/http/handler/analytics.go
func (h *AnalyticsHandler) RecordPerformance(c *gin.Context) {
    var req struct {
        Events []PerformanceEvent `json:"events"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "Invalid request"})
        return
    }
    
    // Store in PostgreSQL or time-series DB
    h.analyticsRepo.BatchInsert(req.Events)
    c.JSON(200, gin.H{"status": "ok"})
}
```

## Dashboard Queries

### Average Render Time by Component

```sql
SELECT 
  component,
  AVG((data->>'renderTime')::float) as avg_render_time,
  COUNT(*) as sample_count
FROM analytics_events
WHERE type = 'render'
  AND timestamp > NOW() - INTERVAL '24 hours'
GROUP BY component
ORDER BY avg_render_time DESC;
```

### Error Rate by Component

```sql
SELECT 
  component,
  COUNT(*) as error_count,
  COUNT(*) * 100.0 / (
    SELECT COUNT(*) FROM analytics_events 
    WHERE component = ae.component
  ) as error_rate
FROM analytics_events ae
WHERE type = 'error'
  AND timestamp > NOW() - INTERVAL '24 hours'
GROUP BY component
ORDER BY error_rate DESC;
```

### P95 Render Time

```sql
SELECT 
  component,
  PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY (data->>'renderTime')::float) as p95_render_time
FROM analytics_events
WHERE type = 'render'
  AND timestamp > NOW() - INTERVAL '24 hours'
GROUP BY component;
```

## Grafana Dashboard (Optional)

### Panels

1. **Render Time Trend** (Time Series)
   - Query: Average render time per component over time
   - Alert: > 100ms for 5 minutes

2. **Message Count Distribution** (Histogram)
   - Query: Message count distribution across sessions
   - Alert: > 1000 messages in any session

3. **Error Rate** (Gauge)
   - Query: Error count / total events
   - Alert: > 0.1%

4. **Memory Usage** (Time Series)
   - Query: Average memory usage per component
   - Alert: > 50MB

### Alert Rules

```yaml
groups:
  - name: skill_performance
    interval: 1m
    rules:
      - alert: SlowSkillRenderer
        expr: avg_render_time_ms > 100
        for: 5m
        annotations:
          summary: "Skill renderer {{ $labels.component }} is slow"
          
      - alert: HighErrorRate
        expr: error_rate > 0.001
        for: 5m
        annotations:
          summary: "High error rate in {{ $labels.component }}"
```

## Monitoring Checklist

- [ ] Performance monitor integrated in all refactored renderers
- [ ] Error boundaries wrap all skill renderers
- [ ] Analytics endpoint implemented in backend
- [ ] Database schema created for analytics events
- [ ] Grafana dashboard configured (optional)
- [ ] Alert rules configured (optional)
- [ ] Weekly performance review scheduled

## Troubleshooting

### High Render Time

1. Check message count - use VirtualizedMessageList if > 100
2. Profile with React DevTools
3. Check for unnecessary re-renders
4. Verify memoization of expensive computations

### High Memory Usage

1. Check for memory leaks in WebSocket handlers
2. Verify cleanup in useEffect hooks
3. Limit message history (maxMessages option)
4. Use React.memo for expensive components

### High Error Rate

1. Check browser console for error patterns
2. Review error boundary logs
3. Verify WebSocket connection stability
4. Check for malformed backend responses

## Next Steps

1. Implement backend analytics endpoint
2. Create PostgreSQL schema for events
3. Set up Grafana dashboard (optional)
4. Configure alert rules
5. Schedule weekly performance reviews
