package service

import (
	"testing"
	"time"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestEnqueueMessage(t *testing.T) {
	sessionID := "qtest-enqueue"
	defer ClearQueue(sessionID)

	queue := EnqueueMessage(sessionID, model.QueuedMessage{
		Text:      "msg1",
		CreatedAt: time.Now().Format(time.RFC3339),
	})
	assert.Len(t, queue, 1)
	assert.Equal(t, "msg1", queue[0].Text)

	queue = EnqueueMessage(sessionID, model.QueuedMessage{
		Text:      "msg2",
		CreatedAt: time.Now().Format(time.RFC3339),
	})
	assert.Len(t, queue, 2)
	assert.Equal(t, "msg1", queue[0].Text)
	assert.Equal(t, "msg2", queue[1].Text)
}

func TestDequeueMessage(t *testing.T) {
	sessionID := "qtest-dequeue"
	defer ClearQueue(sessionID)

	EnqueueMessage(sessionID, model.QueuedMessage{Text: "first", CreatedAt: time.Now().Format(time.RFC3339)})
	EnqueueMessage(sessionID, model.QueuedMessage{Text: "second", CreatedAt: time.Now().Format(time.RFC3339)})

	msg, ok := DequeueMessage(sessionID)
	assert.True(t, ok)
	assert.Equal(t, "first", msg.Text)

	msg, ok = DequeueMessage(sessionID)
	assert.True(t, ok)
	assert.Equal(t, "second", msg.Text)
}

func TestDequeueMessage_Empty(t *testing.T) {
	sessionID := "qtest-dequeue-empty"
	defer ClearQueue(sessionID)

	// Enqueue then dequeue all
	EnqueueMessage(sessionID, model.QueuedMessage{Text: "only", CreatedAt: time.Now().Format(time.RFC3339)})
	DequeueMessage(sessionID)

	_, ok := DequeueMessage(sessionID)
	assert.False(t, ok)
}

func TestDequeueMessage_NonexistentSession(t *testing.T) {
	_, ok := DequeueMessage("qtest-nonexistent")
	assert.False(t, ok)
}

func TestGetQueue(t *testing.T) {
	sessionID := "qtest-get"
	defer ClearQueue(sessionID)

	EnqueueMessage(sessionID, model.QueuedMessage{Text: "a", CreatedAt: time.Now().Format(time.RFC3339)})
	EnqueueMessage(sessionID, model.QueuedMessage{Text: "b", CreatedAt: time.Now().Format(time.RFC3339)})

	queue := GetQueue(sessionID)
	assert.Len(t, queue, 2)
	assert.Equal(t, "a", queue[0].Text)
	assert.Equal(t, "b", queue[1].Text)
}

func TestGetQueue_Empty(t *testing.T) {
	sessionID := "qtest-get-empty"
	defer ClearQueue(sessionID)

	// Enqueue then dequeue all → entry gets deleted
	EnqueueMessage(sessionID, model.QueuedMessage{Text: "x", CreatedAt: time.Now().Format(time.RFC3339)})
	DequeueMessage(sessionID)

	queue := GetQueue(sessionID)
	assert.Nil(t, queue)
}

func TestGetQueue_Nonexistent(t *testing.T) {
	queue := GetQueue("qtest-nonexistent-get")
	assert.Nil(t, queue)
}

func TestRemoveQueueItem(t *testing.T) {
	sessionID := "qtest-remove"
	defer ClearQueue(sessionID)

	EnqueueMessage(sessionID, model.QueuedMessage{Text: "a", CreatedAt: time.Now().Format(time.RFC3339)})
	EnqueueMessage(sessionID, model.QueuedMessage{Text: "b", CreatedAt: time.Now().Format(time.RFC3339)})
	EnqueueMessage(sessionID, model.QueuedMessage{Text: "c", CreatedAt: time.Now().Format(time.RFC3339)})

	queue := RemoveQueueItem(sessionID, 1)
	assert.Len(t, queue, 2)
	assert.Equal(t, "a", queue[0].Text)
	assert.Equal(t, "c", queue[1].Text)
}

func TestRemoveQueueItem_OutOfRange(t *testing.T) {
	sessionID := "qtest-remove-oob"
	defer ClearQueue(sessionID)

	EnqueueMessage(sessionID, model.QueuedMessage{Text: "a", CreatedAt: time.Now().Format(time.RFC3339)})

	queue := RemoveQueueItem(sessionID, 5)
	assert.Len(t, queue, 1)
	assert.Equal(t, "a", queue[0].Text)

	queue = RemoveQueueItem(sessionID, -1)
	assert.Len(t, queue, 1)
}

func TestRemoveQueueItem_LastItem(t *testing.T) {
	sessionID := "qtest-remove-last"
	defer ClearQueue(sessionID)

	EnqueueMessage(sessionID, model.QueuedMessage{Text: "only", CreatedAt: time.Now().Format(time.RFC3339)})

	queue := RemoveQueueItem(sessionID, 0)
	// Last item removed → entry deleted from map → nil
	assert.Nil(t, queue)
}

func TestClearQueue(t *testing.T) {
	sessionID := "qtest-clear"
	defer ClearQueue(sessionID)

	EnqueueMessage(sessionID, model.QueuedMessage{Text: "a", CreatedAt: time.Now().Format(time.RFC3339)})
	EnqueueMessage(sessionID, model.QueuedMessage{Text: "b", CreatedAt: time.Now().Format(time.RFC3339)})

	ClearQueue(sessionID)
	assert.Nil(t, GetQueue(sessionID))
}

func TestEnqueueReturnsCopy(t *testing.T) {
	sessionID := "qtest-copy"
	defer ClearQueue(sessionID)

	queue := EnqueueMessage(sessionID, model.QueuedMessage{Text: "a", CreatedAt: time.Now().Format(time.RFC3339)})

	// Modify the returned slice — should not affect internal state
	queue[0].Text = "modified"

	// Get a fresh snapshot
	fresh := GetQueue(sessionID)
	assert.Equal(t, "a", fresh[0].Text)
	assert.Len(t, fresh, 1)
}
