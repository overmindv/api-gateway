package graphql

import (
	"log/slog"

	contentclient "github.com/overmindv/api-gateway/internal/client/content"
	"github.com/overmindv/api-gateway/internal/client/entities"
	"github.com/overmindv/api-gateway/internal/client/feed"
	"github.com/overmindv/api-gateway/internal/client/media"
	"github.com/overmindv/api-gateway/internal/client/recommendations"
	"github.com/overmindv/api-gateway/internal/client/taskhunter"
	"github.com/overmindv/api-gateway/internal/client/tasks"
	"github.com/overmindv/api-gateway/internal/client/users"
)

type Resolver struct {
	Content         contentclient.Service
	Recommendations recommendations.Service
	Users           users.UserService
	Catalog         entities.CatalogService
	Tasks           tasks.Service
	Candidates      tasks.CandidateService
	TaskHunter      taskhunter.Service
	Media           media.Service
	Feed            feed.FeedService
	Log             *slog.Logger
	Metrics         *Metrics
}
