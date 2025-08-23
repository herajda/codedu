# UUID Migration Guide

This document outlines the comprehensive migration from integer IDs to UUIDs in the code-grader project.

## Migration Status

### ✅ Completed
1. Database schema updated to use UUID columns with `uuid_generate_v4()`
2. Go model structs updated to use `uuid.UUID` types
3. Added `github.com/google/uuid` dependency

### 🔄 In Progress  
4. Backend handlers and API endpoints
5. Authentication system (JWT tokens)

### ⏳ Pending
6. Frontend API calls and routing
7. Svelte route parameters `[id]` → `[uuid]`
8. Testing and verification

## Key Changes Made

### Database Schema (schema.sql)
- Added `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`
- Changed all `id SERIAL PRIMARY KEY` to `id UUID PRIMARY KEY DEFAULT uuid_generate_v4()`
- Updated all foreign key references to use UUID types

### Go Models (models.go)
- Updated all struct ID fields to `uuid.UUID`
- Added UUID import
- Updated function signatures that accept/return IDs

## Next Steps

### Authentication System
Need to update `auth.go`:
- `issueTokens(uid uuid.UUID, role string)`
- JWT claims to store UUID strings
- Update authentication middleware

### Backend Handlers
Need to replace ~60+ instances of:
```go
id, err := strconv.Atoi(c.Param("id"))
```
with:
```go
id, err := uuid.Parse(c.Param("id"))
```

### Frontend
- Update all API calls to send/receive UUID strings
- Update Svelte route parameters from `[id]` to handle UUIDs
- Update any integer ID parsing in TypeScript/JavaScript

## Risks and Considerations

1. **Breaking Change**: This is a complete breaking change requiring database recreation
2. **Performance**: UUIDs are larger than integers, may impact performance
3. **URL Length**: URLs will be longer with UUID parameters
4. **Client-side**: Frontend needs to handle UUID strings instead of numbers

## Rollback Strategy

If needed to rollback:
1. Restore database schema to use SERIAL PRIMARY KEY
2. Revert Go model changes
3. Revert handler changes
4. Revert frontend changes

## Testing Strategy

1. Create new database with UUID schema
2. Test backend compilation
3. Test API endpoints with UUID parameters
4. Test frontend with UUID routing
5. Verify all functionality works end-to-end
