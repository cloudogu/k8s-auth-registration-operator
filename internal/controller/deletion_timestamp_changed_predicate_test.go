package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestDeletionTimestampChangedPredicate(t *testing.T) {
	p := deletionTimestampChangedPredicate{}

	t.Run("returns false when deletion timestamp stays unset", func(t *testing.T) {
		oldObj := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		newObj := oldObj.DeepCopy()

		assert.False(t, p.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}))
	})

	t.Run("returns true when deletion timestamp is added", func(t *testing.T) {
		oldObj := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		newObj := oldObj.DeepCopy()
		now := metav1.NewTime(time.Now())
		newObj.DeletionTimestamp = &now

		assert.True(t, p.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}))
	})

	t.Run("returns true when deletion timestamp changes", func(t *testing.T) {
		oldObj := newAuthRegistrationForControllerTest("ecosystem", "auth-reg")
		oldTime := metav1.NewTime(time.Now())
		oldObj.DeletionTimestamp = &oldTime
		newObj := oldObj.DeepCopy()
		newTime := metav1.NewTime(time.Now().Add(time.Second))
		newObj.DeletionTimestamp = &newTime

		assert.True(t, p.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj}))
	})
}
