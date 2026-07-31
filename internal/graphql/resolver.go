package graphql

import (
	"github.com/overmindv/laserbeak/internal/client/arcee"
	"github.com/overmindv/laserbeak/internal/client/ironhide"
)

type Resolver struct {
	Users   arcee.UserService
	Catalog ironhide.CatalogService
}
