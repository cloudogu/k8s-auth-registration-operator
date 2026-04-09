package controller

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// deletionTimestampChangedPredicate keeps finalizer-driven cleanup reachable even when only metadata changes.
type deletionTimestampChangedPredicate struct {
	predicate.Funcs
}

func (deletionTimestampChangedPredicate) Update(e event.UpdateEvent) bool {
	oldDeletion := e.ObjectOld.GetDeletionTimestamp()
	newDeletion := e.ObjectNew.GetDeletionTimestamp()

	switch {
	case oldDeletion == nil && newDeletion == nil:
		return false
	case oldDeletion == nil || newDeletion == nil:
		return true
	default:
		return !oldDeletion.Equal(newDeletion)
	}
}
