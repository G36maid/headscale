# Headscale API Documentation

This document describes all available gRPC and REST APIs in this modified Headscale server.

## Overview

Headscale provides both gRPC and REST API endpoints using gRPC-Gateway for automatic HTTP/JSON to gRPC translation. All REST API endpoints are under the `/api/v1/` path prefix and require Bearer token authentication.

## Authentication

All API calls require authentication using a Bearer token in the Authorization header:
```
Authorization: Bearer <your-api-key>
```

## gRPC Service: HeadscaleService

### User Management

#### GetUser
- **gRPC**: `GetUser(GetUserRequest) -> GetUserResponse`
- **REST**: `GET /api/v1/user/{name}`
- **Description**: Retrieve information about a specific user

#### CreateUser
- **gRPC**: `CreateUser(CreateUserRequest) -> CreateUserResponse`
- **REST**: `POST /api/v1/user`
- **Description**: Create a new user

#### RenameUser
- **gRPC**: `RenameUser(RenameUserRequest) -> RenameUserResponse`
- **REST**: `POST /api/v1/user/{old_name}/rename/{new_name}`
- **Description**: Rename an existing user

#### DeleteUser
- **gRPC**: `DeleteUser(DeleteUserRequest) -> DeleteUserResponse`
- **REST**: `DELETE /api/v1/user/{name}`
- **Description**: Delete a user

#### ListUsers
- **gRPC**: `ListUsers(ListUsersRequest) -> ListUsersResponse`
- **REST**: `GET /api/v1/user`
- **Description**: List all users

### PreAuth Keys Management

#### CreatePreAuthKey
- **gRPC**: `CreatePreAuthKey(CreatePreAuthKeyRequest) -> CreatePreAuthKeyResponse`
- **REST**: `POST /api/v1/preauthkey`
- **Description**: Create a new pre-authentication key

#### ExpirePreAuthKey
- **gRPC**: `ExpirePreAuthKey(ExpirePreAuthKeyRequest) -> ExpirePreAuthKeyResponse`
- **REST**: `POST /api/v1/preauthkey/expire`
- **Description**: Expire a pre-authentication key

#### EnablePreAuthKeys
- **gRPC**: `EnablePreAuthKeys(PreAuthKeysRequest) -> PreAuthKeysResponse`
- **REST**: `POST /api/v1/preauthkeys/enable`
- **Description**: Enable multiple pre-authentication keys

#### RemovePreAuthKeys
- **gRPC**: `RemovePreAuthKeys(PreAuthKeysRequest) -> PreAuthKeysResponse`
- **REST**: `POST /api/v1/preauthkeys/remove`
- **Description**: Remove multiple pre-authentication keys

#### ListPreAuthKeys
- **gRPC**: `ListPreAuthKeys(ListPreAuthKeysRequest) -> ListPreAuthKeysResponse`
- **REST**: `GET /api/v1/preauthkey`
- **Description**: List all pre-authentication keys

#### LockPreAuthKeyTagLock
- **gRPC**: `LockPreAuthKeyTagLock(PreAuthKeyTagLockRequest) -> PreAuthKeyTagLockResponse`
- **REST**: `POST /api/v1/preauthkey/lock`
- **Description**: Lock pre-authentication key tag lock

#### UnlockPreAuthKeyTagLock
- **gRPC**: `UnlockPreAuthKeyTagLock(PreAuthKeyTagLockRequest) -> PreAuthKeyTagLockResponse`
- **REST**: `POST /api/v1/preauthkey/unlock`
- **Description**: Unlock pre-authentication key tag lock

### Node Management

#### DebugCreateNode
- **gRPC**: `DebugCreateNode(DebugCreateNodeRequest) -> DebugCreateNodeResponse`
- **REST**: `POST /api/v1/debug/node`
- **Description**: Create a node for debugging purposes

#### GetNode
- **gRPC**: `GetNode(GetNodeRequest) -> GetNodeResponse`
- **REST**: `GET /api/v1/node/{node_id}`
- **Description**: Retrieve information about a specific node

#### SetTags
- **gRPC**: `SetTags(SetTagsRequest) -> SetTagsResponse`
- **REST**: `POST /api/v1/node/{node_id}/tags`
- **Description**: Set tags for a node

#### RegisterNode
- **gRPC**: `RegisterNode(RegisterNodeRequest) -> RegisterNodeResponse`
- **REST**: `POST /api/v1/node/register`
- **Description**: Register a new node

#### DeleteNode
- **gRPC**: `DeleteNode(DeleteNodeRequest) -> DeleteNodeResponse`
- **REST**: `DELETE /api/v1/node/{node_id}`
- **Description**: Delete a node

#### ExpireNode
- **gRPC**: `ExpireNode(ExpireNodeRequest) -> ExpireNodeResponse`
- **REST**: `POST /api/v1/node/{node_id}/expire`
- **Description**: Expire a node

#### RenameNode
- **gRPC**: `RenameNode(RenameNodeRequest) -> RenameNodeResponse`
- **REST**: `POST /api/v1/node/{node_id}/rename/{new_name}`
- **Description**: Rename a node

#### ListNodes
- **gRPC**: `ListNodes(ListNodesRequest) -> ListNodesResponse`
- **REST**: `GET /api/v1/node`
- **Description**: List all nodes

#### MoveNode
- **gRPC**: `MoveNode(MoveNodeRequest) -> MoveNodeResponse`
- **REST**: `POST /api/v1/node/{node_id}/user`
- **Description**: Move a node to a different user

#### BackfillNodeIPs
- **gRPC**: `BackfillNodeIPs(BackfillNodeIPsRequest) -> BackfillNodeIPsResponse`
- **REST**: `POST /api/v1/node/backfillips`
- **Description**: Backfill node IP addresses

### Route Management

#### GetRoutes
- **gRPC**: `GetRoutes(GetRoutesRequest) -> GetRoutesResponse`
- **REST**: `GET /api/v1/routes`
- **Description**: Get all routes

#### EnableRoute
- **gRPC**: `EnableRoute(EnableRouteRequest) -> EnableRouteResponse`
- **REST**: `POST /api/v1/routes/{route_id}/enable`
- **Description**: Enable a specific route

#### DisableRoute
- **gRPC**: `DisableRoute(DisableRouteRequest) -> DisableRouteResponse`
- **REST**: `POST /api/v1/routes/{route_id}/disable`
- **Description**: Disable a specific route

#### GetNodeRoutes
- **gRPC**: `GetNodeRoutes(GetNodeRoutesRequest) -> GetNodeRoutesResponse`
- **REST**: `GET /api/v1/node/{node_id}/routes`
- **Description**: Get routes for a specific node

#### DeleteRoute
- **gRPC**: `DeleteRoute(DeleteRouteRequest) -> DeleteRouteResponse`
- **REST**: `DELETE /api/v1/routes/{route_id}`
- **Description**: Delete a route

### API Key Management

#### CreateApiKey
- **gRPC**: `CreateApiKey(CreateApiKeyRequest) -> CreateApiKeyResponse`
- **REST**: `POST /api/v1/apikey`
- **Description**: Create a new API key

#### ExpireApiKey
- **gRPC**: `ExpireApiKey(ExpireApiKeyRequest) -> ExpireApiKeyResponse`
- **REST**: `POST /api/v1/apikey/expire`
- **Description**: Expire an API key

#### ListApiKeys
- **gRPC**: `ListApiKeys(ListApiKeysRequest) -> ListApiKeysResponse`
- **REST**: `GET /api/v1/apikey`
- **Description**: List all API keys

#### DeleteApiKey
- **gRPC**: `DeleteApiKey(DeleteApiKeyRequest) -> DeleteApiKeyResponse`
- **REST**: `DELETE /api/v1/apikey/{prefix}`
- **Description**: Delete an API key

### Policy Management

#### GetPolicy
- **gRPC**: `GetPolicy(GetPolicyRequest) -> GetPolicyResponse`
- **REST**: `GET /api/v1/policy`
- **Description**: Get the current ACL policy

#### SetPolicy
- **gRPC**: `SetPolicy(SetPolicyRequest) -> SetPolicyResponse`
- **REST**: `PUT /api/v1/policy`
- **Description**: Set/update the ACL policy

### ACL Management

#### ACLCreateGroup
- **gRPC**: `ACLCreateGroup(ACLGroupRequest) -> ACLGroupResponse`
- **REST**: `POST /api/v1/acl/group/{group_name}`
- **Description**: Create an ACL group

#### ACLGroupAddUser
- **gRPC**: `ACLGroupAddUser(ACLGroupUserRequest) -> ACLGroupUserResponse`
- **REST**: `POST /api/v1/acl/group/{group_name}/{username}`
- **Description**: Add a user to an ACL group

#### ACLGroupRemoveUser
- **gRPC**: `ACLGroupRemoveUser(ACLGroupUserRequest) -> ACLGroupUserResponse`
- **REST**: `DELETE /api/v1/acl/group/{group_name}/{username}`
- **Description**: Remove a user from an ACL group

#### ACLRemoveGroup
- **gRPC**: `ACLRemoveGroup(ACLGroupRequest) -> ACLGroupResponse`
- **REST**: `DELETE /api/v1/acl/group/{group_name}`
- **Description**: Remove an ACL group

#### ACLBindHostname
- **gRPC**: `ACLBindHostname(ACLHostnameRequest) -> ACLHostnameResponse`
- **REST**: `POST /api/v1/acl/host/{hostname}`
- **Description**: Bind a hostname in ACL

#### ACLUpdateHostname
- **gRPC**: `ACLUpdateHostname(ACLHostnameRequest) -> ACLHostnameResponse`
- **REST**: `PATCH /api/v1/acl/host/{hostname}`
- **Description**: Update a hostname binding in ACL

#### ACLRemoveHostname
- **gRPC**: `ACLRemoveHostname(ACLHostnameRequest) -> ACLHostnameResponse`
- **REST**: `DELETE /api/v1/acl/host/{hostname}`
- **Description**: Remove a hostname binding from ACL

#### ACLCreateTag
- **gRPC**: `ACLCreateTag(ACLTagRequest) -> ACLTagResponse`
- **REST**: `POST /api/v1/acl/tag/{tag}`
- **Description**: Create an ACL tag

#### ACLRemoveTag
- **gRPC**: `ACLRemoveTag(ACLTagRequest) -> ACLTagResponse`
- **REST**: `DELETE /api/v1/acl/tag/{tag}`
- **Description**: Remove an ACL tag

#### ACLCreateRule
- **gRPC**: `ACLCreateRule(ACLRuleRequest) -> ACLRuleResponse`
- **REST**: `POST /api/v1/acl/accept`
- **Description**: Create an ACL accept rule

#### ACLRemoveRule
- **gRPC**: `ACLRemoveRule(ACLRuleRequest) -> ACLRuleResponse`
- **REST**: `POST /api/v1/acl/deny`
- **Description**: Create an ACL deny rule

#### ACLForceRemoveRule
- **gRPC**: `ACLForceRemoveRule(ACLRuleRequest) -> ACLRuleResponse`
- **REST**: `POST /api/v1/acl/remove`
- **Description**: Force remove an ACL rule

#### ACLRuleInclude
- **gRPC**: `ACLRuleInclude(ACLRuleRequest) -> ACLRuleResponse`
- **REST**: `POST /api/v1/acl/include`
- **Description**: Include rule in ACL

#### ACLRuleExclude
- **gRPC**: `ACLRuleExclude(ACLRuleRequest) -> ACLRuleResponse`
- **REST**: `POST /api/v1/acl/exclude`
- **Description**: Exclude rule from ACL

#### ACLCtrl
- **gRPC**: `ACLCtrl(ACLCtrlRequest) -> ACLCtrlResponse`
- **REST**: `POST /api/v1/acl/ctrl/{action}`
- **Description**: Control ACL operations

## Additional HTTP Endpoints (Non-gRPC)

These endpoints are handled directly by HTTP handlers and not through gRPC-Gateway:

### Core Endpoints
- **POST** `/ts2021-upgrade` - Noise protocol upgrade handler
- **GET** `/health` - Health check endpoint
- **GET** `/key` - Get Headscale public key
- **GET** `/register/{mkey}` - Web registration endpoint

### OIDC Endpoints
- **GET** `/oidc/register/{mkey}` - OIDC registration endpoint
- **GET** `/oidc/callback` - OIDC callback endpoint

### Configuration Endpoints
- **GET** `/apple` - Apple configuration message
- **GET** `/apple/{platform}` - Apple platform-specific configuration
- **GET** `/windows` - Windows configuration message

### Documentation
- **GET** `/swagger` - Swagger UI
- **GET** `/swagger/v1/openapiv2.json` - OpenAPI v2 specification

### DERP (Optional)
If DERP server is enabled:
- **GET/POST** `/derp` - DERP server handler
- **GET** `/derp/probe` - DERP probe handler
- **GET** `/bootstrap-dns` - DERP bootstrap DNS handler

## Error Handling

All API endpoints return standard HTTP status codes:
- **200** - Success
- **400** - Bad Request
- **401** - Unauthorized (missing or invalid API key)
- **404** - Not Found
- **500** - Internal Server Error

Error responses include JSON-formatted error messages with details about the failure.

## Rate Limiting

Rate limiting may be applied to prevent abuse. Check the response headers for rate limit information.

## Protocol Support

- **gRPC**: Native gRPC calls on the gRPC port
- **gRPC-Web**: gRPC-Web calls for browser compatibility
- **REST/JSON**: HTTP REST API with JSON payloads via gRPC-Gateway
- **Noise Protocol**: TS2021 protocol support for Tailscale clients