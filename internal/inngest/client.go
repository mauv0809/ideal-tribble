package inngest

import (
	"context"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/inngest/inngestgo"
	"github.com/inngest/inngestgo/step"
)

// New creates a new ClubStore.
func New(inngestClient inngestgo.Client) InngestClient {
	c := &client{
		inngestClient: inngestClient,
	}
	c.createBallBringerFunction()
	return c
}
func (i *client) createBallBringerFunction() inngestgo.ServableFunction {
	config := inngestgo.FunctionOpts{
		ID:   "ball-bringer-assigner",
		Name: "Assign ball bringer",
	}
	f, err := inngestgo.CreateFunction(
		i.inngestClient,
		config,
		// trigger (event or cron)
		inngestgo.EventTrigger("test", nil),
		// handler function
		func(ctx context.Context, input inngestgo.Input[map[string]any]) (any, error) {
			// Here goes the business logic
			// By wrapping code in steps, it will be retried automatically on failure
			_, err := step.Run(ctx, "fetch-players", func(ctx context.Context) (string, error) {
				log.Info("test-1")
				return "OK", nil
			})
			if err != nil {
				return nil, err
			}

			// You can include numerous steps in your function
			_, err = step.Run(ctx, "test_2", func(ctx context.Context) (int, error) {
				log.Info("test-2")
				return 42, nil
			})
			if err != nil {
				return nil, err
			}

			return "OK", nil
		},
	)
	if err != nil {
		log.Fatal("Failed to create function", "error", err)
	}
	return f
}
func (i *client) Serve() http.Handler {
	return i.inngestClient.Serve()
}
func (i *client) SendEvent(name string, data map[string]any) {
	i.inngestClient.Send(context.Background(), inngestgo.Event{Name: name, Data: data})
}
