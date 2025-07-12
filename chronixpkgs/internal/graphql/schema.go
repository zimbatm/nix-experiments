package graphql

import (
	"time"

	"github.com/graphql-go/graphql"
	"github.com/zimbatm/nix-experiments/chronixpkgs/internal/storage"
)

type Resolver struct {
	store storage.Store
	repo  string
}

func NewResolver(store storage.Store, repo string) *Resolver {
	return &Resolver{
		store: store,
		repo:  repo,
	}
}

func (r *Resolver) BuildSchema() (graphql.Schema, error) {
	// Event type
	eventType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Event",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.String,
			},
			"repo": &graphql.Field{
				Type: graphql.String,
			},
			"type": &graphql.Field{
				Type: graphql.String,
			},
			"actor": &graphql.Field{
				Type: graphql.String,
			},
			"createdAt": &graphql.Field{
				Type: graphql.DateTime,
			},
			"payload": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					event := p.Source.(storage.Event)
					return string(event.Payload), nil
				},
			},
		},
	})

	// EventConnection type for pagination
	eventConnectionType := graphql.NewObject(graphql.ObjectConfig{
		Name: "EventConnection",
		Fields: graphql.Fields{
			"events": &graphql.Field{
				Type: graphql.NewList(eventType),
			},
			"totalCount": &graphql.Field{
				Type: graphql.Int,
			},
			"hasMore": &graphql.Field{
				Type: graphql.Boolean,
			},
		},
	})

	// Query type
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"events": &graphql.Field{
				Type: eventConnectionType,
				Args: graphql.FieldConfigArgument{
					"since": &graphql.ArgumentConfig{
						Type: graphql.DateTime,
					},
					"afterId": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
					"types": &graphql.ArgumentConfig{
						Type: graphql.NewList(graphql.String),
					},
					"limit": &graphql.ArgumentConfig{
						Type:         graphql.Int,
						DefaultValue: 100,
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					limit := p.Args["limit"].(int)
					if limit > 1000 {
						limit = 1000
					}

					var events []storage.Event
					var err error

					// Handle different query patterns
					if afterId, ok := p.Args["afterId"].(string); ok && afterId != "" {
						// Query by ID
						ctx := p.Context
						events, err = r.store.GetEventsAfterId(ctx, r.repo, afterId, limit+1)
					} else if types, ok := p.Args["types"].([]interface{}); ok && len(types) > 0 {
						// Query with type filter
						eventTypes := make([]string, len(types))
						for i, t := range types {
							eventTypes[i] = t.(string)
						}

						since := time.Time{}
						if sinceArg, ok := p.Args["since"].(time.Time); ok {
							since = sinceArg
						}

						filter := storage.EventFilter{
							Repo:       r.repo,
							Since:      since,
							EventTypes: eventTypes,
							Limit:      limit + 1,
						}
						ctx := p.Context
						events, err = r.store.GetEventsFiltered(ctx, filter)
					} else {
						// Simple time-based query
						since := time.Time{}
						if sinceArg, ok := p.Args["since"].(time.Time); ok {
							since = sinceArg
						}
						ctx := p.Context
						events, err = r.store.GetEvents(ctx, r.repo, since, limit+1)
					}

					if err != nil {
						return nil, err
					}

					// Check if there are more results
					hasMore := len(events) > limit
					if hasMore {
						events = events[:limit]
					}

					return map[string]interface{}{
						"events":     events,
						"totalCount": len(events),
						"hasMore":    hasMore,
					}, nil
				},
			},
			"eventTypes": &graphql.Field{
				Type: graphql.NewList(graphql.String),
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					ctx := p.Context
					return r.store.ListEventTypes(ctx, r.repo)
				},
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
}
