package graphql

import (
	"github.com/overmindv/api-gateway/internal/client/arcee"
	"github.com/overmindv/api-gateway/internal/client/ironhide"
	"github.com/overmindv/api-gateway/internal/client/tasksit"
)

type Resolver struct {
	Users   arcee.UserService
	Catalog ironhide.CatalogService
	Tasks   tasksit.Service
}
