# Upgrade flag for install command

## Abstract
Add an `--upgrade` flag to the install command that enables applying existing resources rather than creating them, allowing for in-place Velero upgrades.

## Background
The current Velero install command creates resources but doesn't provide a straightforward way to upgrade an existing installation.
Users attempting to run the install command on an existing installation receive "already exists" messages.
A dedicated upgrade mechanism would improve the user experience for updating Velero components.

## Goals
- Provide a simple flag to enable upgrading an existing Velero installation.
- Use server-side apply to update existing resources rather than attempting to create them.
- Maintain consistency with the regular install flow.

## Non Goals
- Implement special logic for specific version-to-version upgrades.
- Add complex upgrade validation or pre/post-upgrade hooks.
- Provide rollback capabilities.

## High-Level Design
The `--upgrade` flag will be added to the Velero install command.
When this flag is set, the installation process will use server-side apply to update existing resources instead of using create on new resources.

## Detailed Design
The implementation adds a new boolean flag `--upgrade` to the install command.
This flag will be passed through to the underlying install functions where the resource creation logic resides.

When the flag is set to true:
- The `createOrApplyResource` function will use server-side apply with field manager "velero-cli" and `force=true` to update resources.
- Resources will be applied in the same order as they would be created during installation.
- Custom Resource Definitions will still be processed first, and the system will wait for them to be established before continuing.

The server-side apply approach with `force=true` ensures that resources are updated even if there are conflicts with the last applied state.
This provides a best-effort upgrade mechanism that follows the same flow as installation but updates resources instead of creating them.

No special handling is added for specific versions or resource structures, making this a general-purpose upgrade mechanism.

## Alternatives Considered
1. Creating a separate `upgrade` command that would duplicate much of the install command logic.
   - Rejected due to code duplication and maintenance overhead.

2. Implementing version-specific upgrade logic to handle breaking changes between versions.
   - Rejected as overly complex and difficult to maintain across multiple version paths.

3. Adding automatic detection of existing resources and switching to upgrade mode.
   - Rejected as it could lead to unexpected behavior and confusion if users unintentionally upgrade.

## Security Considerations
The upgrade flag maintains the same security profile as the install command.
No additional permissions are required beyond what is needed for resource creation.
The use of `force=true` with server-side apply could potentially override manual changes made to resources, but this is a necessary trade-off for the upgrade process.

## Compatibility
This enhancement is compatible with all existing Velero installations as it is a new opt-in flag.
It does not change any resource formats or API contracts.
The upgrade process is best-effort and does not guarantee compatibility between arbitrary versions of Velero.
Users should still consult release notes for any breaking changes that may require manual intervention.
This flag could be adopted by the helm chart, specifically for CRD upgrades, to simplify the CRD upgrade job.

## Implementation
The implementation involves:
1. Adding support for `Apply` to the existing Kubernetes client code.
1. Adding the `--upgrade` flag to the install command options.
1. Changing `createResource` to `createOrApplyResource` and updating it to use server-side apply when the `upgrade` boolean is set.

The implementation is straightforward and follows existing code patterns.
No migration of state or special handling of specific resources is required.
