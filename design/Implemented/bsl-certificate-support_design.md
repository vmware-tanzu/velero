# Design for BSL Certificate Support Enhancement

## Abstract

This design document describes the enhancement of BackupStorageLocation (BSL) certificate management in Velero, introducing a Secret-based certificate reference mechanism (`caCertRef`) alongside the existing inline certificate field (`caCert`). This enhancement provides a more secure, Kubernetes-native approach to certificate management while enabling future CLI improvements for automatic certificate discovery.

## Background

Currently, Velero supports TLS certificate verification for object storage providers through an inline `caCert` field in the BSL specification. While functional, this approach has several limitations:

- **Security**: Certificates are stored directly in the BSL YAML, potentially exposing sensitive data
- **Management**: Certificate rotation requires updating the BSL resource itself
- **CLI Usability**: Users must manually specify certificates when using CLI commands
- **Size Limitations**: Large certificate bundles can make BSL resources unwieldy

Issue #9097 and PR #8557 highlight the need for improved certificate management that addresses these concerns while maintaining backward compatibility.

## Goals

- Provide a secure, Secret-based certificate storage mechanism
- Maintain full backward compatibility with existing BSL configurations
- Enable future CLI enhancements for automatic certificate discovery
- Simplify certificate rotation and management
- Provide clear migration path for existing users

## Non-Goals

- Removing support for inline certificates immediately
- Changing the behavior of existing BSL configurations
- Implementing client-side certificate validation
- Supporting certificates from ConfigMaps or other resource types

## High-Level Design

### API Changes

#### New Field: CACertRef

```go
type ObjectStorageLocation struct {
    // Existing field (now deprecated)
    // +optional
    // +kubebuilder:deprecatedversion:warning="caCert is deprecated, use caCertRef instead"
    CACert []byte `json:"caCert,omitempty"`

    // New field for Secret reference
    // +optional
    CACertRef *corev1api.SecretKeySelector `json:"caCertRef,omitempty"`
}
```

The `SecretKeySelector` follows standard Kubernetes patterns:
```go
type SecretKeySelector struct {
    // Name of the Secret
    Name string `json:"name"`
    // Key within the Secret
    Key string `json:"key"`
}
```

### Certificate Resolution Logic

The system follows a priority-based resolution:

1. If `caCertRef` is specified, retrieve certificate from the referenced Secret
2. If `caCert` is specified (and `caCertRef` is not), use the inline certificate
3. If neither is specified, no custom CA certificate is used

### Validation

BSL validation ensures mutual exclusivity:
```go
func (bsl *BackupStorageLocation) Validate() error {
    if bsl.Spec.ObjectStorage != nil &&
        bsl.Spec.ObjectStorage.CACert != nil &&
        bsl.Spec.ObjectStorage.CACertRef != nil {
        return errors.New("cannot specify both caCert and caCertRef in objectStorage")
    }
    return nil
}
```

## Detailed Design

### BSL Controller Changes

The BSL controller incorporates validation during reconciliation:

```go
func (r *backupStorageLocationReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
    // ... existing code ...
    
    // Validate BSL configuration
    if err := location.Validate(); err != nil {
        r.logger.WithError(err).Error("BSL validation failed")
        return ctrl.Result{}, err
    }
    
    // ... continue reconciliation ...
}
```

### Repository Provider Integration

All repository providers implement consistent certificate handling:

```go
func configureCACert(bsl *velerov1api.BackupStorageLocation, credGetter *credentials.CredentialGetter) ([]byte, error) {
    if bsl.Spec.ObjectStorage == nil {
        return nil, nil
    }

    // Prefer caCertRef (new method)
    if bsl.Spec.ObjectStorage.CACertRef != nil {
        certString, err := credGetter.FromSecret.Get(bsl.Spec.ObjectStorage.CACertRef)
        if err != nil {
            return nil, errors.Wrap(err, "error getting CA certificate from secret")
        }
        return []byte(certString), nil
    }

    // Fall back to caCert (deprecated)
    if bsl.Spec.ObjectStorage.CACert != nil {
        return bsl.Spec.ObjectStorage.CACert, nil
    }

    return nil, nil
}
```

### CLI Certificate Discovery Integration

#### Background: PR #8557 Implementation
PR #8557 ("CLI automatically discovers and uses cacert from BSL") was merged in August 2025, introducing automatic CA certificate discovery from BackupStorageLocation for Velero CLI download operations. This eliminated the need for users to manually specify the `--cacert` flag when performing operations like `backup describe`, `backup download`, `backup logs`, and `restore logs`.

#### Current Implementation (Post PR #8557)
The CLI now automatically discovers certificates from BSL through the `pkg/cmd/util/cacert/bsl_cacert.go` module:

```go
// Current implementation only supports inline caCert
func GetCACertFromBSL(ctx context.Context, client kbclient.Client, namespace, bslName string) (string, error) {
    // ... fetch BSL ...
    if bsl.Spec.ObjectStorage != nil && len(bsl.Spec.ObjectStorage.CACert) > 0 {
        return string(bsl.Spec.ObjectStorage.CACert), nil
    }
    return "", nil
}
```

#### Enhancement with caCertRef Support
This design extends the existing CLI certificate discovery to support the new `caCertRef` field:

```go
// Enhanced implementation supporting both caCert and caCertRef
func GetCACertFromBSL(ctx context.Context, client kbclient.Client, namespace, bslName string) (string, error) {
    // ... fetch BSL ...

    // Prefer caCertRef over inline caCert
    if bsl.Spec.ObjectStorage.CACertRef != nil {
        secret := &corev1api.Secret{}
        key := types.NamespacedName{
            Name:      bsl.Spec.ObjectStorage.CACertRef.Name,
            Namespace: namespace,
        }
        if err := client.Get(ctx, key, secret); err != nil {
            return "", errors.Wrap(err, "error getting certificate secret")
        }

        certData, ok := secret.Data[bsl.Spec.ObjectStorage.CACertRef.Key]
        if !ok {
            return "", errors.Errorf("key %s not found in secret",
                bsl.Spec.ObjectStorage.CACertRef.Key)
        }
        return string(certData), nil
    }

    // Fall back to inline caCert (deprecated)
    if bsl.Spec.ObjectStorage.CACert != nil {
        return string(bsl.Spec.ObjectStorage.CACert), nil
    }

    return "", nil
}
```

#### Certificate Resolution Priority

The CLI follows this priority order for certificate resolution:

1. **`--cacert` flag** - Manual override, highest priority
2. **`caCertRef`** - Secret-based certificate (recommended)
3. **`caCert`** - Inline certificate (deprecated)
4. **System certificate pool** - Default fallback

#### User Experience Improvements

With both PR #8557 and this enhancement:

```bash
# Automatic discovery - works with both caCert and caCertRef
velero backup describe my-backup
velero backup download my-backup
velero backup logs my-backup
velero restore logs my-restore

# Manual override still available
velero backup describe my-backup --cacert /custom/ca.crt

# Debug output shows certificate source
velero backup download my-backup --log-level=debug
# [DEBUG] Resolved CA certificate from BSL 'default' Secret 'storage-ca-cert' key 'ca-bundle.crt'
```

#### RBAC Considerations for CLI

CLI users need read access to Secrets when using `caCertRef`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: velero-cli-user
  namespace: velero
rules:
- apiGroups: ["velero.io"]
  resources: ["backups", "restores", "backupstoragelocations"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]
  # Limited to secrets referenced by BSLs
```

### Migration Strategy

#### Phase 1: Introduction (Current)
- Add `caCertRef` field
- Mark `caCert` as deprecated
- Both fields supported, mutual exclusivity enforced

#### Phase 2: Migration Period
- Documentation and tools to help users migrate
- Warning messages for `caCert` usage
- CLI enhancements to leverage `caCertRef`

#### Phase 3: Future Removal
- Remove `caCert` field in major version update
- Provide migration tool for automatic conversion

## User Experience

### Creating a BSL with Certificate Reference

1. Create a Secret containing the CA certificate:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: storage-ca-cert
  namespace: velero
type: Opaque
data:
  ca-bundle.crt: <base64-encoded-certificate>
```

2. Reference the Secret in BSL:
```yaml
apiVersion: velero.io/v1
kind: BackupStorageLocation
metadata:
  name: default
  namespace: velero
spec:
  provider: aws
  objectStorage:
    bucket: my-bucket
    caCertRef:
      name: storage-ca-cert
      key: ca-bundle.crt
```

### Certificate Rotation

With Secret-based certificates:
```bash
# Update the Secret with new certificate
kubectl create secret generic storage-ca-cert \
  --from-file=ca-bundle.crt=new-ca.crt \
  --dry-run=client -o yaml | kubectl apply -f -

# No BSL update required - changes take effect on next use
```

### CLI Usage Examples

#### Immediate Benefits
- No change required for existing workflows
- Certificate validation errors include helpful context

#### Future CLI Enhancements
```bash
# Automatic certificate discovery
velero backup download my-backup

# Manual override still available
velero backup download my-backup --cacert /custom/ca.crt

# Debug certificate resolution
velero backup download my-backup --log-level=debug
# [DEBUG] Resolved CA certificate from BSL 'default' Secret 'storage-ca-cert'
```

## Security Considerations

### Advantages of Secret-based Storage

1. **Encryption at Rest**: Secrets are encrypted in etcd
2. **RBAC Control**: Fine-grained access control via Kubernetes RBAC
3. **Audit Trail**: Secret access is auditable
4. **Separation of Concerns**: Certificates separate from configuration

### Required Permissions

The Velero server requires additional RBAC permissions:
```yaml
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]
  # Scoped to secrets referenced by BSLs
```

## Compatibility

### Backward Compatibility

- Existing BSLs with `caCert` continue to function unchanged
- No breaking changes to API
- Gradual migration path

### Forward Compatibility

- Design allows for future enhancements:
  - Multiple certificate support
  - Certificate chain validation
  - Automatic certificate discovery from cloud providers

## Implementation Phases

### Phase 1: Core Implementation ✓ (Current PR)
- API changes with new `caCertRef` field
- Controller validation
- Repository provider updates
- Basic testing

### Phase 2: CLI Enhancement (Future)
- Automatic certificate discovery in CLI
- Enhanced error messages
- Debug logging for certificate resolution

### Phase 3: Migration Tools (Future)
- Automated migration scripts
- Validation tools
- Documentation updates

## Testing

### Unit Tests
- BSL validation logic
- Certificate resolution in providers
- Controller behavior

### Integration Tests
- End-to-end backup/restore with `caCertRef`
- Certificate rotation scenarios
- Migration from `caCert` to `caCertRef`

### Manual Testing Scenarios
1. Create BSL with `caCertRef`
2. Perform backup/restore operations
3. Rotate certificate in Secret
4. Verify continued operation

## Documentation

### User Documentation
- Migration guide from `caCert` to `caCertRef`
- Examples for common cloud providers
- Troubleshooting guide

### API Documentation
- Updated API reference
- Deprecation notices
- Field descriptions

## Alternatives Considered

### ConfigMap-based Storage
- Pros: Similar to Secrets, simpler API
- Cons: Not designed for sensitive data, no encryption at rest
- Decision: Secrets are the Kubernetes-standard for sensitive data

### External Certificate Management
- Pros: Integration with cert-manager, etc.
- Cons: Additional complexity, dependencies
- Decision: Keep it simple, allow users to manage certificates as needed

### Immediate Removal of Inline Certificates
- Pros: Cleaner API, forces best practices
- Cons: Breaking change, migration burden
- Decision: Gradual deprecation respects existing users

## Conclusion

This design provides a secure, Kubernetes-native approach to certificate management in Velero while maintaining backward compatibility. It establishes the foundation for enhanced CLI functionality and improved user experience, addressing the concerns raised in issue #9097 and enabling the features proposed in PR #8557.

The phased approach ensures smooth migration for existing users while delivering immediate security benefits for new deployments.