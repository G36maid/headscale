# PreAuthKey API Specification

This document provides a comprehensive specification for the PreAuthKey-related gRPC and HTTP REST APIs in this headscale fork.

## Overview

The PreAuthKey APIs provide functionality for managing pre-authentication keys used by Tailscale nodes to join the network. This fork extends the standard headscale APIs with bulk operations, UUID-based organization, and advanced node lifecycle management.

## Key Concepts

- **SGU UUID**: Security Group User UUID - identifies a logical user group
- **Client UUID**: Individual client/device identifier within a user group
- **ACL Tags**: Tags in format `tag:sguser_<uuid>` or `tag:client_<uuid>` for organization
- **Tag Locking**: Mechanism to prevent operations on specific SGUser tags

## Authentication

All APIs require authentication via API key in the `Authorization` header:
```
Authorization: Bearer <api_key>
```

## Standard PreAuthKey APIs

### 1. Create PreAuthKey

**Endpoint**: `POST /api/v1/preauthkey`

**Description**: Creates a new pre-authentication key for a user with optional ACL tags and expiration.

**Request Body**:
```json
{
  "user": "john-doe",
  "reusable": true,
  "ephemeral": false,
  "expiration": {
    "seconds": 1767225599,
    "nanos": 0
  },
  "acl_tags": [
    "tag:sguser_550e8400-e29b-41d4-a716-446655440000",
    "tag:client_123e4567-e89b-12d3-a456-426614174000",
    "tag:vpnip_100.64.0.50"]
}
```

**Request Parameters**:
- `user` (string, required): Username to create the key for
- `reusable` (boolean, optional): Whether the key can be used multiple times. Default: false
- `ephemeral` (boolean, optional): Whether nodes using this key are ephemeral. Default: false
- `expiration` (timestamp object, optional): When the key expires. If not set, key doesn't expire
- `acl_tags` (array of strings, optional): ACL tags to assign to the key

**Timestamp Format**: All timestamps use Google Protocol Buffers timestamp format:
```json
{
  "seconds": 1735689599,  // Unix timestamp in seconds
  "nanos": 0              // Nanoseconds component (0-999,999,999)
}
```

**Response**:
```json
{
  "pre_auth_key": {
    "user": "john-doe",
    "id": "123",
    "key": "nodekey:abc123def456...",
    "reusable": true,
    "ephemeral": false,
    "used": false,
    "expiration": {
      "seconds": 1767225599,
      "nanos": 0
    },
    "created_at": "2024-01-15T10:30:00Z",
    "acl_tags": ["tag:sguser_550e8400-e29b-41d4-a716-446655440000", "tag:client_123e4567-e89b-12d3-a456-426614174000"]
  }
}
```

**Error Responses**:
- `400 Bad Request`: Invalid ACL tag format (must start with "tag:")
- `404 Not Found`: User not found
- `500 Internal Server Error`: Database error

### 2. List PreAuthKeys

**Endpoint**: `GET /api/v1/preauthkey?user=<username>`

**Description**: Lists all pre-authentication keys for a specific user.

**Query Parameters**:
- `user` (string, required): Username to list keys for

**Response**:
```json
{
  "pre_auth_keys": [
    {
      "user": "john-doe",
      "id": "123",
      "key": "nodekey:abc123def456...",
      "reusable": true,
      "ephemeral": false,
      "used": false,
      "expiration": {
        "seconds": 1767225599,
        "nanos": 0
      },
      "created_at": "2024-01-15T10:30:00Z",
      "acl_tags": ["tag:sguser_550e8400-e29b-41d4-a716-446655440000"]
    },
    {
      "user": "john-doe",
      "id": "124",
      "key": "nodekey:def456ghi789...",
      "reusable": false,
      "ephemeral": true,
      "used": true,
      "expiration": null,
      "created_at": "2024-01-14T09:15:00Z",
      "acl_tags": []
    }
  ]
}
```

**Note**: Results are sorted by key ID in ascending order.

### 3. Expire PreAuthKey

**Endpoint**: `POST /api/v1/preauthkey/expire`

**Description**: Immediately expires a specific pre-authentication key.

**Request Body**:
```json
{
  "user": "john-doe",
  "key": "nodekey:abc123def456..."
}
```

**Response**:
```json
{}
```

**Error Responses**:
- `404 Not Found`: Key not found or user mismatch
- `500 Internal Server Error`: Database error

## Bulk Operations APIs

### 4. Enable PreAuthKeys (Bulk)

**Endpoint**: `POST /api/v1/preauthkeys/enable`

**Description**: Bulk enables pre-authentication keys by setting `reusable=true` on keys matching the provided SGU or Client UUIDs.

**Request Body** (Option 1 - by SGU UUIDs):
```json
{
  "sgu_uuids": ["550e8400-e29b-41d4-a716-446655440000", "6ba7b810-9dad-11d1-80b4-00c04fd430c8"]
}
```

**Request Body** (Option 2 - by Client UUIDs):
```json
{
  "client_uuids": ["123e4567-e89b-12d3-a456-426614174000", "987fcdeb-51a2-43d1-9b12-345678901234"]
}
```

**Request Parameters**:
- `sgu_uuids` (array of strings, optional): SGU UUIDs to match against `tag:sguser_<uuid>` tags
- `client_uuids` (array of strings, optional): Client UUIDs to match against `tag:client_<uuid>` tags

**Behavior**:
- Finds all PreAuthKeys with ACL tags matching the provided UUIDs
- Sets `reusable=true` on all matching keys
- Must provide either `sgu_uuids` OR `client_uuids`, not both

**Response**:
```json
{}
```

### 5. Remove PreAuthKeys (Bulk)

**Endpoint**: `POST /api/v1/preauthkeys/remove`

**Description**: Bulk removes pre-authentication keys with different behaviors based on UUID type.

**Request Body** (Option 1 - by SGU UUIDs):
```json
{
  "sgu_uuids": ["550e8400-e29b-41d4-a716-446655440000"]
}
```

**Request Body** (Option 2 - by Client UUIDs):
```json
{
  "client_uuids": ["123e4567-e89b-12d3-a456-426614174000"]
}
```

**Behavior Differences**:

#### By SGU UUIDs:
1. Finds PreAuthKeys with matching `tag:sguser_<uuid>` tags
2. Destroys the keys immediately
3. No impact on existing nodes

#### By Client UUIDs (Advanced Workflow):
1. Finds PreAuthKeys with matching `tag:client_<uuid>` tags
2. **Disables** keys (sets `reusable=false`, `used=true`)
3. Finds all nodes that used these auth keys
4. **Expires** all related nodes (via internal `ExpireNodes` call)
5. **Destroys** the PreAuthKeys
6. **Asynchronously deletes** nodes after 10-second delay (allows graceful shutdown)

**Response**:
```json
{}
```

**Example with Client UUIDs - Complete Workflow**:
```bash
# Request
curl -X POST /api/v1/preauthkeys/remove \
  -H "Authorization: Bearer <api_key>" \
  -d '{"client_uuids": ["123e4567-e89b-12d3-a456-426614174000"]}'

# What happens:
# 1. Keys with tag:client_123e4567-e89b-12d3-a456-426614174000 are disabled
# 2. Nodes using those keys are expired immediately
# 3. Keys are destroyed
# 4. After 10 seconds, nodes are permanently deleted
```

## Tag Locking APIs

### 6. Lock PreAuthKey Tag

**Endpoint**: `POST /api/v1/preauthkey/lock`

**Description**: Locks a specific SGUser tag to prevent operations on associated PreAuthKeys.

**Request Body**:
```json
{
  "sgu_uuid": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response**:
```json
{}
```

**Behavior**:
- Creates or updates a lock record for `tag:sguser_<uuid>`
- Prevents PreAuthKey operations when the tag is locked
- Lock validation occurs during key usage/validation

### 7. Unlock PreAuthKey Tag

**Endpoint**: `POST /api/v1/preauthkey/unlock`

**Description**: Unlocks a specific SGUser tag to allow operations on associated PreAuthKeys.

**Request Body**:
```json
{
  "sgu_uuid": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response**:
```json
{}
```

## Internal/gRPC-Only APIs

### ExpireNodes (Bulk)

**Note**: This API is implemented server-side and used internally (e.g., from RemovePreAuthKeys), but not exposed via HTTP gateway.

**Description**: Bulk expires nodes by SGU or Client UUIDs with optional recovery interval.

**gRPC Request**:
```protobuf
message ExpireNodesRequest {
  int32 recovery_interval = 1;  // Seconds after which to re-enable keys
  repeated string sgu_uuids = 2;
  repeated string client_uuids = 3;
}
```

**Behavior**:
- Disables associated PreAuthKeys
- Expires all nodes using those keys
- Optionally re-enables keys after recovery_interval seconds
- Does not automatically "unexpire" nodes

## Error Handling

### Common HTTP Status Codes

- `200 OK`: Successful operation
- `400 Bad Request`: Invalid request parameters or body
- `401 Unauthorized`: Missing or invalid API key
- `404 Not Found`: Resource not found (user, key, etc.)
- `500 Internal Server Error`: Server-side error

### Common Error Response Format
```json
{
  "error": "error description",
  "code": "ERROR_CODE",
  "details": "additional details"
}
```

## Data Models

### PreAuthKey Object
```json
{
  "user": "string",           // Username that owns the key
  "id": "string",             // Unique key identifier
  "key": "string",            // The actual pre-auth key value
  "reusable": "boolean",      // Can be used multiple times
  "ephemeral": "boolean",     // Creates ephemeral nodes
  "used": "boolean",          // Has been used at least once
  "expiration": {             // When key expires (null = no expiration)
    "seconds": "int64",       // Unix timestamp seconds
    "nanos": "int32"          // Nanoseconds component
  },
  "created_at": "timestamp",  // When key was created
  "acl_tags": ["string"]      // Associated ACL tags
}
```

### PreAuthKeysRequest Object
```json
{
  "sgu_uuids": ["string"],     // SGU UUIDs (optional)
  "client_uuids": ["string"]   // Client UUIDs (optional)
}
```

## Timestamp Examples

### Creating a key that expires on December 31, 2025 at 23:59:59 UTC:
```bash
curl -X POST /api/v1/preauthkey \
  -H "Authorization: Bearer <api_key>" \
  -d '{
    "user": "john-doe",
    "reusable": true,
    "expiration": {
      "seconds": 1767225599,
      "nanos": 0
    }
  }'
```

### Creating a key that never expires:
```bash
curl -X POST /api/v1/preauthkey \
  -H "Authorization: Bearer <api_key>" \
  -d '{
    "user": "john-doe",
    "reusable": true
  }'
```

### Converting from ISO 8601 to timestamp object:
- ISO 8601: `2025-12-31T23:59:59Z`
- Unix timestamp: `1767225599` seconds
- Timestamp object: `{"seconds": 1767225599, "nanos": 0}`

## Usage Examples

### Complete Client Lifecycle Management

1. **Create keys for a new client**:
```bash
curl -X POST /api/v1/preauthkey \
  -H "Authorization: Bearer <api_key>" \
  -d '{
    "user": "client-team",
    "reusable": true,
    "acl_tags": ["tag:sguser_group-a", "tag:client_device-123"]
  }'
```

2. **Lock the group to prevent changes**:
```bash
curl -X POST /api/v1/preauthkey/lock \
  -H "Authorization: Bearer <api_key>" \
  -d '{"sgu_uuid": "group-a"}'
```

3. **Remove client and cleanup**:
```bash
curl -X POST /api/v1/preauthkeys/remove \
  -H "Authorization: Bearer <api_key>" \
  -d '{"client_uuids": ["device-123"]}'
```

4. **Unlock the group**:
```bash
curl -X POST /api/v1/preauthkey/unlock \
  -H "Authorization: Bearer <api_key>" \
  -d '{"sgu_uuid": "group-a"}'
```

## Security Considerations

1. **API Key Protection**: Ensure API keys are stored securely and transmitted over HTTPS
2. **UUID Validation**: UUIDs should be validated to prevent injection attacks
3. **Rate Limiting**: Consider implementing rate limiting for bulk operations
4. **Audit Logging**: All operations should be logged for security auditing
5. **Tag Validation**: ACL tags must follow the `tag:` prefix format
6. **Lock Conflicts**: Check for existing locks before performing operations

## Performance Notes

- Bulk operations are optimized for handling multiple keys/nodes efficiently
- The 10-second delay in `RemovePreAuthKeysByClientUuids` allows graceful node shutdown
- Tag locking uses database-level locking to prevent race conditions
- Large UUID lists should be paginated for better performance
