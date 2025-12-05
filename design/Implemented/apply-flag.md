# Apply flag for install command

## Abstract
Add an `--apply` flag to the install command that enables applying existing resources rather than creating them. This can be useful as part of the upgrade process for existing installations.

## Background
The current Velero install command creates resources but doesn't provide a direct way to apply updates to an existing installation.
Users attempting to run the install command on an existing installation receive "already exists" messages.
Upgrade steps for existing installs typically involve a three (or more) step process to apply updated CRDs (using `--dry-run` and piping to `kubectl apply`) and then updating/setting images on the Velero deployment and node-agent.

## Goals
- Provide a simple flag to enable applying resources on an existing Velero installation.
- Use server-side apply to update existing resources rather than attempting to create them.
- Maintain consistency with the regular install flow.

## Non Goals
- Implement special logic for specific version-to-version upgrades (i.e. resource deletion, etc).
- Add complex upgrade validation or pre/post-upgrade hooks.
- Provide rollback capabilities.

## High-Level Design
The `--apply` flag will be added to the Velero install command.
When this flag is set, the installation process will use server-side apply to update existing resources instead of using create on new resources.
This flag can be used as _part_ of the upgrade process, but will not always fully handle an upgrade.

## Detailed Design
The implementation adds a new boolean flag `--apply` to the install command.
This flag will be passed through to the underlying install functions where the resource creation logic resides.

When the flag is set to true:
- The `createOrApplyResource` function will use server-side apply with field manager "velero-cli" and `force=true` to update resources.
- Resources will be applied in the same order as they would be created during installation.
- Custom Resource Definitions will still be processed first, and the system will wait for them to be established before continuing.

The server-side apply approach with `force=true` ensures that resources are updated even if there are conflicts with the last applied state.
This provides a best-effort mechanism to apply resources that follows the same flow as installation but updates resources instead of creating them.

No special handling is added for specific versions or resource structures, making this a general-purpose mechanism for applying resources.

## Alternatives Considered
1. Creating a separate `upgrade` command that would duplicate much of the install command logic.
   - Rejected due to code duplication and maintenance overhead.

2. Implementing version-specific upgrade logic to handle breaking changes between versions.
   - Rejected as overly complex and difficult to maintain across multiple version paths.
   - This could be considered again in the future, but is not in the scope of the current design.

3. Adding automatic detection of existing resources and switching to apply mode.
   - Rejected as it could lead to unexpected behavior and confusion if users unintentionally apply changes to existing resources.

## Security Considerations
The apply flag maintains the same security profile as the install command.
No additional permissions are required beyond what is needed for resource creation.
The use of `force=true` with server-side apply could potentially override manual changes made to resources, but this is a necessary trade-off to ensure apply is successful.

## Compatibility
This enhancement is compatible with all existing Velero installations as it is a new opt-in flag.
It does not change any resource formats or API contracts.
The apply process is best-effort and does not guarantee compatibility between arbitrary versions of Velero.
Users should still consult release notes for any breaking changes that may require manual intervention.
This flag could be adopted by the helm chart, specifically for CRD updates, to simplify the CRD update job.

## Implementation
The implementation involves:
1. Adding support for `Apply` to the existing Kubernetes client code.
1. Adding the `--apply` flag to the install command options.
1. Changing `createResource` to `createOrApplyResource` and updating it to use server-side apply when the `apply` boolean is set.

The implementation is straightforward and follows existing code patterns.
No migration of state or special handling of specific resources is required.
