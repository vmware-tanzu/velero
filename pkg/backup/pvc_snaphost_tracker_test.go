package backup

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/vmware-tanzu/velero/pkg/builder"
)

func TestOptoutVolume(t *testing.T) {
	pod := builder.ForPod("ns-1", "pod-1").Volumes(
		builder.ForVolume("pod-vol-1").PersistentVolumeClaimSource("pvc-1").Result(),
		builder.ForVolume("pod-vol-2").PersistentVolumeClaimSource("pvc-2").Result(),
	).Result()
	tracker := newPVCSnapshotTracker()
	tracker.Optout(pod, "pod-vol-1")
	ok, pn := tracker.OptedoutByPod("ns-1", "pvc-1")
	assert.True(t, ok)
	assert.Equal(t, "pod-1", pn)
	// if a volume is tracked for opted out, it can't be tracked as "tracked" or "taken"
	tracker.Track(pod, "pod-vol-1")
	tracker.Track(pod, "pod-vol-2")
	assert.False(t, tracker.Has("ns-1", "pvc-1"))
	assert.True(t, tracker.Has("ns-1", "pvc-2"))
	tracker.Take(pod, "pod-vol-1")
	tracker.Take(pod, "pod-vol-2")
	ok1, _ := tracker.TakenForPodVolume(pod, "pod-vol-1")
	assert.False(t, ok1)
	ok2, _ := tracker.TakenForPodVolume(pod, "pod-vol-2")
	assert.True(t, ok2)
}

func TestABC(t *testing.T) {
	tracker := newPVCSnapshotTracker()
	v1, v2 := tracker.OptedoutByPod("a", "b")
	t.Logf("v1: %v, v2: %v", v1, v2)
}
