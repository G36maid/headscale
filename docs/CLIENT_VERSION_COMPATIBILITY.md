# Headscale Tailscale Client Version Compatibility

This document outlines the version-specific attributes and features that Headscale supports for different Tailscale client versions, based on their `CapabilityVersion` (CapVer).

## Overview

Headscale tracks client capabilities using the `CapabilityVersion` parameter sent by Tailscale clients. This allows the server to provide version-appropriate features and maintain backward compatibility.

## Minimum Requirements

- **Minimum Supported CapVer**: 61 (Tailscale v1.42+)
- **TS2021 Protocol Support**: CapVer 39+ (Tailscale v1.30+)
- **EarlyNoise Support**: CapVer 49+ (Protocol version 49)

## Version-Specific Features

### CapVer 39+ (Tailscale v1.30+)
- **TS2021 Protocol**: Full Noise protocol support over HTTPS
- **Feature**: Secure communication protocol upgrade
- **Implementation**: `hscontrol/handlers.go:29` - `NoiseCapabilityVersion = 39`

### CapVer 49+
- **EarlyNoise Support**: Protocol optimization for faster connection establishment
- **Implementation**: `hscontrol/noise.go:32` - `earlyNoiseCapabilityVersion = 49`

### CapVer 61+ (Tailscale v1.42+)
- **Minimum Version**: Required for Headscale compatibility
- **Implementation**: `hscontrol/noise.go:169` - `MinimumCapVersion = 61`

### CapVer 68+ (Tailscale v1.48+)
- **Streaming Requests**: All streaming requests treated as read-only
- **Long Polling**: Improved map request handling
- **Implementation**: `hscontrol/poll.go:218-220`

### CapVer 72+ (Tailscale v1.48+)
- **UPnP Re-enabled**: TS-2023-006 UPnP issue fixed, UPnP can be used again
- **Security**: Previous UPnP vulnerability patched
- **Implementation**: `hscontrol/mapper/tail.go:141-143`

### CapVer 74+ (Tailscale v1.50+)
- **NodeCapMap Support**: New capability map format
- **Capabilities**: 
  - File Sharing (`tailcfg.CapabilityFileSharing`)
  - Admin Access (`tailcfg.CapabilityAdmin`) 
  - SSH Access (`tailcfg.CapabilitySSH`)
  - Randomized Client Port (`tailcfg.NodeAttrRandomizeClientPort`)
- **Backward Compatibility**: Older clients use legacy `Capabilities` array
- **Implementation**: `hscontrol/mapper/tail.go:118-138`

### CapVer 81+ (Tailscale v1.56+)
- **Incremental Packet Filters**: `MapResponse.PacketFilters` for incremental updates
- **Performance**: Reduces bandwidth by sending only filter changes
- **Format**: Uses map format with "base" key for full updates
- **Implementation**: `hscontrol/mapper/mapper.go:589-595`

## Client Feature Matrix

| Tailscale Version | CapVer | TS2021 | EarlyNoise | Streaming | UPnP | NodeCapMap | Incremental Filters | SSH | Notes |
|-------------------|--------|--------|------------|-----------|------|------------|-------------------|-----|-------|
| v1.24             | ~39    | ✅     | ❌         | ❌        | ⚠️   | ❌         | ❌                | ⚠️  | SSH introduced |
| v1.30             | ~46    | ✅     | ❌         | ❌        | ⚠️   | ❌         | ❌                | ✅  | First fully supported |
| v1.42             | 61     | ✅     | ✅         | ❌        | ⚠️   | ❌         | ❌                | ✅  | Minimum supported |
| v1.48             | 68     | ✅     | ✅         | ✅        | ✅   | ❌         | ❌                | ✅  | UPnP fixed |
| v1.50             | 74     | ✅     | ✅         | ✅        | ✅   | ✅         | ❌                | ✅  | New capability format |
| v1.56             | 81+    | ✅     | ✅         | ✅        | ✅   | ✅         | ✅                | ✅  | Incremental filters |

**Legend:**
- ✅ Fully Supported
- ⚠️ Limited/Deprecated
- ❌ Not Available

## Node Capabilities

### File Sharing
- **CapVer**: 74+ (NodeCapMap), All versions (legacy)
- **Description**: Enables Tailscale file sharing between nodes
- **Config**: Always enabled by Headscale

### SSH Access  
- **CapVer**: 74+ (NodeCapMap), All versions (legacy)
- **Description**: Tailscale SSH functionality
- **Config**: Controlled by ACL SSH rules
- **Environment**: Requires `HEADSCALE_EXPERIMENTAL_FEATURE_SSH=1`

### Admin Access
- **CapVer**: 74+ (NodeCapMap), All versions (legacy)
- **Description**: Administrative capabilities for the node
- **Config**: Always enabled by Headscale

### Randomized Client Port
- **CapVer**: 74+ (NodeCapMap), All versions (legacy) 
- **Description**: Randomizes client port for improved NAT traversal
- **Config**: `RandomizeClientPort` in Headscale configuration

### UPnP Disable (Legacy)
- **CapVer**: <72 only
- **Description**: Disables UPnP due to security vulnerability TS-2023-006
- **Status**: Deprecated, UPnP re-enabled in CapVer 72+

## Protocol Features

### TS2021 (Noise Protocol)
- **Minimum CapVer**: 39
- **Description**: Secure communication protocol upgrade from legacy HTTP
- **Benefits**: Better security, performance, and reliability
- **Endpoint**: `/ts2021` for WebSocket upgrade

### EarlyNoise
- **Minimum CapVer**: 49  
- **Description**: Optimization that sends initial data during Noise handshake
- **Benefits**: Reduces round trips for faster connection establishment
- **Magic**: `\xff\xff\xffTS` header followed by JSON payload

### Streaming Requests
- **Minimum CapVer**: 68
- **Description**: All streaming map requests treated as read-only
- **Benefits**: Improved performance for long-polling connections

## Development Notes

### Testing Versions

Integration tests use these version categories:

**2021 Protocol Era (TS2021 Support)**:
- `head`, `unstable` - Latest development versions
- `1.70` to `1.46` - Modern clients with full feature support  
- `1.44`, `1.42` - Older supported clients with limited features

**2019 Protocol Era (Legacy)**:
- `1.28` to `1.18` - Legacy clients for compatibility testing
- **Note**: Most legacy versions use older protocol features

### Code References

Key files for version handling:
- `hscontrol/handlers.go` - Version parsing and TS2021 detection
- `hscontrol/mapper/tail.go` - Node capability assignment based on version
- `hscontrol/mapper/mapper.go` - Map response generation with version features
- `hscontrol/noise.go` - Noise protocol and EarlyNoise support
- `hscontrol/poll.go` - Long polling and streaming request handling
- `integration/scenario.go` - Version compatibility matrix for testing

## Migration Guide

### Upgrading Clients

When upgrading Tailscale clients:

1. **v1.24 → v1.30+**: Gains full TS2021 support, more stable connections
2. **v1.30 → v1.42+**: Minimum version for current Headscale, required upgrade  
3. **v1.42 → v1.48+**: Enables streaming requests, UPnP support restored
4. **v1.48 → v1.50+**: New NodeCapMap format, more efficient capability handling
5. **v1.50 → v1.56+**: Incremental packet filter updates, reduced bandwidth usage

### Server Configuration

No special configuration needed - Headscale automatically detects and adapts to client versions. Optional features:

```yaml
# Enable randomized client ports (CapVer 74+ clients)
randomize_client_port: true

# SSH requires environment variable
# HEADSCALE_EXPERIMENTAL_FEATURE_SSH=1
```

## Troubleshooting

### Common Issues

1. **"unsupported client connected"**: Client CapVer < 61, upgrade required
2. **TS2021 connection failures**: Check reverse proxy WebSocket support
3. **Missing capabilities**: Verify client version supports required CapVer
4. **SSH not working**: Ensure experimental flag is set and ACL rules configured

### Version Detection

Check client capability version in Headscale logs:
```
level=debug handler=/key cap_ver=74 msg="New noise client"
```

Or examine map requests for version information.