package ai

import (
	"context"
	"log/slog"
)

// ExitPlanModeBackend wraps an AIBackend and handles ExitPlanMode by ending
// the stream cleanly. When it detects a tool_use event with Name="ExitPlanMode"
// and Done=true, it cancels the CLI process and emits "done" so the caller can
// finalize. No auto-resume — the user sends a new message to continue.
type ExitPlanModeBackend struct {
	inner AIBackend
}

// Name returns the wrapped backend's name.
func (b *ExitPlanModeBackend) Name() string {
	return b.inner.Name()
}

// ExecuteStream runs the inner backend and monitors for ExitPlanMode.
// When detected, the CLI is cancelled and the stream ends with "done".
func (b *ExitPlanModeBackend) ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	outerCh := make(chan StreamEvent, streamChanSize)

	innerCtx, innerCancel := context.WithCancel(ctx)

	innerCh, err := b.inner.ExecuteStream(innerCtx, req)
	if err != nil {
		innerCancel()
		close(outerCh)
		return nil, err
	}

	go b.stream(ctx, innerCtx, innerCancel, req, innerCh, outerCh)
	return outerCh, nil
}

func (b *ExitPlanModeBackend) stream(
	ctx context.Context,
	innerCtx context.Context,
	innerCancel context.CancelFunc,
	origReq ChatRequest,
	innerCh <-chan StreamEvent,
	outerCh chan<- StreamEvent,
) {
	defer close(outerCh)

	for {
		select {
		case event, ok := <-innerCh:
			if !ok {
				return
			}

			// Detect ExitPlanMode: end the stream instead of auto-resuming
			if event.Type == "tool_use" && event.Tool != nil &&
				event.Tool.Name == "ExitPlanMode" && event.Tool.Done {
				slog.Info("ExitPlanMode detected, ending stream",
					slog.String("session", origReq.SessionID))

				forwardEvent(outerCh, event)

				// Cancel the inner CLI process
				innerCancel()

				// Drain remaining events (captures raw_output)
				for drainEvent := range innerCh {
					if drainEvent.Type == "raw_output" {
						forwardEvent(outerCh, drainEvent)
					}
				}

				// Emit done so handler finalizes the message
				forwardEvent(outerCh, StreamEvent{Type: "done"})
				return
			}

			if event.Type == "done" {
				return
			}
			forwardEvent(outerCh, event)

		case <-ctx.Done():
			return
		}
	}
}

// forwardEvent sends an event to the channel, dropping if full.
func forwardEvent(ch chan<- StreamEvent, event StreamEvent) {
	select {
	case ch <- event:
	default:
		slog.Warn("exit_plan_mode: event dropped — channel full",
			slog.String("type", event.Type),
		)
	}
}
