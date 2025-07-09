# ACL Feature Porting - To-Do List

This document outlines the steps required to port the ACL control functions from `headscale-0.22.3-dev` to `headscale-0.23-dev`, adapting to the new `Policy` paradigm and addressing the `DBmode` limitation for `SetPolicy`.

## Phase 1: Understanding and Preparation

- [x] **Review `ACLPolicy` Structure:** Fully understand the `ACLPolicy` struct defined in `vpncloud-service-controlplane/headscale-0.23-dev/hscontrol/policy/acls_types.go`, including `Groups`, `Hosts`, `TagOwners`, `ACLs`, `Tests`, `AutoApprovers`, and `SSHs`. This is crucial for modifying the policy programmatically.
- [x] **Analyze Old ACL Functions:** For each ACL control function in `vpncloud-service-controlplane/headscale-0.22.3-dev/hscontrol/grpcv1.go` (e.g., `ACLCreateGroup`, `ACLGroupAddUser`, `ACLRemoveRule`, etc.), document:
    - Its purpose and expected input/output.
    - How it modifies the ACL configuration in the old system.
    - Which parts of the `ACLPolicy` struct it will affect in the new system.
- [ ] **Decision on `SetPolicy` `PolicyModeFile` Support:**
    - [ ] Confirm if `PolicyModeFile` support for `SetPolicy` is a hard requirement.
    - [ ] If yes, plan the modifications to `SetPolicy` in `vpncloud-service-controlplane/headscale-0.23-dev/hscontrol/grpcv1.go` to allow writing to file. This will involve:
        -   Adding a `case types.PolicyModeFile:` block.
        -   Writing the `request.GetPolicy()` string to `api.h.cfg.Policy.Path`.
        -   **Consider potential file corruption:** If directly writing to file, evaluate if a mechanism similar to `copyACLConfig` (e.g., writing to a temporary file and then atomically renaming) is needed for basic protection against crashes.
        -   Reloading `api.h.ACLPolicy`.
        -   Notifying nodes of the update.
    - [ ] If no, document that only `DBmode` will be supported for API-driven ACL modifications and consider implications for existing file-based configurations.
    - [ ] **Evaluate necessity of old helper functions:** The old helper functions like `copyACLConfig`, `getPendingACLConfig`, `updatePendingACLConfig`, `reloadPendingACLConfig`, and `discardPendingACLConfig` are likely **not needed** in the new `Policy` paradigm, especially when using `DBmode`. If `PolicyModeFile` support is added to `SetPolicy`, evaluate if any *simplified* form of these helpers is required for transactional updates or basic file protection.


## Phase 2: Implementation of Ported ACL Functions

For each old ACL function:

- [ ] **Implement `ACLCreateGroup`:**
    - [ ] Call `api.h.GetPolicy()` to retrieve the current policy.
    - [ ] Parse the `Policy.Data` (HuJSON string) into a `policy.ACLPolicy` struct.
    - [ ] Add the new group to `policy.ACLPolicy.Groups`.
    - [ ] Marshal the modified `policy.ACLPolicy` struct back into a HuJSON string.
    - [ ] Call `api.h.SetPolicy()` with the updated HuJSON string.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLGroupAddUser`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Add the user to the specified group in `policy.ACLPolicy.Groups`.
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLGroupRemoveUser`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Remove the user from the specified group in `policy.ACLPolicy.Groups`.
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLRemoveGroup`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Remove the specified group from `policy.ACLPolicy.Groups`.
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLBindHostname`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Update `policy.ACLPolicy.Hosts` with the new hostname binding.
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLUpdateHostname`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Modify the existing hostname in `policy.ACLPolicy.Hosts`.
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLRemoveHostname`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Remove the hostname from `policy.ACLPolicy.Hosts`.
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLCreateTag`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Add the new tag to `policy.ACLPolicy.TagOwners`.
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLRemoveTag`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Remove the tag from `policy.ACLPolicy.TagOwners`.
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLCreateRule`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Add the new ACL rule to `policy.ACLPolicy.ACLs`.
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLRemoveRule`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Remove the specified ACL rule from `policy.ACLPolicy.ACLs`.
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLForceRemoveRule`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Forcefully remove the specified ACL rule from `policy.ACLPolicy.ACLs` (handle potential dependencies or references).
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLRuleInclude`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Add the include pattern to the relevant ACL rule in `policy.ACLPolicy.ACLs`.
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

- [ ] **Implement `ACLRuleExclude`:**
    - [ ] Retrieve and parse the current policy.
    - [ ] Add the exclude pattern to the relevant ACL rule in `policy.ACLPolicy.ACLs`.
    - [ ] Marshal and set the updated policy.
    - [ ] Add necessary error handling and logging.

## Phase 3: Testing and Validation

- [ ] **Unit Tests:** Create or adapt unit tests for each ported ACL function to ensure they behave as expected and correctly modify the `ACLPolicy`.
- [ ] **Integration Tests:** Set up integration tests to verify that the ported ACL functions work correctly within the Headscale environment, especially concerning policy application and node updates.
- [ ] **Backward Compatibility:** If `PolicyModeFile` is supported, ensure that existing file-based ACL configurations can still be read and modified via the new API.
- [ ] **Error Handling:** Verify that all error conditions are handled gracefully and informative error messages are returned.

## Phase 4: Clean-up and Documentation

- [ ] **Remove Old Code:** Once the new functions are working and tested, remove the old ACL control functions from `headscale-0.22.3-dev/hscontrol/grpcv1.go` (or wherever they reside).
- [ ] **Update Documentation:** Document the new ACL API endpoints and any changes in usage for users.
- [ ] **Migration Guide:** If migrating from file mode to DB mode is required, provide a clear migration guide for users.
