package inngest

import "net/http"

type InngestClient interface {
	Serve() http.Handler
	SendEvent(name string, data map[string]any)
}
