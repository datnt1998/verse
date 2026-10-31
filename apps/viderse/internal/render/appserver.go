package render

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

// AppServer identifies the already-running Go application by a request-scoped
// runtime response, not merely a listening port. The SPA and all request APIs
// are served by that same process; no second Next/Vite development server exists.
type AppServer struct {
	Origin, WorkspaceID string
	Client              *http.Client
}

func (a AppServer) WaitReady(ctx context.Context, kind, requestID, token string) error {
	client := a.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	origin, err := url.Parse(a.Origin)
	if err != nil {
		return err
	}
	host := net.ParseIP(origin.Hostname())
	if origin.Scheme != "http" || host == nil || !host.IsLoopback() {
		return fmt.Errorf("headless server must use an explicit loopback origin")
	}
	deadline := time.NewTimer(time.Minute)
	defer deadline.Stop()
	tick := time.NewTicker(400 * time.Millisecond)
	defer tick.Stop()
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.Origin+"/api/"+kind+"/"+url.PathEscape(requestID)+"/runtime", nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(request)
		if err == nil {
			var result struct {
				Runtime struct {
					WorkspaceID string `json:"workspaceId"`
					Ready       bool   `json:"ready"`
				} `json:"runtime"`
			}
			decodeErr := json.NewDecoder(response.Body).Decode(&result)
			response.Body.Close()
			if response.StatusCode == 200 && decodeErr == nil && result.Runtime.Ready {
				if result.Runtime.WorkspaceID != a.WorkspaceID {
					return fmt.Errorf("cổng headless đang phục vụ workspace khác")
				}
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("server headless không sẵn sàng trong thời hạn")
		case <-tick.C:
		}
	}
}
