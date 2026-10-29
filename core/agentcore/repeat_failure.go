package agentcore

import (
	llmtypes "github.com/datnt1998/verse/core/llm"

	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// RepeatFailureLimit is how many times an identical call (same tool name,
// same raw arguments) may fail before it gets blocked.
const RepeatFailureLimit = 2

// RepeatFailureMaxKeys caps the failure-count map so a very long session
// cannot grow it without bound.
const RepeatFailureMaxKeys = 256

// RepeatFailureOptions customizes NewRepeatFailureMiddleware.
type RepeatFailureOptions struct {
	// Refusal builds the error surfaced to the model when a repeated failing
	// call is blocked. call is the blocked call; failed is how many times the
	// identical call already failed. The returned error's message is what the
	// model reads — it should tell the model how to proceed (try different
	// arguments or move on). nil uses the generic default message.
	//
	// The guard still wraps it in ToolBlockedError{Reason: "repeat_failure"}
	// regardless of the custom message, so classification and the
	// tool_blocked event stay identical.
	Refusal func(call llmtypes.ToolCall, failed int) error
}

// NewRepeatFailureMiddleware returns middleware that breaks the loop of
// "retry the exact call that just failed".
//
// Counting is by (tool name + raw arguments): a successful call clears its own
// counter, so re-reading the same input after a fix still works. After
// RepeatFailureLimit failures, that exact (tool, args) pair is refused —
// other argument shapes still run, so the agent keeps a path forward: a call
// that already failed twice verbatim will almost surely fail a third time.
// Counters live in the returned middleware's closure: scope is whatever
// shares the middleware instance (a process-wide runner blocks across runs).
//
// The refusal is returned as *ToolBlockedError, meaning the tool never runs;
// the loop turns it into an is_error tool result classified "blocked" and
// emits a tool_blocked event with Reason "repeat_failure".
func NewRepeatFailureMiddleware(opts *RepeatFailureOptions) ToolMiddleware {
	refusal := func(call llmtypes.ToolCall, failed int) error {
		if opts != nil && opts.Refusal != nil {
			return opts.Refusal(call, failed)
		}
		return fmt.Errorf("call %s with these exact arguments already failed %d times in a row and repeating it will not succeed; change the arguments or use the information already available to take the next step", call.Name, failed)
	}
	var mu sync.Mutex
	failures := make(map[string]int)
	return func(ctx context.Context, call llmtypes.ToolCall, next ToolExecuteFunc) (json.RawMessage, error) {
		key := call.Name + "\x00" + string(call.Args)

		mu.Lock()
		failed := failures[key]
		if len(failures) > RepeatFailureMaxKeys {
			// Overlong session: drop the counters rather than grow memory
			// without bound; blocking re-arms after RepeatFailureLimit new
			// failures.
			clear(failures)
			failed = 0
		}
		mu.Unlock()

		if failed >= RepeatFailureLimit {
			return nil, &ToolBlockedError{
				Reason: "repeat_failure",
				Err:    refusal(call, failed),
			}
		}

		out, err := next(ctx, call.Args)

		mu.Lock()
		if err != nil {
			failures[key] = failures[key] + 1
		} else {
			delete(failures, key)
		}
		mu.Unlock()
		return out, err
	}
}
