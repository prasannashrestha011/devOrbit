package traffic_inspector

import "github.com/go-chi/chi/v5"

func (c *Controller) Register(r chi.Router) {
	r.Get("/workspaces/{id}/traffic", c.List)
	r.Post("/workspaces/{id}/traffic", c.Start)
	r.Get("/workspaces/{id}/traffic/{sessionID}", c.Get)
	r.Delete("/workspaces/{id}/traffic/{sessionID}", c.Stop)
	r.Get("/workspaces/{id}/traffic/{sessionID}/records", c.Recent)
	r.Delete("/workspaces/{id}/traffic/{sessionID}/records", c.Clear)
}
