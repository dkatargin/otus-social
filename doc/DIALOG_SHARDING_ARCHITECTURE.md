# Dialog System Sharding Architecture

## Overview

The OTUS Social Network implements a comprehensive dialog system with horizontal sharding for message storage. This architecture enables the system to handle millions of messages with optimal performance and scalability.

## Sharding Strategy

### Core Principles

1. **Deterministic Distribution**: All messages between any pair of users always go to the same shard
2. **Even Load Distribution**: Enhanced hash algorithm ensures balanced shard utilization
3. **Hybrid System**: Automatic distribution with manual override capabilities
4. **Horizontal Scalability**: Each shard can be hosted on separate database servers

### Hash Algorithm

The system uses an enhanced hash function for optimal distribution:

```go
func calculateShard(userID1, userID2 int) int {
    minID := min(userID1, userID2)
    maxID := max(userID1, userID2)
    
    // Enhanced hash function with good distribution properties
    hash := uint64(minID)*2654435761 + uint64(maxID)*2654435789
    hash = hash ^ (hash >> 16)
    hash = hash * 2654435761
    hash = hash ^ (hash >> 16)
    
    return int(hash % uint64(NumShards))
}
```

**Algorithm Features:**
- Uses prime number multiplication for better distribution
- XOR and bit shifting to reduce collisions
- Consistent ordering (min/max) ensures deterministic placement
- O(1) computation time

## Database Schema

### Shard Tables

The system creates 4 shard tables by default:

```sql
-- Shard 0
CREATE TABLE messages_0 (
    id SERIAL PRIMARY KEY,
    from_user_id INTEGER NOT NULL,
    to_user_id INTEGER NOT NULL,
    text TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Shard 1, 2, 3 follow the same pattern
```

### Manual Override Table

```sql
CREATE TABLE shard_map (
    user1 INTEGER NOT NULL,
    user2 INTEGER NOT NULL,
    shard_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user1, user2)
);
```

**Use Cases for Manual Override:**
- "Lady Gaga Effect" - Popular users causing hotspots
- Geographic distribution optimization
- Load balancing adjustments
- Maintenance and migration scenarios

## Architecture Components

### Handler Layer (`api/handlers/dialog.go`)

**Send Message Flow:**
1. Extract user IDs from request
2. Check `shard_map` for manual assignment
3. If no manual assignment, calculate shard using hash algorithm
4. Insert message into appropriate `messages_N` table
5. Return success response

**Get Dialog Flow:**
1. Determine shard for user pair
2. Query appropriate `messages_N` table
3. Apply pagination and sorting
4. Return message history

### Performance Optimizations

1. **Single Query Lookup**: One query to determine shard, one to fetch/store data
2. **Index Optimization**: Indexes on (from_user_id, to_user_id, created_at)
3. **Connection Pooling**: Efficient database connection management
4. **Prepared Statements**: Reduced SQL parsing overhead

## Scalability Features

### Horizontal Scaling

Each shard can be:
- Hosted on separate database servers
- Replicated independently
- Backed up on different schedules
- Optimized with shard-specific settings

### Load Isolation

- Problems in one shard don't affect others
- Independent monitoring and alerting per shard
- Targeted maintenance and upgrades
- Granular performance tuning

### Capacity Planning

Current configuration supports:
- **4 shards** with automatic distribution
- **Unlimited manual overrides** via shard_map
- **Easy expansion** by adding new message tables
- **Migration tools** for redistributing data

## Testing Strategy

### Distribution Tests (`tests/07_sharding_distribution_test.go`)

1. **Even Distribution Test**:
   - Creates messages between random user pairs
   - Verifies messages are distributed across shards
   - Measures distribution variance

2. **Consistency Test**:
   - Ensures all messages between same users go to same shard
   - Tests both directions (A→B and B→A)
   - Validates deterministic behavior

3. **Manual Override Test**:
   - Tests shard_map functionality
   - Verifies override takes precedence over hash
   - Tests edge cases and error conditions

### Performance Tests (`tests/06_sharding_test.go`)

1. **Concurrent Message Creation**:
   - Simulates high load with parallel goroutines
   - Measures throughput and latency
   - Tests under various load patterns

2. **Query Performance**:
   - Benchmarks dialog retrieval across shards
   - Tests pagination performance
   - Measures query response times

## Monitoring and Debugging

### Debug Tools

1. **`debug_sharding.go`**: 
   - Shows message distribution across shards
   - Displays shard_map overrides
   - Calculates distribution statistics

2. **`debug_messages.go`**:
   - Lists all messages with shard information
   - Shows detailed message metadata
   - Helps troubleshoot distribution issues

### Metrics to Monitor

- **Messages per shard**: Ensure even distribution
- **Query latency per shard**: Identify performance bottlenecks
- **shard_map utilization**: Track manual overrides
- **Database connections per shard**: Monitor resource usage

## Migration and Maintenance

### Adding New Shards

1. Create new `messages_N` table
2. Update NumShards constant
3. Run migration tool to redistribute existing data
4. Update monitoring and alerting

### Rebalancing Data

1. Analyze current distribution
2. Identify hotspots or imbalanced shards
3. Use shard_map to manually reassign heavy user pairs
4. Monitor impact and adjust as needed

### Backup Strategy

- **Per-shard backups**: Independent backup schedules
- **shard_map backup**: Critical for system recovery
- **Cross-shard consistency**: Ensure backup coordination

## Production Considerations

### Security

- **SQL Injection Prevention**: All queries use prepared statements
- **User ID Validation**: Strict input validation
- **Access Control**: Authentication required for all dialog operations

### Error Handling

- **Graceful Degradation**: Fallback to read-only mode if shards unavailable
- **Retry Logic**: Automatic retry for transient failures
- **Circuit Breakers**: Prevent cascade failures

### Performance

- **Connection Pooling**: Optimized database connections
- **Query Optimization**: Efficient indexes and query plans
- **Caching**: Consider adding Redis cache for frequently accessed dialogs

## Future Enhancements

### Planned Improvements

1. **Auto-rebalancing**: Automatic detection and resolution of hotspots
2. **Geographic Sharding**: Shard based on user location for reduced latency
3. **Read Replicas**: Add read replicas per shard for read scaling
4. **Compression**: Implement message compression for storage efficiency

### Scalability Roadmap

- **16 shards**: Next expansion target for higher throughput
- **Cross-datacenter**: Deploy shards across multiple datacenters
- **Event Sourcing**: Consider event-driven architecture for audit trails
- **Message Archiving**: Archive old messages to cold storage

---

This sharding architecture provides a solid foundation for scaling the dialog system to handle millions of users and billions of messages while maintaining performance and reliability.