## ACL Functions Behavior

This document describes the behavior of each function related to Access Control Lists (ACLs) in the `hscontrol/grpcv1.go` file. These functions are part of the Headscale API and manage ACL policies, groups, hostnames, tags, and rules.

### Core Functions

*   **`copyACLConfig(dst, src string) error`**:
    *   Copies an ACL configuration file from the source path (`src`) to the destination path (`dst`).

Example:
```jsonc
// Request:
{
  "dst": "/path/to/destination.json",
  "src": "/path/to/source.json"
}

// Action: Copies the content of source.json to destination.json
```
    *   Opens both files, reads the source file's contents, and writes them to the destination file.
    *   Returns an error if any file operation fails.

*   **`getPendingACLConfig(h *Headscale) (*ACLPolicy, error)`**:
    *   Retrieves the pending ACL configuration. It stores the pending ACL config within the directory of the ACL config with the file name `.acl.json`.
    *   Determines the path to the pending ACL configuration file (e.g., `.acl.json` in the same directory as the active ACL policy file).
    *   If the pending ACL configuration file does not exist, it copies the current ACL policy file to the pending location.
    *   Loads and returns the ACL policy from the pending ACL configuration file.

Example:
```jsonc
// No request body

// Action: Retrieves the ACL policy from /.acl.json or copies the current ACL policy to /.acl.json if it doesn't exist.
```

*   **`updatePendingACLConfig(h *Headscale, policy *ACLPolicy) error`**:
    *   Updates the pending ACL configuration with the provided `policy`.
    *   Marshals the `policy` to JSON format and writes it to the pending ACL configuration file.

Example:
```jsonc
// Assuming an ACLPolicy object is passed.

// Action: Updates the .acl.json file with the new ACLPolicy object.
```

*   **`3(h *Headscale) error`**:
    *   Reloads the pending ACL configuration by replacing the current ACL policy file with the pending ACL configuration file.
    *   Renames the pending ACL configuration file to the active ACL policy file, effectively applying the changes.
    *   Loads the ACL policy from the updated ACL policy file.

*   **`discardPendingACLConfig(h *Headscale) error`**:
    *   Discards the pending ACL configuration by removing the pending ACL configuration file.

Example:
```jsonc
// No request body

// Action: Deletes the .acl.json file.
```

### ACL Group Functions

*   **`ACLCreateGroup(ctx context.Context, request *v1.ACLGroupRequest) (*v1.ACLGroupResponse, error)`**:
    *   Creates a new ACL group with the name specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Checks if the group already exists; returns an error if it does.
    *   Creates a new group entry in the ACL policy.
    *   Updates the pending ACL configuration with the new group.

Example:
```jsonc
// Request:
{
  "group_name": "testers"
}

// Resulting change to ACL configuration:
{
  "groups": {
    "group:boss": ["boss"],
    "group:dev": ["dev1", "dev2"],
    "group:admin": ["admin1"],
    "group:intern": ["intern1"],
    "group:testers": [] // New group added
  },
  // ... (rest of the config)
}
```

*   **`ACLGroupAddUser(ctx context.Context, request *v1.ACLGroupUserRequest) (*v1.ACLGroupUserResponse, error)`**:
    *   Adds a user to an ACL group specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Checks if the group exists; returns an error if it does not.
    *   Adds the user to the specified group in the ACL policy.
    *   Returns an error if the user is already present in the group.
    *   Updates the pending ACL configuration with the updated group.

Example:
```jsonc
// Request:
{
  "group_name": "dev",
  "username": "newdev"
}

// Resulting change to ACL configuration:
{
  "groups": {
    "group:boss": ["boss"],
    "group:dev": ["dev1", "dev2", "newdev"], // User added to group
    "group:admin": ["admin1"],
    "group:intern": ["intern1"]
  },
  // ... (rest of the config)
}
```

*   **`ACLGroupRemoveUser(ctx context.Context, request *v1.ACLGroupUserRequest) (*v1.ACLGroupUserResponse, error)`**:
    *   Removes a user from an ACL group specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Checks if the group exists; returns an error if it does not.
    *   Removes the user from the specified group in the ACL policy.
    *   Returns an error if the user is not present in the group.
    *   Updates the pending ACL configuration with the updated group.

Example:
```jsonc
// Request:
{
  "group_name": "dev",
  "username": "dev1"
}

// Resulting change to ACL configuration:
{
  "groups": {
    "group:boss": ["boss"],
    "group:dev": ["dev2"], // User removed from group
    "group:admin": ["admin1"],
    "group:intern": ["intern1"]
  },
  // ... (rest of the config)
}
```

*   **`ACLRemoveGroup(ctx context.Context, request *v1.ACLGroupRequest) (*v1.ACLGroupResponse, error)`**:
    *   Removes an ACL group specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Checks if the group exists; returns an error if it does not.
    *   Deletes the group entry from the ACL policy.
    *   Updates the pending ACL configuration, removing the group.

Example:
```jsonc
// Request:
{
  "group_name": "intern"
}

// Resulting change to ACL configuration:
{
  "groups": {
    "group:boss": ["boss"],
    "group:dev": ["dev1", "dev2"],
    "group:admin": ["admin1"]
  },
  // ... (rest of the config)
}
```

### ACL Hostname Functions

*   **`ACLBindHostname(ctx context.Context, request *v1.ACLHostnameRequest) (*v1.ACLHostnameResponse, error)`**:
    *   Binds a hostname to a subnet, as specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Checks if the hostname is of the "subnet" type; returns an error if not.
    *   Checks if the hostname already exists; returns an error if it does.
    *   Adds the hostname and subnet binding to the ACL policy.
    *   Updates the pending ACL configuration.

Example:
```jsonc
// Request:
{
  "hostname": "newserver.internal",
  "type": "subnet",
  "address": "10.20.20.10/32"
}

// Resulting change to ACL configuration:
{
  "hosts": {
    "postgresql.internal": "10.20.0.2/32",
    "webservers.internal": "10.20.10.1/29",
    "subnet:newserver.internal": "10.20.20.10/32" // Hostname added
  },
  // ... (rest of the config)
}
```

*   **`ACLUpdateHostname(ctx context.Context, request *v1.ACLHostnameRequest) (*v1.ACLHostnameResponse, error)`**:
    *   Updates a hostname's subnet binding, as specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Checks if the hostname is of the "subnet" type; returns an error if not.
    *   Checks if the hostname exists; returns an error if it does not.
    *   Updates the hostname's subnet binding in the ACL policy.
    *   Updates the pending ACL configuration.

Example:
```jsonc
// Request:
{
  "hostname": "webservers.internal",
  "type": "subnet",
  "address": "10.20.10.1/24"
}

// Resulting change to ACL configuration:
{
  "hosts": {
    "postgresql.internal": "10.20.0.2/32",
    "webservers.internal": "10.20.10.1/24" // Hostname updated
  },
  // ... (rest of the config)
}
```

*   **`ACLRemoveHostname(ctx context.Context, request *v1.ACLHostnameRequest) (*v1.ACLHostnameResponse, error)`**:
    *   Removes a hostname's subnet binding.
    *   Retrieves the pending ACL configuration.
    *   Checks if the hostname exists; returns an error if it does not.
    *   Removes the hostname from the ACL policy.
    *   Updates the pending ACL configuration.

Example:
```jsonc
// Request:
{
  "hostname": "webservers.internal",
  "type": "subnet"
}

// Resulting change to ACL configuration:
{
  "hosts": {
    "postgresql.internal": "10.20.0.2/32" // Hostname removed
  },
  // ... (rest of the config)
}
```

### ACL Tag Functions

*   **`ACLCreateTag(ctx context.Context, request *v1.ACLTagRequest) (*v1.ACLTagResponse, error)`**:
    *   Creates a new ACL tag, as specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Checks if the tag already exists; returns an error if it does.
    *   Adds the tag to the ACL policy.
    *   Updates the pending ACL configuration.

Example:
```jsonc
// Request:
{
  "tag": "monitoring"
}

// Resulting change to ACL configuration:
{
  "tagOwners": {
    "tag:prod-databases": ["group:admin"],
    "tag:prod-app-servers": ["group:admin"],
    "tag:internal": ["group:boss"],
    "tag:dev-databases": ["group:admin", "group:dev"],
    "tag:dev-app-servers": ["group:admin", "group:dev"],
    "tag:monitoring": [] // Tag added
  },
  // ... (rest of the config)
}
```

*   **`ACLRemoveTag(ctx context.Context, request *v1.ACLTagRequest) (*v1.ACLTagResponse, error)`**:
    *   Removes an ACL tag, as specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Checks if the tag exists; returns an error if it does not.
    *   Removes the tag from the ACL policy.
    *   Updates the pending ACL configuration.

Example:
```jsonc
// Request:
{
  "tag": "internal"
}

// Resulting change to ACL configuration:
{
  "tagOwners": {
    "tag:prod-databases": ["group:admin"],
    "tag:prod-app-servers": ["group:admin"],
    "tag:dev-databases": ["group:admin", "group:dev"],
    "tag:dev-app-servers": ["group:admin", "group:dev"] // Tag removed
  },
  // ... (rest of the config)
}
```

### ACL Rule Functions

*   **`getACLRuleIdx(rules []ACL, target ACL) int`**:
    *   Helper function to find the index of a given ACL rule within a slice of ACL rules.
    *   Returns the index if the rule is found, otherwise returns -1.

*   **`getACLRuleIdxBySrc(rules []ACL, target ACL) int`**:
    *   Helper function to find the index of an ACL rule by source.
    *   Returns the index if found, otherwise returns -1.

*   **`getACLRuleIdxsByDst(rules []ACL, target ACL) []int`**:
    *   Helper function to get indices of ACL rules by destination.
    *   Returns a slice of indices.

*   **`getACLDstIdx(ruleDsts []string, targetDst string) int`**:
    *   Helper function to get the index of destination.
    *   Returns index if found, otherwise returns -1.

*   **`ACLCreateRule(ctx context.Context, request *v1.ACLRuleRequest) (*v1.ACLRuleResponse, error)`**:
    *   Creates a new ACL rule, as specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Checks if the rule already exists; returns an error if it does.
    *   Adds the rule to the ACL policy.
    *   Updates the pending ACL configuration.

Example:
```jsonc
// Request:
{
  "src": ["group:testers"],
  "dst": ["tag:monitoring:*"]
}

// Resulting change to ACL configuration:
{
  "acls": [
    // boss have access to all servers
    {
      "action": "accept",
      "src": ["group:boss"],
      "dst": [
        "tag:prod-databases:*",
        "tag:prod-app-servers:*",
        "tag:internal:*",
        "tag:dev-databases:*",
        "tag:dev-app-servers:*",
      ],
    },
    // ... (existing rules)
    {
      "action": "accept",
      "src": ["group:testers"], // New rule added
      "dst": ["tag:monitoring:*"]
    }
  ]
  // ... (rest of the config)
}
```

*   **`ACLRemoveRule(ctx context.Context, request *v1.ACLRuleRequest) (*v1.ACLRuleResponse, error)`**:
    *   Removes an ACL rule, as specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Checks if the rule exists; returns an error if it does not.
    *   Removes the rule from the ACL policy.
    *   Updates the pending ACL configuration.

Example:
```jsonc
// Request:
{
  "src": ["group:boss"],
  "dst": [
    "tag:prod-databases:*",
    "tag:prod-app-servers:*",
    "tag:internal:*",
    "tag:dev-databases:*",
    "tag:dev-app-servers:*"
  ]
}

// Resulting change to ACL configuration:
{
  "acls": [
    // admin have only access to administrative ports of the servers, in tcp/22
    {
      "action": "accept",
      "src": ["group:admin"],
      "proto": "tcp",
      "dst": [
        "tag:prod-databases:22",
        "tag:prod-app-servers:22",
        "tag:internal:22",
        "tag:dev-databases:22",
        "tag:dev-app-servers:22",
      ],
    },
    // ... (rest of the config)
  ]
  // ... (rest of the config)
}
```

*   **`ACLForceRemoveRule(ctx context.Context, request *v1.ACLRuleRequest) (*v1.ACLRuleResponse, error)`**:
    *   Force removes an ACL rule by source, as specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Removes the rule from the ACL policy.
    *   Updates the pending ACL configuration.

*   **`ACLRuleInclude(ctx context.Context, request *v1.ACLRuleRequest) (*v1.ACLRuleResponse, error)`**:
    *   Includes destinations to an ACL rule, as specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Updates the pending ACL configuration.

Example:

```jsonc
// Request:
{
  "src": ["group:dev"],
  "dst": ["tag:monitoring:*"]
}

// Resulting change to ACL configuration:
{
  "acls": [
        {
            "action": "accept",
            "src": [
                "group:dev"
            ],
            "dst": [
                "tag:dev-databases:*",
                "tag:dev-app-servers:*",
                "tag:prod-app-servers:80,443",
                "tag:monitoring:*" //New destination added
            ]
        },
   // ... (rest of the config)
  ]
  // ... (rest of the config)
}
```

*   **`ACLRuleExclude(ctx context.Context, request *v1.ACLRuleRequest) (*v1.ACLRuleResponse, error)`**:
    *   Excludes destinations from an ACL rule, as specified in the `request`.
    *   Retrieves the pending ACL configuration.
    *   Updates the pending ACL configuration.

### ACL Control Function

*   **`ACLCtrl(ctx context.Context, request *v1.ACLCtrlRequest) (*v1.ACLCtrlResponse, error)`**:
    *   Performs control actions on ACLs, such as reload and discard, based on the `action` specified in the `request`.
    *   If the action is "reload", it calls `reloadPendingACLConfig` to reload the ACL policy.
    *   If the action is "discard", it calls `discardPendingACLConfig` to discard the pending ACL policy.
    *   Returns an error if the action is not supported.
