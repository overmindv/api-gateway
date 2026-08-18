package graphql

import (
	"log/slog"

	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/media"
	"github.com/overmindv/api-gateway/internal/client/taskhunter"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/client/users"
)

type Resolver struct {
	Users      users.UserService
	Catalog    entities.CatalogService
	Tasks      tasks.Service
	Candidates tasks.CandidateService
	TaskHunter taskhunter.Service
	Media      media.Service
	Log        *slog.Logger
	Metrics    *Metrics
}
