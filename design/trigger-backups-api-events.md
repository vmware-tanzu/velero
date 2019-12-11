# Trigger backups based on Kubernetes events 

Triggering backups with Velero based on Kubernetes api events (Terminating, deleting, CrashLoopBackOff, etc..).

## Goals

- Rather than performing backups manually or by scheduling them, backups are created based on events.
- Recovering the components of the cluster when they were deleted accidently. 
- Blocking the delete process of a component in the cluster if it was a human intervention and trigger a backup.

## Non Goals

- N/A

## Background

## High-Level Design

A custom controller that list and watch all the events of Kubenretes and based on that it trigger backups.

## Detailed Design

A custom controller where it have its own refelctor to list and watch a specific components using the kubernetes watch api. 
It will add the object mentored with its status/current event to a queue.
Then pop up the object based on its status (criticity and priority).
Block the delete process if it was performed from a human being and trigger a backup for that component.
Trying to catch the errors happening on the components so it predicts when they will be down and trigger backups.

## Alternatives Considered

Apply the same design in a separate plugin as a new kind.

## Security Considerations

N/A
