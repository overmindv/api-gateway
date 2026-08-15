package graphql

import (
	"github.com/overmindv/api-gateway/internal/client/entities"
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
}
