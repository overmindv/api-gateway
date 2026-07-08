package graphql

import "github.com/overmindv/laserbeak/internal/client/arcee"

type Resolver struct{ Users arcee.UserService }
